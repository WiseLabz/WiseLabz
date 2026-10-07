# Proposal

## Why

Runbook steps execute one at a time from the browser, only restart/start/stop are allowed, and the only record of an execution is the audit log (#510). An operator cannot run a remediation end to end, verify that it worked, or look back at what a past execution did.

## What Changes

- Add **runbook runs**: `POST /api/runbooks/{id}/run` starts a server-side run that executes the runbook's steps in order and persists the state of the run and of every step. The run continues when the browser tab is closed.
- A run is approved with **one elevation** (new action `runbook.run`, scoped to the runbook) against an aggregated dry-run preview of all steps. A run executes a frozen copy of the steps as they were at start.
- Add three **step kinds** beside the existing lifecycle step: sync-and-wait, wait-until-healthy (connector level) and manual checkbox. Automated steps have a timeout.
- A failed, timed-out or interrupted run halts and can be **resumed** from the failed step with a fresh elevation. Runs can be cancelled. A backend restart never continues a run on its own.
- Add **run history**: list and detail endpoints, a history view and a live run view in the web app, and a read-only MCP tool.
- Notify through the existing dispatcher when a run fails and when it waits on a manual step.
- Open runs expire and finished runs are pruned after configurable periods in the retention settings.
- The existing single-step execute endpoint and button are unchanged and stay outside run history.

Out of scope, tracked separately: config-push step kind (#649), second approver (#650), entity-level health conditions (#652), starting or resuming runs from MCP, multi-replica execution (#419).

## Capabilities

### New Capabilities
- `runbook-runs`: starting, executing, pausing, resuming, cancelling and recording whole-runbook runs, including step kinds, authorization, elevation, expiry, notifications and history visibility.

### Modified Capabilities

None. No existing spec covers runbooks.

## Impact

- **Database**: new `runbook_runs` and `runbook_run_steps` tables; new `kind`, `timeout_seconds` columns on `runbook_steps`; two new retention settings columns (sqlite and postgres migrations).
- **Backend**: `internal/api/runbooks`, `internal/store/runbook*.go`, new run executor package, `internal/api/connectors/lifecycle.go` (reusable preview and mutate cores), `internal/retention`, `internal/notifications`, `internal/ws`, `internal/mcp/runbooks.go`, backup export/import for the new step columns, startup wiring.
- **API**: new run endpoints and schemas in OpenAPI; `RunbookStep` gains `kind` and `timeoutSeconds`; retention settings gain two fields. Generated web client regenerated. No breaking change.
- **Web**: `RunbookPanel`, `RunbooksPage` step editor, new run dialog, live run view and history, retention settings form, en and pt-BR strings.
- **Docs**: `docs/AUDIT.md` action table, an ADR extending 0001/0002 for `runbook.run`, user documentation for runs.
