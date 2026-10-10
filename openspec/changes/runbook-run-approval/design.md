# Design

## Context

See proposal.md for motivation. Whole runs already persist frozen steps, use runbook.run elevation, authorize every connector, and recheck the acting user's grants before each step. The executor exposes tracked asynchronous execution; notifications share fanOut; retention settings are stored in a singleton row. Current main includes connector_action steps, which the snapshot and migration must preserve.

## Goals / Non-Goals

**Goals:** bind a different operator's approval to immutable requested steps, retain the initiator as execution actor, serialize every transition from awaiting_approval in the database, and preserve ordinary run behavior.

**Non-Goals:** changing journal action visibility, persisting approval in backups of operational history, automatic approval or recovery, new dependencies, or starting phase 2 without a new dispatch.

## Decisions

The approved 2026-10-09 implementation contract below is binding. Existing authorization, dispatcher fanOut, scheduler leader gating and table-rebuild patterns are reused rather than introducing another policy, queue or notification mechanism. Frozen run rows are created at request time rather than referencing mutable authored steps; state-conditional updates replace process-only locks. A separate runbook.approve token distinguishes second-operator consent from the initiator's runbook.run consent. Request expiry uses its own setting rather than the existing open-run retention period.


### State machine

New states: `awaiting_approval` (open, never executes) and `rejected` (terminal).

| Transition | Trigger | Guards and effects |
|---|---|---|
| none to `awaiting_approval` | `POST /runbooks/{id}/run` on an opted-in runbook | In order: `runAuthorized`, runbook exists, at least one step, at least one eligible approver (else `409 no_eligible_approver`, checked before elevation so no token is burned), initiator's `runbook.run` elevation, one-active index. Audit `runbook.run.approval_requested`, notify approvers, 202. |
| to `running` | `POST /runbook-runs/{runId}/approve` | Approver guards plus `runbook.approve` elevation on the run id. Sets `approved_by`, `approved_at`; `publishRun` and `spawn`. Audit `runbook.run.approved`. |
| to `rejected` | `POST /runbook-runs/{runId}/reject` | Approver guards, no elevation. Steps skipped, `rejected_by`, `finished_at`. Audit `runbook.run.rejected`. |
| to `cancelled` | existing cancel endpoint | Today's guards; the initiator's withdrawal. |
| to `expired` | per-minute leader job | `updated_at` older than `runbook_approval_hours`; `reason = 'approval_expired'`, steps skipped, run update published. |

Every transition out of `awaiting_approval` is `UPDATE ... WHERE id = ? AND state = 'awaiting_approval'`; zero rows returns `409 conflict`. Approver guards: `runAuthorized` on the frozen steps (operator on every connector, API-key clamp, read-only keys refused), not `run.StartedBy`, not disabled. Resume, confirm, `InterruptRunningRunbookRuns`, `ExpireOpenRunbookRuns` and the per-step grant recheck are unchanged. `ExecuteStep` (`api/runbooks/handlers.go:1106`) returns `409 approval_required` for an opted-in runbook.

### Migration `000068_runbook_run_approval`

- `runbooks.requires_approval INTEGER NOT NULL DEFAULT 0 CHECK (requires_approval IN (0,1))`.
- `runbook_runs`: `requires_approval`, `approved_by`, `approved_at`, `rejected_by`; state CHECK with the eight states; `idx_runbook_runs_one_active` recreated to include `awaiting_approval`.
- `retention_settings.runbook_approval_hours INTEGER NOT NULL DEFAULT 24`.
- Postgres: drop and re-add `runbook_runs_state_check` (as `000065` does for `runbook_run_steps_kind_check`). SQLite: rebuild with the `000064` pattern (create `_new`, copy, drop, rename), not rename-to-`_old`, because `runbook_runs` is a parent table.
- Down: map `awaiting_approval` and `rejected` rows to `cancelled` (lossy, as the `000062` down documents), then restore the old check and index and drop the columns.

### Backend

1. `backend/internal/store/runbook.go:24,109`: `RequiresApproval` on `RunbookRecord` and in `UpdateRunbook`.
2. `backend/internal/store/runbook_run.go`: `CreateRunbookRun` takes the initial state and flag; new `ApproveRunbookRun`, `RejectRunbookRun`, `ExpireAwaitingApprovalRunbookRuns` (modelled on `:522-551`); open/terminal predicates (`:834-840`), `CancelRunbookRun` and `PruneRunbookRuns` learn the new states.
3. `backend/internal/store/connector_permission.go`: new `EligibleRunbookApprovers(ctx, connectorIDs, excludeUserID)`:
   ```sql
   SELECT u.id FROM users u
   WHERE u.disabled = 0 AND u.id <> ?
     AND (SELECT COUNT(DISTINCT g.connector_id) FROM user_connector_roles g
          WHERE g.user_id = u.id AND g.role = 'operator'
            AND g.connector_id IN (?, ...)) = ?
   ```
   For a manual-only runbook it mirrors `connectorlessRunAuthorized` (`api/runbooks/runs.go:106-111`).
