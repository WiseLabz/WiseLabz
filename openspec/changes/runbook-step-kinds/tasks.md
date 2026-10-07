# Tasks

## 1. Schema and store

- [x] 1.1 Add paired sqlite/postgres migration `000065`: nullable `field_key`, `target_value`, `attribute`, `operator`, `expected_value` on `runbook_steps` and `runbook_run_steps`, with a down migration that deletes steps of kind `config_push`/`wait_for_entity` before dropping the columns; verify the migration up/down test on both drivers.
- [x] 1.2 Extend `RunbookStepRecord`, `RunbookRunStepRecord`, step CRUD, `FreezeSteps` and backup export/import with the five fields; verify store tests, a freeze test that copies them, and a backup round-trip test.

## 2. Config-push core and reader

- [x] 2.1 Add the optional `ConfigReader` interface and a `configRead` capability flag in `internal/connector/connector.go`; verify the capability descriptor test covers a connector with and without it.
- [x] 2.2 Implement `ConfigRead` on each existing pusher (docker, proxmox, cloudflare, pfsense, netbird, pihole, opnsense, dnsresolver) where the value is available from data the connector already fetches; verify a unit test per implemented connector, and file one follow-up GitHub issue listing any pusher left without a reader and why.
- [x] 2.3 Extract `MutateRunbookConfigPush` (writable check, read-and-skip when already at target, fetch/push/fetch/verify, revert when the previous value is known, alert, audit with extra detail) from `config_push.go` and make the HTTP handler a wrapper; verify all existing config-push handler tests pass unchanged.
- [x] 2.4 Add tests calling the core directly: verified write with audit detail, already-at-target with no write and no audit, mismatch with revert and alert, mismatch without a known previous value (no revert, alert), field no longer writable.

## 3. Authoring

- [x] 3.1 Extend `validateSteps`, step input/response and `toStepResponses` for `config_push` (pusher, writable field, entity scope, field type not password/secret, value shape) and `wait_for_entity` (entity, attribute, operator set, regex compile, numeric `gt`/`lt`, 1 to 30 minute timeout); verify a handler test for every authoring scenario in the spec delta.
- [x] 3.2 Confirm single-step execution rejects both new kinds and `ListEntityRunbookSteps` returns them for their entity; verify handler and store tests.
- [x] 3.3 Update OpenAPI `RunbookStep`, `RunbookStepInput`, `RunbookRunStep` and the kind enums, then regenerate the web client; verify the OpenAPI contract test and web typecheck.

## 4. Executor

- [x] 4.1 Add a `ConfigPush` dependency to `runbookrun.Deps` and the `config_push` case in `perform`, reusing `authorize` and `auditDetail`; verify executor tests with a fake for success, frozen value after a runbook edit, already-at-target, mismatch failing the run, field withdrawn, revoked grant, and resume of an `unknown` step.
- [x] 4.2 Export the compliance condition matcher and make the connector entity loader importable from `runbookrun` without importing `quality`; verify existing compliance and quality tests pass unchanged.
- [x] 4.3 Implement `wait_for_entity` with `EntityPollInterval` of 30 seconds, last-observation tracking and timeout reasons; verify tests with fake sync and entities for: holds on first sync, holds on third, entity absent then present, timeout with last value, timeout with entity never found, a failed sync mid-wait, cancel mid-wait.
- [x] 4.4 Wire the new dependencies in `cmd/server/main.go`; verify the server starts and an integration test runs a `config_push` then `wait_for_entity` run to `succeeded` against the fake connector.

## 5. Preview, history and MCP

- [ ] 5.1 Extend the run preview with field, target, current value or `currentValueKnown: false`, and the wait condition; mark a push step not executable when its field is no longer writable; verify handler tests for each preview scenario in the spec delta, including redaction on a hidden connector.
- [ ] 5.2 Include the new fields in run detail/list responses and in `internal/mcp/runbooks.go` with the existing redaction; verify handler and MCP tests with mixed grants.
- [ ] 5.3 Document the run-originated `connector.configPush` audit detail in `docs/AUDIT.md`; verify the audit assertions in 4.1 match the documented fields.
- [ ] 5.4 Write ADR `0007` (config push inside runbook runs: single elevation, frozen value, reader and revert rules) extending 0003 and 0006, linked where the other ADRs are indexed; verify the link resolves.

## 6. Web

- [ ] 6.1 Add both kinds to the step editor in `features/settings/RunbooksPage.tsx`: field picker from the connector's writable fields with a typed value input, entity picker, attribute combobox fed by the compliance schema with free text, operator select, expected value, timeout; verify `RunbooksPage.test.tsx` covers saving each kind and showing server field errors.
- [ ] 6.2 Show current -> target (or "current value unknown") and wait conditions in `StartRunDialog.tsx` and the run view, including the timeout reason; verify component tests for known, unknown and not-executable push steps.
- [ ] 6.3 Add en and pt-BR strings for the new kinds, fields and errors; verify the i18n key parity test and web lint.

## 7. Integration

- [ ] 7.1 Run backend tests, web tests, lint and typecheck with the low-memory settings, then `graphify update .`; verify all pass in CI on the PR.
- [ ] 7.2 Manually run a runbook against a real pusher connector: restart, `wait_for_entity status eq running`, `config_push`; verify the run history, audit log and preview match the spec delta scenarios.
