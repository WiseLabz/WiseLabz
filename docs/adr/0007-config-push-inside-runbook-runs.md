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

### 7. Connector-specific behaviour (#677)

These two cases were first accepted as limitations. They are not any more: both connectors now
keep the saved state and the live state consistent after a failed push, so the already-at-target
rule (D4) cannot report success for a write that was never applied.

**OPNsense.** `setRule` only saves a rule; `apply` makes it live. Before #677 a failed `apply`
left the new value saved. A retry, or a resumed step, then read the target through `ConfigRead`,
skipped the write and reported success although the rule was not live. After any failed push the
saved value is now the previous one again whenever the firewall can still be reached.

Savepoint path (OPNsense 24.1 to 26.1):

- `ConfigPush` calls `POST /api/firewall/filter/savepoint`, then `setRule/<uuid>`, then
  `apply/<revision>`, then `cancelRollback/<revision>`. The revision must look like a unix time
  (`123.456`) before it is put into a URL path.
- `apply/<revision>` starts a 60 second rollback timer and answers with the raw configd output:
  `OK` plus newlines on success, an error text or an empty string otherwise. The status is compared
  trimmed and case-insensitively with `ok`; anything else, an empty status and an HTTP error are a
  failed apply.
- A failed or timed-out apply, and a `setRule` that may or may not have saved, are reverted
  explicitly with `revert/<revision>`. The revert runs with a context detached from the caller's
  cancellation and bounded to 30 seconds, so it still happens when the run was cancelled.
- After a successful revert `ConfigPush` sends `cancelRollback/<revision>` (best effort), so the
  pending 60 second timer cannot roll the filter back a second time and undo a later push. If that
  call fails the error says the timer may still fire within about a minute and the failure is logged.
- When the revert itself cannot be delivered, `cancelRollback` is not sent. The timer is left alone
  so OPNsense rolls the change back by itself.
- A `cancelRollback` that fails after a successful apply is a failed push, never a success: OPNsense
  would roll the rule back about a minute later. The change is reverted and the error says that the
  apply succeeded but the rollback could not be cancelled. Its status string is not interpreted,
  only a transport or HTTP error counts.

Fallback path (OPNsense 26.7 and later):

- Upstream removed savepoint, revert and cancelRollback (commit `17b84612eb`, 2026-06-18).
  `ConfigPush` detects this per call: `savepoint` answering 404 selects the fallback, any other
  error stops the push before anything is written.
- The push reads the previous value with `GET /api/firewall/filter/getRule/<uuid>`, calls
  `setRule`, then `apply` (without a revision). If the apply fails, it writes the previous value
  back with `setRule` and applies again, with the same detached bounded context. The original
  error is returned, saying the previous value was restored, or that the undo failed too and the
  rule may be saved with the new value without being applied.

What a retry or resumed step sees: after a failed push the saved value is the previous one again,
so `ConfigRead` does not report the target and the core writes and applies again.

Pushes to one firewall are serialised, keyed on the connector URL, because savepoint, revert and
the rollback timer act on the whole filter section and interleaved pushes could undo each other.
The lock covers the whole flow and honours the caller's context while waiting. It is per process:
it covers one WiseLabz process only.

Remaining limitations:

1. On 26.7 and later, a push cut off after `setRule` and before the undo can be sent (process
   crash, lost network) leaves the rule saved but not applied, because OPNsense offers no
   server-side rollback there. A retry then reads the target and reports success.
2. On 24.1 to 26.1 the same holds if neither the apply nor the revert reaches the firewall. If the
   apply did reach it, OPNsense rolls back within about 60 seconds; during that time a retry can
   still read the target value.
3. Pushes are serialised per firewall only inside one WiseLabz process (replacing this for
   active/active is tracked in #419).

**Proxmox.** `ConfigRead` for `memory` reads the value from the guest `/config` that is already
fetched for `cores`, for VMs and containers, so it makes no extra request. This matches the key
`ConfigPush` writes, and `/config` returns pending values, so a pending change is what the reader
sees; the running `maxmem` of the guest list does not show it.

- `memory` is decoded from a JSON number, a numeric string or a property string with
  `current=<n>` (such as `current=2048,max=4096`). It must be a positive whole number of MiB.
- When `/config` cannot be read, or its `memory` cannot be decoded, the reader returns an error
  instead of the running `maxmem`. A pre-push read failure therefore stops the push, and a failed
  verification can no longer write the live value over a pending change. A config without a
  `memory` key keeps the guest-list value, because nothing is pending then.
