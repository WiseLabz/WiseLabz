# Design

## Context

See proposal.md for motivation. Current state that shapes the approach:

- `runbooks.Handler.ExecuteStep` checks an operator grant on the step's connector and delegates to `connectors.Handler.ServeLifecycleOp`, which is HTTP-shaped: `dryRun=true` calls `lifecycleOpPreview`, otherwise `lifecycleOpMutate` validates an `X-Elevation-Token` for `connector.<verb>`, calls the connector, raises a failure alert and audits.
- `BulkRestart` already shows the pattern for one elevation covering several operations: it validates once and calls `restartConnector`, a core without the per-call elevation.
- Elevation tokens are action-scoped, optionally target-scoped (`auth.ValidateElevationHeaderFor`), and live 60 seconds (ADR 0001).
- Sync is asynchronous: `sync.Engine.RunSyncFields(ctx, connectorID, jobID, fields)` blocks until the sync ends and returns its result; `Engine.Go` tracks background work against the engine's base context.
- Health is connector level only: `health.RunHealthCheck` runs `Validate`, persists the status and returns `Result{Status}`.
- `toStepResponses` already redacts steps on connectors the caller cannot view and computes `canExecute` per user.
- Retention is a single-row `retention_settings` table swept by `retention.RunCleanupOnce`.
- Migrations are paired sqlite/postgres files; the latest is `000061`.
- Multi-replica work is deferred (#419); leader election exists but in-process serialization is still assumed.

## Goals / Non-Goals

**Goals:**
- A run never mutates the lab without a human having elevated for exactly the steps it executes.
- Run state is durable enough that a crash leaves an accurate, resumable record.
- Lifecycle steps inside a run behave identically (alert, audit) to direct lifecycle operations.

**Non-Goals:**
- Running on more than one replica. A run executes in the process that accepted it.
- Parallel steps, branching, per-step continue-on-error or automatic retry.
- Rollback of steps already executed.

## Decisions

**1. Server-side executor in a new `internal/runbookrun` package.**
It owns the state machine and is driven by narrow interfaces for the store, lifecycle operation, sync and health, so it is testable without HTTP. The API handler only validates, authorizes and calls it. Alternative: keep the browser sequencing steps and only record history; rejected because a closed tab abandons the run and waits would live in the client.

**2. Frozen steps are copied into `runbook_run_steps` at start.**
Each run step row carries position, kind, title, connector id, verb, entity ref, timeout and its execution state. `runbook_runs` stores the runbook id as a nullable reference (SET NULL on runbook delete) plus the runbook title at start. Alternative: reference `runbook_steps` rows; rejected because `ReplaceRunbookSteps` deletes and re-creates them, and the elevation must cover exactly what was previewed.

**3. One elevation, new action `runbook.run` targeted at the runbook id.**
Validated in the handler with `auth.ValidateElevationHeaderFor` at start and at resume. The executor then calls an elevation-free lifecycle core. This extends ADR 0001/0002 the same way `BulkRestart` does and is recorded in a short new ADR. The token's 60 second life only bounds the time between preview and start, not the run.

**4. Extract lifecycle cores from the connectors handler.**
Split `lifecycleOpPreview` and `lifecycleOpMutate` into context-based functions that return a value or error (preview result; mutate with failure alert and audit, taking the acting user id and extra audit detail). The HTTP handlers and `ServeLifecycleOp` become thin wrappers, so single-step execution keeps its exact behaviour. The executor calls the cores through an interface. Because the run is not an HTTP request, audit rows are written with an explicit actor instead of `RecordAuditFromContext`.

**5. Authorization helper shared by start, resume, confirm and cancel.**
"Operator on every distinct connector in the run's steps", including the API-key connector restriction, is one function over the frozen steps. The per-step re-check before execution uses the user who started or last resumed the run.

**6. One active run per runbook enforced by the database.**
A partial unique index on `runbook_runs(runbook_id)` where state is `running`, `waiting_manual` or `failed`, in both dialects. A constraint violation maps to 409. Alternative: an in-process mutex; rejected because it does not survive restarts and gives no protection against a stale `failed` run.

**7. Execution loop and cancellation.**
Each active run has one goroutine started through the sync engine's tracked `TryGo` so shutdown waits for it, with a per-run cancel function held in an in-memory registry; a second goroutine for the same run waits for the first to stop. A start, resume or confirm that arrives after shutdown began is refused and the run is recorded as `failed` with reason `interrupted`. Every step transition is a single store update followed by a WebSocket event. Cancel sets the state in the database and cancels the context; the loop re-reads the run state before each step. `wait_until_healthy` polls `RunHealthCheck` every 10 seconds; `sync_and_wait` calls `RunSyncFieldsWhenFree`, which waits for a sync already running on the connector, with the step timeout as deadline. A manual step ends the goroutine; confirm and resume start a new one from the first non-succeeded step.

**8. Startup recovery marks, never continues.**
Before the HTTP server accepts requests, one statement moves `running` runs to `failed` (reason `interrupted`) and their `running` step to `unknown`. Alternative: auto-continue; rejected because the outcome of the in-flight mutation is unknown and no human is present.

**9. Expiry and pruning in the retention job.**
Two new columns, `runbook_open_run_hours` (default 24) and `runbook_run_days` (default 90, 0 keeps forever), exposed in the existing retention settings API and form. Activity is the run's `updated_at`, touched on every transition.

**10. Events and notifications.**
New WebSocket event `runbook.run.updated`, sent with `BroadcastConnector` when a step has a connector and to users otherwise permitted for the run for run-level changes. Two notification event types, `runbook.run_failed` and `runbook.run_waiting`, dispatched like existing events.

**11. API shape.**
`POST /api/runbooks/{id}/run` (`dryRun=true` for preview), `GET /api/runbooks/{id}/runs`, `GET /api/runbook-runs/{runId}`, `POST /api/runbook-runs/{runId}/steps/{stepId}/confirm`, `POST /api/runbook-runs/{runId}/resume`, `POST /api/runbook-runs/{runId}/cancel`. Run routes sit outside the admin-only group, like `ExecuteStep`.

**12. Step schema.**
`runbook_steps` gains `kind` (default `lifecycle`) and `timeout_seconds`; `connector_id` and `verb` become nullable for manual steps. Backup export/import carries the new columns; runs themselves are operational history and are not backed up.

## Risks / Trade-offs

- [A grant revoked after start still allows the step in flight] → the re-check runs immediately before each step; one step is the smallest unit.
- [Interrupted lifecycle step has an unknown outcome and resume repeats it] → the step is shown as `unknown` with a warning in the resume dialog; restart, start and stop are safe to repeat.
- [A long `sync_and_wait` holds a sync slot] → it uses the engine's normal concurrency limits and the step timeout.
- [Multi-replica deployments would run recovery on every replica and could fail another replica's run] → single-replica is the supported mode today; noted on #419.
- [Refactoring `lifecycle.go` could change single-step behaviour] → existing lifecycle and runbook handler tests must pass unchanged before the executor is wired.

## Migration Plan

Additive migrations `000062` (step columns, run tables, partial unique index) and `000063` (retention columns), each with a down file. Existing steps become `lifecycle` through the column default. Rollback is the down migrations; no data outside the new tables and columns is touched.
