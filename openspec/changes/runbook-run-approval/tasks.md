# Tasks

## 1. Persistence and authorization

- [x] 1.1 Add paired SQLite/Postgres approval migrations, preserve existing rows/foreign keys on up and map new states to cancelled on down; verify migration tests.
- [x] 1.2 Persist runbook opt-in and frozen run approval fields/initial state; add conditional approve/reject/withdraw/expiry and open/terminal/prune support; verify store transitions, second-call conflicts, stale-only expiry, one-active request and rejected pruning tests.
- [x] 1.3 Add eligible-approver query covering every distinct connector and connectorless fallback; verify initiator exclusion, disabled users, partial grants, duplicate sources and manual-only cases.
- [x] 1.4 Preserve requiresApproval in backup import; verify JSON and ZIP runbook round trips.

## 2. Executor, notifications and expiry

- [x] 2.1 Implement request without spawn, approve with original actor, reject and approval expiry; verify frozen execution, initiator identity, revoked initiator grants, awaiting resume/confirm conflict and startup isolation tests.
- [x] 2.2 Add NotifyUsers through fanOut and register runbook.run_approval_requested in dispatcher/digest; verify only specified users receive inbox entries and external fanout occurs once.
- [x] 2.3 Expose default-24-hour approval retention and wire per-minute leader expirer; verify retention/API/job tests and published stale-request expiry updates.
- [x] 2.4 Update ADR 0006 and WebSocket event documentation; verify documented behavior against executor and event registration.

## 3. API and contract

- [x] 3.1 Add runbook opt-in create/update/response, preview flags and approval-aware start with eligible check before consuming elevation; verify 202 without execution and no-approver 409 retaining the elevation token.
- [x] 3.2 Add approve/reject routes, runbook.approve elevation action and explicit current approver checks; verify initiator/disabled/partial-grant/restricted-key/read-only-key 403, missing/wrong elevation, reject without elevation and double-approve 409 matrix.
- [x] 3.3 Return canApprove/approvalExpiresAt using frozen connectors, preserve ordinary start response, block opted-in single-step execution and audit the three actions; verify awaiting resume/confirm conflicts, ExecuteStep 409 and request/approval/rejection audit assertions.
- [x] 3.4 Update OpenAPI paths, schemas, retention and event lists, regenerate client; verify openapi_contract_test.go including both routes and a repeat gen:api with no generated diff.
- [x] 3.5 Document audit actions without extending the journal allowlist; verify docs/AUDIT.md and unchanged journal allowlist.

## 4. Web

- [x] 4.1 Add second-approver runbook checkbox and approval-hours retention field with en/pt-BR strings; verify runbook/retention page tests and i18n tests.
- [x] 4.2 Add approval notice, request button and unavailable-approver disabled state to StartRunDialog; verify dialog tests.
- [x] 4.3 Add awaiting detail, eligible approval elevation/reject actions, initiator cancel-request, terminal approval/rejection/expiry display and history labels; verify RunDetail, RunPage and panel/history tests.
- [x] 4.4 Register approval-request event in web routing/types/WebSocket provider; verify settings event routing tests and web typecheck.

## 5. Integration and phase 1 delivery

- [x] 5.1 Run sequential locked Go store, executor/notifications/retention/backup, API/server tests and go vet ./...; record fresh results.
- [x] 5.2 Run sequential locked API generation, runbook/settings/i18n vitest at maxWorkers=4, typecheck and lint; record results and generated reproducibility.
- [x] 5.3 Validate runbook-run-approval --strict, update graph without committing GRAPH_REPORT.md, commit and push implementation; verify clean scoped diff and validation.
- [x] 5.4 Open one assigned/labeled conventional-title PR with Closes #650, watch CI and fix until green, then mark draft and record dependency/rebase notice; verify PR metadata and green checks.

## Workflow follow-up

- Phase 2 requires a new dispatch after #618 and #617 merge: rebase on main, settle migration number 000068, adopt auditRun's frozen connector scope parameter and regenerate the client; rerun verification and CI.
- Do not merge or archive this change in this dispatch.
