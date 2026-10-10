# 0006 — Runbook runs elevation and execution boundaries

Status: accepted (implementation in progress, #510)

## Context

ADR 0001 established the permission, step-up, audit, dry-run, and rollback
model for lab-mutating operations (`service.restart`), requiring an operator
grant, an action-scoped elevation token (`X-Elevation-Token`), and a mandatory
dry-run preview. ADR 0002 extended this model to `service.start` and
`service.stop`.

Runbooks sequence multiple operational steps across connectors and services.
Previously, steps executed one at a time from the browser, where the caller
interactively supplied an elevation token for each individual lifecycle step.
Whole-runbook execution introduces automated sequential runs
(`POST /api/runbooks/{id}/run`) executing server-side across lifecycle,
sync-and-wait, wait-until-healthy, and manual steps, continuing even if the
browser tab is closed.

Requiring interactive per-step elevation during a whole-runbook run is
incompatible with server-side execution: elevation tokens have a hardcoded 60-second TTL
(`elevationTTL` in `backend/internal/auth/jwt.go`), while a multi-step run involving reboots,
syncs, and health verification may span several minutes. However, executing
mutations on lab infrastructure without upfront elevated confirmation would
violate ADR 0001's security boundary: a run must never mutate the lab without an
elevated human having approved exactly the steps it executes.

This ADR defines the elevation, authorization, and interruption model for
whole-runbook runs, extending ADR 0001 and ADR 0002.

## Decision

### One elevation for the whole run: `runbook.run`

Starting or resuming a whole-runbook run requires exactly one elevation token
for the new action `runbook.run`, scoped to the target runbook ID:

- Validated with `auth.ValidateElevationHeaderFor`, passing the action
  `runbook.run` and the runbook ID as the target, at run start and again on
  resume.
- The elevation covers exactly the steps shown in the dry-run preview.
- The standard 60-second elevation token TTL bounds only the interval between
  preview confirmation and starting (or resuming) the run; it does not limit the
  run's execution duration.
- This follows the existing `BulkRestart` precedent of validating elevation once
  before invoking underlying lifecycle operations.

### Per-step execution without inner elevation

Steps inside a run execute via elevation-free lifecycle cores extracted from the
connectors handler (`internal/api/connectors/lifecycle.go`):

- The executor does not require or generate per-step elevation tokens
  (`connector.restart`, `connector.start`, `connector.stop`).
- Each lifecycle step executes with the explicit identity of the starting (or
  resuming) user.
- Failure alerts and audit records are generated identically to direct
  lifecycle operations, with the run and step IDs recorded in the audit detail.

### Authorization: operator on all referenced connectors

Starting, resuming, confirming a manual step, and cancelling a run require the
acting user to hold an operator grant on every distinct connector referenced by
the run's frozen steps:

- Connectors are determined from the frozen copy of steps captured at start.
- API-key connector scope restrictions apply across all referenced connectors.
- Ticking a manual step (`POST /api/runbook-runs/{runId}/steps/{stepId}/confirm`)
  and cancelling (`POST /api/runbook-runs/{runId}/cancel`) require the operator
  grant on every connector of the run, but do **not** require elevation.
- If there is no connector to check, the caller must be an instance admin or
  hold an operator grant on at least one connector. This covers manual-only
  runs and cancellation after every connector in a run has been deleted.
- API-key connector restrictions still apply to the fallback operator grant;
  a restricted key cannot use a grant outside its allowed connector set.

### Pre-step operator grant re-check

Before each automated step referencing a connector executes, the system verifies
that the user who started or last resumed the run still holds an operator grant
on that connector:

- If the initiating user's grant was revoked after the run began, the step
  immediately fails with a permission reason without touching the connector.
- The run halts in state `failed`.

### Interruption on restart and recovery

When the backend starts up:

- Every run found in state `running` transitions to `failed` with reason
  `interrupted`.
- The in-flight step that was `running` transitions to `unknown`.
- Runs in state `waiting_manual` survive restarts and remain confirmable.
- The system **never** automatically resumes an interrupted run on restart.
  Because the exact outcome of an in-flight mutation on the lab cannot be
  guaranteed after a crash and no human is present, a human operator must inspect
  the lab and explicitly resume the run with a fresh `runbook.run` elevation
  token.
- An `unknown` step is repeated on resume. It is shown with a warning in the
  resume dialog; restart, start, and stop are safe to repeat.

### Audit

An audit entry with the acting user is recorded for each of: run started,
manual step confirmed, run resumed, and run cancelled. Each entry carries the
runbook and run identifiers. Lifecycle steps executed by a run keep their
existing `connector.<verb>` audit action, with the run and step identifiers
added in `detail`.

### Dry-run preview

`POST /api/runbooks/{id}/run?dryRun=true` produces a preview that changes
nothing: no run is created and no connector is mutated.

- Lists the runbook's steps in order with kind, title, and target.
- Includes, for each lifecycle step, the same affected-entity preview a direct
  lifecycle dry-run returns.
- States for each step whether the caller may execute it and, if not, why.
- Redacts steps on connectors the caller cannot view.

### Rollback expectations

Consistent with ADR 0001 and ADR 0002, there is no automatic rollback of
completed steps. If a step fails, the run halts immediately, remaining steps
remain `pending`, and the operator may inspect the state, fix underlying issues,
and resume from the failed step.

## Consequences

- Extends ADR 0001 and ADR 0002 to asynchronous composite workflows without
  weakening security boundaries.
- Operators confirm the full scope of mutations once at start, rather than
  scrambling to re-elevate across 60-second windows during automated sequences.
- Permissions are re-checked immediately before each step. A grant revoked
  after the run started can still allow the step already in flight, because the
  re-check happens only before a step begins.
- Crashes and restarts fail safely to `interrupted` / `unknown`, requiring human
  supervision and re-elevation to continue.

### Optional second-operator approval (#650)

A runbook may opt into `requiresApproval`. Starting it checks the initiator's
operator grants, existence and nonempty steps, then availability of a different
eligible enabled operator before consuming `runbook.run` elevation. The run and
frozen steps are created in `awaiting_approval`; nothing executes yet. The
single-step endpoint returns `409 approval_required` for these runbooks.

Approve and reject use current operator grants on every frozen connector, apply
API-key restrictions and reject read-only keys, disabled users and the initiator.
Connectorless runs retain the instance-admin or any-connector-operator fallback.
Approve requires the approver's own `runbook.approve` elevation targeted at the
run ID. Reject requires none. These elevation actions are distinct: approval
cannot use `runbook.run`, and start/resume cannot use `runbook.approve`.

Approval stores `approvedBy`/`approvedAt`, publishes and starts the frozen run as
the initiator, retaining the per-step initiator grant recheck. Rejection stores
`rejectedBy` and `finishedAt`, skips pending steps and is terminal. The initiator
can withdraw through cancel with its existing guards. Each transition from
`awaiting_approval` conditionally updates that state; repeated or competing
transitions conflict. Resume, manual confirmation and startup recovery do not
execute awaiting runs.

The separate `runbookApprovalHours` retention setting defaults to 24 hours. A
per-minute leader job expires stale requests with reason `approval_expired`,
skips their pending steps and publishes the update. Existing open-run expiry
remains unchanged; terminal rejected runs follow finished-run retention.
Eligible approvers receive `runbook.run_approval_requested` via the dispatcher,
with external delivery once per request. Request, approval and rejection audit
the acting user without adding those actions to the Journal allowlist. Backup
and restore preserve the authored approval opt-in; run history stays operational.

Approval follows the existing instance-wide step-up setting, like start and
resume: when step-up is enabled, `runbook.approve` is required and the action,
user and run ID binding is enforced; when the instance disables step-up, the
existing elevation helper skips token validation. Disabling step-up never
bypasses the different-operator, enabled-account, all-frozen-connector, API-key
or conditional-transition guards. Whether second-operator approval should
always require elevation regardless of this setting is left for explicit user
confirmation, rather than changing the existing instance setting semantics.
