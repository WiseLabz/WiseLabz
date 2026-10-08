# 0007 — Config push inside runbook runs

Status: accepted (implementation in progress, #649, #652)

## Context

ADR 0003 defined the permission, elevation, audit, and rollback model for direct,
field-level configuration changes (`connector.configPush`), using a per-connector
writable whitelist, snapshot-diff verification, and auto-revert on mismatch. ADR 0006
established the elevation and execution boundary for whole-runbook runs (`runbook.run`),
allowing multi-step operational sequences to execute server-side without per-step
interactive elevation prompts.

Previously, runbook runs supported only runtime lifecycle operations (`restart`,
`start`, `stop`), syncs, health checks, and manual confirmation steps. Remediation
workflows that require reconfiguring a service or container (such as adjusting memory,
changing firewall rules, or updating DNS records) forced operators to execute those
configuration changes out-of-band in separate direct API or UI interactions.

Adding a `config_push` step kind to runbooks requires integrating persistent configuration
writes into automated, server-side sequential runs. This ADR defines the elevation,
parameter freezing, current-value reading, idempotency, and rollback rules for config
push inside runbook runs, extending ADR 0003 and ADR 0006.

## Decision

### 1. Single `runbook.run` elevation covers config writes (D1)

Starting or resuming a runbook run requires a single elevation token for the action
`runbook.run`, scoped to the target runbook ID. This elevation covers all steps within
the run, including `config_push` steps:

- Before elevation is granted, the dry-run preview presents every step in sequence,
  including the target field, frozen target value, and current live value (or an explicit
  indication that the current value is unknown) for every `config_push` step.
- Because the operator reviews and approves the exact target configuration before
  elevation, and the execution parameters are immutably frozen at start, a separate
  step-up elevation at the moment of configuration write is unnecessary.
- The acting user must hold an operator grant on the step's connector at start, at resume,
  and during the pre-step grant re-check immediately before execution (ADR 0006).

**Alternatives considered and rejected:**
- *Re-elevation per push step:* Requiring an interactive `connector.configPush` elevation
  token when execution reaches a config-push step would block unattended runs. Elevation
  tokens have a 60-second TTL (`elevationTTL` in `backend/internal/auth/jwt.go`), whereas
  runs routinely span minutes due to restarts, syncs, and health verification waits.
- *Distinct `runbook.run_config` elevation action at start:* Prompts the operator for a
  second elevation action during run initiation without providing any additional security
  boundary, since the preview already details both lifecycle and configuration operations.

### 2. Frozen target value at run start

When a run is created, `FreezeSteps` copies the authored step's `fieldKey`, `targetValue`,
and `entityRef` into the durable `runbook_run_steps` record:

- Subsequent modifications to the parent runbook do not affect active, pending, or paused runs.
- Execution always writes the target value frozen when the run started.

### 3. Optional `ConfigReader` and the already-at-target rule (D4)

ADR 0003 verifies writes via snapshot comparison (`sync.Compare(pre, post)`): a non-empty
diff confirms that the write took effect. However, writing a value that the system already
holds produces an empty diff, which would be erroneously flagged as a verification mismatch
and trigger an auto-revert.

To resolve this, connectors may optionally implement `connector.ConfigReader`:

```go
type ConfigReader interface {
    ConfigRead(ctx context.Context, config map[string]any, entityRef, fieldKey string) (value any, err error)
}
```

- When a connector implements `ConfigReader`, the executor reads the live field value
  immediately before writing.
- If the current value already matches the target value, the push succeeds immediately
  without issuing a write and without recording an audit log entry.
- If a connector lacks a `ConfigReader`, this case cannot be distinguished from a failed write;
  the step fails with a verification error without auto-revert, as specified below.

### 4. Auto-revert only when the previous value is known

Direct config pushes (`POST /api/connectors/{id}/config-push`) receive a `previousValue`
from the web client, which read the field before editing. Automated runs have no client in
the loop:

- When the connector implements `ConfigReader`, the pre-push read provides the verified
  previous value. If the write fails verification, the executor restores this previous
  value via `ConfigPush`, raises a critical alert, and fails the step and run.
- When the connector lacks a `ConfigReader`, the previous value is unknown. The executor
  does **not** attempt a blind revert. It raises a critical alert stating that the pushed
  value could not be verified and the previous value was unknown, and fails the step and run.

### 5. Detached bounded context for write, verify, and revert

The executor performs config push via a detached context (`context.WithoutCancel(ctx)`)
bounded by `configPushTimeout` (default 2 minutes):

- If a run is cancelled or the server initiates a graceful shutdown while a write is in flight,
  the write, post-write fetch/verification, and auto-revert sequence are allowed to complete
  within the bounded timeout.
- This prevents cancelling the run from severing an in-flight write mid-mutation, leaving
  the lab in an inconsistent state, dropping the audit record, or emitting a spurious mismatch alert.

### 6. Interrupted and cancelled pushes stay `unknown`

If a backend crash or shutdown interrupts a run during a config push, or if a cancellation
cancels the step before the bounded core completes:

- The step is recorded in state `unknown`.
- The system never automatically resumes an interrupted run.
- When an operator resumes the run with a fresh `runbook.run` elevation token, the step
  re-executes. Because of the already-at-target rule (D4), if the interrupted write had
  actually landed upstream, the re-execution observes the target value and succeeds cleanly
  without writing again or triggering a false mismatch.
- This only works when the connector implements `ConfigReader`. Without a reader, a resumed
  step whose write had landed fails with the could-not-verify mismatch alert (D4).
- If the 2-minute bound (`configPushTimeout`) cuts the core short, the step fails with a
  message that the field may or may not have been written; the outcome is unknown.

### 7. Connector-specific config-push behaviors (#677)

Two connector-specific behaviors resolved in #677 provide robust idempotency and accurate current-value reading:

1. **OPNsense savepoint flow:** To prevent a failed apply from leaving saved rules unapplied (which
   would cause a subsequent push retry or resume to observe the target value in the reader and falsely
   report success), `ConfigPush` utilizes OPNsense's savepoint mechanism:
   - Initiates an atomic change window with `POST /api/firewall/filter/savepoint` to record a revision ID.
   - Updates the rule configuration with `POST /api/firewall/filter/setRule/<entityRef>`.
   - Applies the configuration with `POST /api/firewall/filter/apply/<revision>`, starting the rollback timer.
   - On success, finalizes the change with `POST /api/firewall/filter/cancelRollback/<revision>`.
   - On any failure during `setRule` or `apply`, immediately calls `POST /api/firewall/filter/revert/<revision>`
     to discard unapplied changes, ensuring failed mutations do not linger in saved state.

2. **Proxmox configured memory reading:** Rather than reading `maxmem` from the host's guest list
   (which reflects effective running allocation and hides pending config changes), `ConfigReader`
   reads `memory` directly from the guest's `/config` endpoint (for both VMs and LXC containers),
   matching the key written by `ConfigPush`:
   - Decodes memory represented as a JSON number, numeric string, or property string containing
     `current=<n>` (such as `current=2048` or `current=2048,max=4096`).
   - If guest `/config` cannot be fetched, `memory` is treated as unavailable (returning an error
     rather than falling back to running `maxmem`), preventing false "already-at-target" skips or
     erroneous auto-reverts against running state.
