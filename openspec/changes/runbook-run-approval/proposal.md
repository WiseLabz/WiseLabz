# Proposal

## Why

Runbook elevation currently lets the initiating operator approve their own execution. Sensitive runbooks need an optional second operator to review the exact frozen steps before any work starts (#650).

## What Changes

- Add a per-runbook `requiresApproval` opt-in, preserved by backup and restore.
- Create opted-in runs and their frozen steps immediately in `awaiting_approval`, without executing. Require an eligible approver before consuming the initiator's elevation and notify only eligible users.
- Add approve/reject routes with current operator access on every frozen connector, disabled-user and read-only-key checks, and a prohibition on self-approval or self-rejection. Approval requires the approver's own `runbook.approve` elevation targeted at the run ID; rejection requires none.
- Execute approved runs as the initiator, retain existing per-step grant checks, support withdrawal through cancel, and expire requests using a separate default-24-hour retention setting and per-minute leader job.
- Show request, approval, rejection and expiry state in run detail/history and settings; document and audit the three new approval actions.
- Preserve ordinary runbooks' execution and start response shape; refuse single-step execution on opted-in runbooks.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `runbook-runs`: optional second-operator approval, frozen review, action-scoped elevation, atomic transitions, notification, expiry and approval-aware history.

## Impact

Paired SQLite/Postgres migration 000068 (temporarily 000067 during phase 1 if required by migration ordering), runbook/run stores, authorization and executor, routes and elevation allowlist, notification routing, retention settings/job, backup import, OpenAPI and regenerated web client, runbook components/settings/i18n, audit/WebSocket docs and ADR 0006. No new dependencies. This PR stays draft after green CI until #618 and #617 merge and a separately dispatched rebase regenerates the client and adopts scoped audit calls.
