# 0006 — Runbook runs elevation and execution boundaries

Status: accepted

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
incompatible with server-side execution: elevation tokens have a 60-second TTL
(`backend/internal/auth/jwt.go:52`), while a multi-step run involving reboots,
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

- Validated via `auth.ValidateElevationHeaderFor(r, "runbook.run", runbookID)`
  at run start and again on resume.
- The elevation covers an aggregated dry-run preview of all steps in the
  runbook.
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

### Audit

Audit records are logged for run lifecycle transitions:

| Action | Trigger | targetType / targetId |
|---|---|---|
| `runbook.run` | `POST /api/runbooks/{id}/run` | runbook / id |
| `runbook.resume` | `POST /api/runbook-runs/{runId}/resume` | runbook / id |
| `runbook.confirm` | `POST /api/runbook-runs/{runId}/steps/{stepId}/confirm` | runbook / id |
| `runbook.cancel` | `POST /api/runbook-runs/{runId}/cancel` | runbook / id |

In addition, individual lifecycle steps emit standard `connector.<verb>` audit
events with the run and step identifiers in `detail`.

### Dry-run preview

`POST /api/runbooks/{id}/run?dryRun=true` produces an aggregated dry-run
preview:

- Lists all frozen steps in sequence.
- Includes affected-entity previews for each lifecycle step matching direct
  lifecycle dry-runs.
- Evaluates per-step execution permissions and identifies any missing grants.
- A dry-run preview is mandatory before a run can be initiated from the UI.

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
- Permissions remain strictly enforced throughout execution via pre-step grant
  re-checks.
- Crashes and restarts fail safely to `interrupted` / `unknown`, requiring human
  supervision and re-elevation to continue.
