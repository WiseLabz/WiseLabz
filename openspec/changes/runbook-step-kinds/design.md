# Design

## Context

See proposal.md for motivation. Current state that shapes the approach:

- `internal/runbookrun` executes frozen steps over small interfaces (`Lifecycle`, `Syncer`, `HealthChecker`, `Grants`). `perform` in `run.go` switches on kind; `authorize` re-checks the acting user's operator grant per step.
- Steps are stored as flat columns (`kind`, `timeout_seconds`, `connector_id`, `verb`, `entity_ref`) on `runbook_steps` and copied to `runbook_run_steps` by `FreezeSteps`.
- Direct config push lives in `internal/api/connectors/config_push.go`: elevation check, `Fetch` before, `ConfigPusher.ConfigPush`, `Fetch` after, `configPushLanded` (any snapshot diff), and on mismatch `revertConfigPush` with the `previousValue` the browser sent, plus a critical alert. The file notes that the backend cannot derive a field's current value.
- Seven connectors implement `ConfigPusher`; their writable fields are number, select, text or toggle.
- The compliance engine (`internal/compliance/engine.go`) already evaluates `eq`, `neq`, `contains`, `regex`, `gt`, `lt` against entity attributes, and `quality.Checker.loadComplianceSnapshot` already loads a connector's latest entities in the shape that engine reads.
- `Syncer.RunSyncFieldsWhenFree` runs a recorded sync and waits behind one already in flight.

## Goals / Non-Goals

**Goals:**
- Both kinds plug into the existing executor switch, freeze, grant re-check, resume and cancel paths without new run states.
- One implementation of push, verify and revert, used by the HTTP handler and by runs.

**Non-Goals:**
- Changing the direct config-push endpoint's contract (it keeps accepting `previousValue`).
- A current-value reader for every pusher; connectors without one degrade as specified.
- Any change to `wait_until_healthy`.

## Decisions

### D1. One elevation covers config writes in a run
`runbook.run` authorizes every step, including `config_push`. The start preview shows current and target for each push and the target is frozen with the steps, so what was approved is what runs. This matches lifecycle steps (ADR 0006).
Alternatives: pause for re-elevation at each push (blocks unattended runs); a distinct `runbook.run_config` action at start (a second prompt for the same decision). Recorded in a new ADR `0007` extending 0003 and 0006.

### D2. Optional `ConfigReader` interface
Add next to `ConfigPusher` in `internal/connector/connector.go`:

```go
type ConfigReader interface {
    ConfigRead(ctx context.Context, config map[string]any, entityRef, fieldKey string) (value any, err error)
}
```

It is separate from `ConfigPusher` so existing pushers compile unchanged, and `Capabilities` gains a `configRead` flag derived from it. Implement it in this change for pushers whose field value is already present in what they fetch; any pusher where it needs a new upstream call is left without it and listed in tasks as a follow-up issue.
Alternative: map each `ConfigField` to an entity attribute name and read the stored snapshot. Rejected: writable fields are not all entity attributes, and a stored snapshot can be stale at push time.

### D3. Shared config-push core
Extract from `config_push.go` a context-based function on the connectors handler, in the same way lifecycle was split for runs (`MutateRunbookLifecycleOp`):

```go
MutateRunbookConfigPush(ctx, connectorID, entityRef, fieldKey string, value any, actor LifecycleActor, extraAudit map[string]any) error
```

Flow: resolve pusher and check the field is in `WritableFields()`; if the connector is a `ConfigReader`, read the current value and return success without writing when it equals the target; fetch, push, fetch, `configPushLanded`; on mismatch revert when a previous value is known, raise the existing alert, return a typed mismatch error. The HTTP handler becomes a wrapper that validates elevation, passes the browser's `previousValue` as the known previous value when the connector has no reader, and maps errors to the current status codes. The executor gets a `ConfigPush` interface in `Deps` that `*connectors.Handler` satisfies.

### D4. "Already at target" succeeds without writing
Verification is "the snapshot changed". A push of the value already held changes nothing and would be read as a mismatch, which would make resuming an interrupted push fail and then revert a correct value. With a reader, equal current and target is a success with no write and no audit entry. Without a reader this case cannot be told apart from a failed write; the step fails with "could not verify" and no revert, as specified. This is the main reason to implement the reader on as many pushers as is cheap.

### D5. Step storage: explicit nullable columns
Migration `000065` (sqlite and postgres) adds to `runbook_steps` and `runbook_run_steps`: `field_key`, `target_value` (JSON text), `attribute`, `operator`, `expected_value` (JSON text). `entity_ref` is reused by both kinds. This follows the existing flat-column pattern, keeps backup export/import a column list change, and keeps `FreezeSteps` a field copy.
Alternative: one `params` JSON column. Rejected for consistency with `verb`/`entity_ref` and simpler SQL in `ListEntityRunbookSteps`.

### D6. `wait_for_entity` reuses sync and the compliance matcher
Loop: `Syncer.RunSyncFieldsWhenFree` (full sync), load the connector's entities through the same loader the quality checker uses, find the entity by `entity_ref`, evaluate one `compliance.Condition{Attribute, Op, Value}` with the engine's exported matcher, sleep 30 seconds (`EntityPollInterval`, a constant beside `HealthPollInterval`, overridable in tests like `healthPollInterval`). The matcher is exported from `internal/compliance` rather than copied; if the entity loader lives in `internal/quality`, move it to `internal/compliance` or the store so `runbookrun` does not import `quality`.
A failed sync is logged and treated as "not yet" so one flaky poll does not fail a restart that is still in progress; the step timeout bounds the total.
Alternatives: `conn.Fetch` without persisting (UI and findings would lag behind what the run saw); a condition option on `wait_until_healthy` (rejected by the user in favour of a separate kind).

### D7. Timeout reason carries the last observation
The wait keeps the last observed state (entity absent, attribute absent, or the value) and formats it into the step error on timeout, truncated to a fixed length. Reason code stays `step_timeout`.

### D8. Authoring validation and attribute names
`validateSteps` in `internal/api/runbooks` checks the new kinds against the live connector: `ConfigPusher`, `WritableFields()`, entity scope, field type, value shape; operator set, regex compile, numeric `gt`/`lt`. Attribute suggestions come from the existing compliance schema endpoint that serves `RegisterAttributeCatalog` data; no new endpoint. Free text is accepted because custom REST recipes and some connectors declare nothing.

### D9. Preview reads live, fails soft
The preview calls `ConfigRead` per push step with a short timeout. A read error or missing reader yields `currentValueKnown: false` rather than failing the preview. A field no longer writable marks the step not executable, like a missing grant.

## Risks / Trade-offs

- [Preview value is stale by the time the step runs] → The step re-reads immediately before writing; revert uses that read, not the preview's.
- [Reader-less pushers cannot resume an interrupted push cleanly] → Step fails with a clear reason and an alert; D4 limits this to connectors without a reader, and the preview already says "current value unknown" for them.
- [An entity wait multiplies sync load] → Fixed 30 second floor, one active run per runbook, 30 minute cap, and it waits behind an in-flight sync instead of stacking.
- [Regex in a wait condition] → Go's `regexp` is linear time; value length is capped by validation.
- [Target value stored in plain text] → Password and secret field types are rejected at authoring; no current writable field is of those types.

## Migration Plan

Additive migration `000065`; the down migration drops the five columns. Existing rows have nulls and existing kinds ignore them. Rolling back the binary after runbooks with new kinds exist leaves steps of unknown kind; the down migration deletes steps of the two new kinds first and the release note says so.