4. `backend/internal/runbookrun/executor.go`: constants; `Request` (`Start` without `spawn`), `Approve`, `Reject`, `ExpireApprovals`.
5. `backend/internal/api/runbooks/runs.go`: `StartRun` branches on the flag; `previewRun` adds `requiresApproval` and `approverAvailable`; new `ApproveRun`, `RejectRun`; `RunResponse` adds `canApprove` and `approvalExpiresAt`. `handlers.go:888-971,165`: `requiresApproval` in create, update and response.
6. Routes in `backend/internal/api/routes_workflow.go:55-58`; `runbook.approve` in `validElevationAction` (`backend/internal/api/auth/session.go:293-303`).
7. Notifications: new `Dispatcher.NotifyUsers(ctx, userIDs, eventType, severity, title, message)` in `backend/internal/notifications/dispatcher.go` reusing `fanOut`, because `NotifyRunbookRun` targets any grant holder and looping `NotifyAlert` would send external channels once per approver. Register `runbook.run_approval_requested` in `dispatcher.go:218-224`, `digest.go:96`, `web/src/features/settings/EventRoutingTable.tsx:43-44`, `web/src/types/ws.ts:30-31,195-196`, `web/src/ws/WebSocketProvider.tsx:325-326`, `docs/openapi.yaml:7231,7505`.
8. Retention setting in `backend/internal/store/retention_settings.go` and `backend/internal/api/system/retention.go`; job `runbookApprovalExpirer` on `"0 * * * * *"` in `backend/cmd/server/main.go` next to `alertExpirer` (`:366`).
9. Backup: `backend/internal/backup/backup.go` exports `store.RunbookRecord`; make `importRunbooks` (`:752`) persist `requiresApproval` so a restore does not silently drop the opt-in, and extend `TestRunbookBackupJSONAndZIPRoundTrip` (`backup_test.go:817`).

### Contract and web

- `docs/openapi.yaml`: two new paths, state enum (`:6838-6840`), run, preview and runbook schemas (`:6342`, `:6854-6953`), retention settings, event lists, start prose (`:3427`). Then `bun run gen:api`.
- `web/src/features/settings/RunbooksPage.tsx:~547-650`: "Require a second approver" checkbox. `RetentionPage.tsx`: approval-hours field.
- `web/src/components/runbook/StartRunDialog.tsx`: when `preview.requiresApproval`, a notice that nothing runs until another operator approves, a "Request approval" button, and a disabled state with a message when `approverAvailable` is false.
- `web/src/components/runbook/RunDetail.tsx`: for `awaiting_approval`, a badge, requester, request time and expiry, steps pending, no resume or confirm; Approve (`ElevationConfirm` with `runbook.approve` on the run id) and Reject when `canApprove`; "Cancel request" for the initiator. `rejected`, approval-expired and "Approved by" are shown on finished runs. State labels in `RunHistory.tsx` and `RunbookPanel.tsx`.
- Strings in `en.ts:987` and `pt-BR.ts:49`.
- Docs: `docs/AUDIT.md`, `docs/WS_CONTRACT.md`, `docs/adr/0006-runbook-runs-elevation-and-execution.md`; a new OpenSpec change `openspec/changes/runbook-run-approval/` (the runbook-runs change is archived).

### Tests

- Store: conditional approve and reject (second call conflicts); expiry touches only stale requests; one-active index blocks a second request; prune removes `rejected`; eligible-approver query (initiator, disabled user, partial grants, duplicate grant sources, manual-only).
- Migration (`runbook_run_migrations_test.go`): rows survive the SQLite rebuild with foreign keys on; down maps the new states.
- API (`backend/internal/api/runbooks/runs_test.go`): opted-in start returns 202 with nothing executed; no approver 409 without consuming elevation; initiator, partial-grant approver and read-only key get 403; approve without elevation refused; reject without elevation succeeds; double approve 409; resume and confirm on an awaiting run 409; `ExecuteStep` 409; audit rows for request, approval and rejection.
- Executor: an approved run executes as the initiator and fails if the initiator's grant was revoked. Dispatcher: `NotifyUsers` reaches only the given users. Expirer job. Contract test.
- Web: `StartRunDialog.test.tsx`, `RunDetail`, `RunPage.test.tsx`, runbooks and retention pages.

Verify: in `backend/`, `GOFLAGS=-p=4 GOMAXPROCS=4 go test ./internal/store/...`, then `./internal/runbookrun/... ./internal/notifications/... ./internal/retention/... ./internal/backup/...`, then `./internal/api/... ./cmd/server/...`, then `go vet ./...`; in `web/`, `bun run gen:api && git diff --stat -- src/api/generated src/api/model`, `bunx vitest run --maxWorkers=4 src/components/runbook src/features/settings src/i18n`, `bun run typecheck`, `bun run lint`. Never run suites in parallel.


## Risks / Trade-offs

- An unanswered request blocks another run of the same runbook → cancellation and default 24-hour request expiry release the one-active index.
- Runbook edits after request could change approval meaning → all approval authorization and displayed steps use the frozen snapshot.
- Grants or enabled status can change → check the approver at approval/rejection time and recheck the initiator before execution.
- SQLite parent-table rebuild could corrupt child foreign keys → create/copy/drop/rename with the migration runner's FK-off protocol and verify surviving children with FK enforcement enabled.
- #618 and #617 share audit and generated surfaces → keep local edits small, leave phase 1 draft after green CI and regenerate after their merge.

## Migration Plan

Use paired up/down SQLite and PostgreSQL migrations numbered 000068. If the absent 000067 gap prevents migration tests, use 000067 in phase 1 and rename all four files and test literals to 000068 in the separately dispatched phase 2. Existing rows default to approval disabled. Rollback maps awaiting_approval and rejected to cancelled before restoring the old state check/index and dropping approval columns; this is deliberately lossy. Preserve frozen step rows throughout SQLite's parent-table rebuild.

## Instance step-up exemption

Approval always requires elevation (`runbook.approve`), regardless of the
instance-wide step-up toggle, joining `mfa.manage` in the step-up exemption
set in `ValidateElevationHeaderFor`. API keys can therefore never approve a
runbook run. Start, resume, reject and cancel remain unchanged. Disabling
instance step-up never bypasses approval elevation.
