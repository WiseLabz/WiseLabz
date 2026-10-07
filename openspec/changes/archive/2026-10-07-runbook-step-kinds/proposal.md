# Proposal

## Why

Runbook runs (#510) can restart things and wait for a connector to come back, but they cannot change a setting or confirm that the one VM or container they just restarted is actually up. Operators finish those two steps by hand outside the run, so the run record is incomplete and the "wait" step passes while the target is still down (GitHub #649, #652).

## What Changes

- Add a `config_push` step kind that writes one whitelisted field through the existing config-push path (ADR 0003) as part of a run, with the target value frozen at run start.
- Add a `wait_for_entity` step kind that re-syncs a connector every 30 seconds until a named attribute of one entity satisfies a condition, within the step timeout. `wait_until_healthy` is unchanged.
- The run preview shows current and target value for every `config_push` step; "current value unknown" when the connector cannot report it.
- A `config_push` step runs under the run's single `runbook.run` elevation, with the same grant re-check, audit record and failure alert as a direct config push. On a verify mismatch it auto-reverts and fails the step.
- Connectors may optionally report the current value of a writable field; this change implements that for the connectors that already support config push where the value is available from their snapshot.
- Runbook editor, start dialog and run view in the web app learn the two kinds. MCP run history shows them read-only.

Not in this change: a second approver for runs (#650), multi-field pushes in one step, configurable polling interval, stricter elevation for config writes.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `runbook-runs`: two new step kinds with their authoring constraints; the run preview and sequential-execution requirements gain config-push and entity-wait behaviour; audit covers config-push steps.

## Impact

- Backend: `internal/connector` (optional current-value reader), `internal/api/connectors/config_push.go` (push/verify/revert core shared with runs), `internal/runbookrun` (two step executors), `internal/api/runbooks` (validation, preview), `internal/store` (step columns, migration `000065`, backup round-trip), `internal/mcp/runbooks.go`, `docs/openapi.yaml`, `docs/AUDIT.md`, a new ADR extending 0003 and 0006.
- Web: `features/settings/RunbooksPage.tsx`, `components/runbook/StartRunDialog.tsx`, run view, i18n (en, pt-BR), regenerated API client.
- Load: an entity wait runs one connector sync every 30 seconds for at most 30 minutes per step.
- No breaking changes: existing runbooks, runs and the direct config-push endpoint behave as before.
