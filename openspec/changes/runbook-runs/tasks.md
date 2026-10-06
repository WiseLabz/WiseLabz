# Tasks

## 1. Schema and store

- [x] 1.1 Add paired sqlite/postgres migration `000062`: `kind` and `timeout_seconds` on `runbook_steps`, nullable `connector_id`/`verb`, tables `runbook_runs` and `runbook_run_steps`, partial unique index for one active run per runbook; verify the migration up/down tests pass on both dialects.
- [x] 1.2 Extend `RunbookStepRecord` and step CRUD with kind and timeout, and carry them through backup export/import; verify store tests and a backup round-trip test.
- [x] 1.3 Add run store functions (create with frozen steps, get, list by runbook with pagination, step and run transitions that touch `updated_at`, interrupt-running, expire-open, prune-finished); verify store tests including the 409 path for a second active run and survival of runs after runbook deletion.
- [x] 1.4 Add migration `000063` with `runbook_open_run_hours` and `runbook_run_days` on `retention_settings` and extend `RetentionSettings` get/update; verify retention settings store tests.

## 2. Step kinds in authoring

- [x] 2.1 Extend `validateSteps`, step input/response and `toStepResponses` for `sync_and_wait`, `wait_until_healthy` and `manual` with timeout bounds; verify handler tests for each kind, the timeout range error and a manual step without connector.
- [x] 2.2 Reject single-step execution of non-lifecycle steps in `ExecuteStep`; verify a handler test and that existing `ExecuteStep` tests pass unchanged.
- [x] 2.3 Update OpenAPI `RunbookStep`/`RunbookStepInput` and regenerate the web client; verify the OpenAPI contract test and web typecheck.

## 3. Lifecycle cores

- [x] 3.1 Split `lifecycleOpPreview` and `lifecycleOpMutate` in `internal/api/connectors/lifecycle.go` into context-based cores (explicit actor, extra audit detail, no elevation) with the HTTP handlers as wrappers; verify all existing lifecycle, bulk restart and runbook handler tests pass without modification.
- [x] 3.2 Add tests calling the cores directly for preview output, failure alert and audit detail with run and step identifiers.

## 4. Run executor

- [ ] 4.1 Create `internal/runbookrun` with the state machine over store, lifecycle, sync and health interfaces; verify unit tests for all-succeed, failure halts with later steps pending, and timestamps on every transition.
- [ ] 4.2 Implement `sync_and_wait` and `wait_until_healthy` with step timeouts and 10 second health polling; verify tests with fake sync/health for success, timeout and cancellation mid-wait.
- [ ] 4.3 Implement manual pause, confirm, resume from the first non-succeeded step and cancel; verify tests for each transition and the 409 cases.
- [ ] 4.4 Implement the per-step grant re-check for the starting or last resuming user; verify a test where a revoked grant fails the step without calling the connector.
- [ ] 4.5 Add startup recovery that marks running runs interrupted and wire it before the server listens; verify a test that a running run becomes failed/interrupted with its step unknown and a waiting run is untouched.
- [ ] 4.6 Emit `runbook.run.updated` WebSocket events and the `runbook.run_failed` / `runbook.run_waiting` notification events; verify hub filtering by connector grant and dispatcher tests, including no notification on success or cancel.

## 5. API

- [ ] 5.1 Add the shared run authorization helper (operator on every connector of the frozen steps, API-key restriction) and the run endpoints: start, dry-run preview, list, get, confirm, resume, cancel; verify a handler authorization matrix covering missing elevation, token for another runbook, missing grant, restricted key, 202 on start and 409 on a second start.
- [ ] 5.2 Apply step redaction to run list/detail and preview; verify tests with mixed grants and a hidden connector.
- [ ] 5.3 Record audit entries for start, confirm, resume and cancel and add them and `runbook.run` to `docs/AUDIT.md`; verify audit assertions in the handler tests.
- [ ] 5.4 Register routes, add OpenAPI paths and schemas, regenerate the web client; verify the router and OpenAPI contract tests.
- [x] 5.5 Write ADR `0006` extending ADR 0001/0002 with the `runbook.run` elevation model and interruption rule; verify it is linked from `docs/adr` index or `docs/ARCHITECTURE.md` as the other ADRs are.

## 6. Retention

- [x] 6.1 Expire open runs and prune finished runs in `retention.RunCleanupOnce`; verify tests for a forgotten manual step, recent activity not expiring, pruning, and history period 0.
- [x] 6.2 Expose the two settings in the retention settings API, OpenAPI and settings form with en and pt-BR labels; verify handler tests and the settings page vitest.

## 7. MCP

- [ ] 7.1 Add read-only list-runs and get-run tools in `internal/mcp/runbooks.go` with the same redaction; verify MCP tests for visibility and that no mutating run tool is registered.

## 8. Web

- [x] 8.1 Extend the step editor in `RunbooksPage` for the new kinds and timeout; verify vitest for adding each kind and showing field errors.
- [ ] 8.2 Add the start-run dialog in `RunbookPanel` showing the aggregated preview, blocked reasons and `ElevationConfirm` for `runbook.run`; verify vitest for preview rendering, disabled start on a blocked step and the 409 message.
- [ ] 8.3 Add the live run view driven by `runbook.run.updated` with confirm, resume (with the unknown-step warning) and cancel; verify vitest for each action and state.
- [ ] 8.4 Add the run history list and detail with redacted steps, en and pt-BR strings, and a user documentation page for runs; verify vitest, typecheck and lint.

## 9. Integration

- [ ] 9.1 Run `openspec validate runbook-runs --strict`, backend tests and lint with `GOFLAGS=-p=4 GOMAXPROCS=4`, then web tests, lint and typecheck at concurrency 4, sequentially; record the results.
- [ ] 9.2 Run the app and exercise one run end to end against a test connector: preview, start, manual confirm, forced failure, resume, cancel and history; then run `graphify update .`.
