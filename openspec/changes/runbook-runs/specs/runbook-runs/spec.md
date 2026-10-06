# Spec Delta

## Purpose

Lets operators execute a whole runbook as one approved, recorded run with verification and manual steps, resume it after a failure, and review past runs.

## ADDED Requirements

### Requirement: Step kinds
A runbook step SHALL have one of four kinds: `lifecycle` (restart, start or stop on a connector, optionally on one entity), `sync_and_wait` (sync a connector and wait for the sync to finish), `wait_until_healthy` (wait until a connector's health check reports online) and `manual` (a human confirms the step). Existing steps SHALL be treated as `lifecycle`. A `lifecycle`, `sync_and_wait` or `wait_until_healthy` step SHALL reference an existing connector; a `manual` step SHALL NOT require one. A `sync_and_wait` or `wait_until_healthy` step SHALL have a timeout between 10 seconds and 30 minutes, defaulting to 5 minutes. A runbook SHALL hold at most 20 steps.

#### Scenario: Existing runbook keeps working
- **WHEN** a runbook created before this change is read
- **THEN** each of its steps SHALL be reported with kind `lifecycle` and its verb, connector and entity unchanged.

#### Scenario: Timeout out of range
- **WHEN** a runbook is saved with a `wait_until_healthy` step whose timeout is 45 minutes
- **THEN** the save SHALL be rejected with a field error naming that step's timeout.

#### Scenario: Manual step without connector
- **WHEN** a runbook is saved with a `manual` step that has a title and no connector
- **THEN** the save SHALL succeed.

#### Scenario: Non-lifecycle step executed singly
- **WHEN** the single-step execute endpoint is called for a step whose kind is not `lifecycle`
- **THEN** the request SHALL be rejected and nothing SHALL be executed.

### Requirement: Run preview
The system SHALL provide a dry-run preview of a run that changes nothing. The preview SHALL list the runbook's steps in order with kind, title and target, SHALL include for each lifecycle step the same affected-entity preview a direct lifecycle dry-run returns, and SHALL state for each step whether the caller may execute it and why not. Steps on connectors the caller cannot view SHALL be redacted.

#### Scenario: Preview before elevation
- **WHEN** a user with operator grants on all of a runbook's connectors requests a run preview
- **THEN** the response SHALL list every step in order with its lifecycle previews, no run SHALL be created and no connector SHALL be mutated.

#### Scenario: Preview with a missing grant
- **WHEN** the caller lacks an operator grant on one step's connector
- **THEN** the preview SHALL mark that step as not executable with the reason, and SHALL indicate that the run cannot be started.

### Requirement: Starting a run
Starting a run SHALL require a valid elevation token for the action `runbook.run` scoped to that runbook and an operator grant on every connector referenced by the runbook's steps. A successful start SHALL create a run in state `running`, record the starting user, respond without waiting for the run to finish, and return the run's identifier. A runbook with no steps SHALL NOT be startable.

#### Scenario: Successful start
- **WHEN** an elevated user holding operator on every referenced connector starts a run
- **THEN** the system SHALL respond with status 202 and the run identifier, and the run SHALL begin executing its first step.

#### Scenario: Missing elevation
- **WHEN** a run is started without a valid `runbook.run` elevation token for that runbook
- **THEN** the request SHALL be rejected with the standard elevation error and no run SHALL be created.

#### Scenario: Elevation for another runbook
- **WHEN** a run is started with a `runbook.run` token issued for a different runbook
- **THEN** the request SHALL be rejected and no run SHALL be created.

#### Scenario: Missing grant on one connector
- **WHEN** the caller lacks an operator grant on any connector referenced by the steps
- **THEN** the request SHALL be rejected with status 403 and no step SHALL execute.

#### Scenario: Restricted API key
- **WHEN** an API key restricted to a subset of connectors starts a run that references a connector outside that subset
- **THEN** the request SHALL be rejected with status 403.

### Requirement: Frozen steps
A run SHALL execute and report the steps as they were when the run started. Editing or deleting the runbook or its steps afterwards SHALL NOT change a run's steps, its progress or its history.

#### Scenario: Runbook edited during a run
- **WHEN** a runbook's steps are replaced while one of its runs is waiting on a manual step
- **THEN** resuming that run SHALL execute the steps recorded at start, not the edited ones.

#### Scenario: Runbook deleted
- **WHEN** a runbook with finished runs is deleted
- **THEN** its runs SHALL remain readable with the runbook title and steps recorded at start.

### Requirement: Sequential execution
A run SHALL execute its steps one at a time in order. Each step SHALL be in exactly one state: `pending`, `running`, `waiting`, `succeeded`, `failed`, `skipped` or `unknown`. A lifecycle step SHALL perform the same operation, failure alert and audit record as a direct lifecycle operation on that connector, with the run and step identified in the audit detail, and SHALL NOT require a further elevation. When the last step succeeds the run SHALL become `succeeded`.

#### Scenario: All steps succeed
- **WHEN** every step of a run completes successfully
- **THEN** the run SHALL be `succeeded`, every step SHALL be `succeeded` with start and end times, and each lifecycle step SHALL have an audit record carrying the run identifier.

#### Scenario: Sync step waits
- **WHEN** a `sync_and_wait` step runs
- **THEN** the following step SHALL NOT start until that connector's sync has finished successfully.

#### Scenario: Health step waits
- **WHEN** a `wait_until_healthy` step runs against a connector that is offline and comes online before the timeout
- **THEN** the step SHALL become `succeeded` once a health check reports online, and the run SHALL continue.

### Requirement: Grant re-check during execution
Before executing each step that references a connector, the system SHALL verify that the user who started or last resumed the run still holds an operator grant on that connector. If not, the step SHALL fail without touching the connector.

#### Scenario: Grant revoked mid-run
- **WHEN** the initiating user's operator grant on a later step's connector is revoked while an earlier step is running
- **THEN** that later step SHALL become `failed` with a permission reason, the connector SHALL NOT be mutated and the run SHALL become `failed`.

### Requirement: Failure halts the run
When a step fails or exceeds its timeout, the run SHALL stop, the step SHALL be `failed` with a human-readable reason, the remaining steps SHALL stay `pending`, and the run SHALL become `failed`. No later step SHALL execute until the run is resumed.

#### Scenario: Lifecycle step fails
- **WHEN** the second of four steps fails
- **THEN** the run SHALL be `failed`, step two SHALL be `failed` with the error, and steps three and four SHALL remain `pending` and unexecuted.

#### Scenario: Health step times out
- **WHEN** a `wait_until_healthy` step's connector is still not online when the timeout elapses
- **THEN** the step SHALL be `failed` with a timeout reason and the run SHALL be `failed`.

### Requirement: Manual steps
When a run reaches a `manual` step, the run SHALL become `waiting_manual` and the step `waiting`, and nothing further SHALL execute until a user confirms the step. Confirming SHALL require an operator grant on every connector referenced by the run's steps and SHALL NOT require elevation. On confirmation the step SHALL become `succeeded`, the confirming user SHALL be recorded, and the run SHALL continue.

#### Scenario: Run pauses on manual step
- **WHEN** a run reaches a manual step
- **THEN** the run SHALL be `waiting_manual` and no following step SHALL have started.

#### Scenario: Another operator confirms
- **WHEN** a user other than the initiator, holding operator on all of the run's connectors, confirms the waiting step
- **THEN** the step SHALL be `succeeded` with that user recorded and the run SHALL resume executing.

#### Scenario: Confirm without grants
- **WHEN** a user lacking an operator grant on one of the run's connectors confirms the waiting step
- **THEN** the request SHALL be rejected with status 403 and the run SHALL stay `waiting_manual`.

#### Scenario: Confirm a step that is not waiting
- **WHEN** a confirmation is sent for a step that is not in state `waiting`
- **THEN** the request SHALL be rejected with status 409.

### Requirement: Resuming a failed run
A `failed` run SHALL be resumable. Resuming SHALL require a fresh `runbook.run` elevation token for that runbook and an operator grant on every connector referenced by the run's steps. Resume SHALL execute again from the first step that is not `succeeded`, including a step in state `failed` or `unknown`, and SHALL record the resuming user. Runs in any other state SHALL NOT be resumable.

#### Scenario: Resume after failure
- **WHEN** an elevated, authorized user resumes a run whose third step failed
- **THEN** the run SHALL become `running`, steps one and two SHALL NOT execute again, and step three SHALL execute again.

#### Scenario: Resume without elevation
- **WHEN** a failed run is resumed without a valid elevation token
- **THEN** the request SHALL be rejected and the run SHALL stay `failed`.

#### Scenario: Resume a finished run
- **WHEN** a resume is requested for a `succeeded`, `cancelled` or `expired` run
- **THEN** the request SHALL be rejected with status 409.

### Requirement: Cancelling a run
A run in state `running`, `waiting_manual` or `failed` SHALL be cancellable by any user holding an operator grant on every connector referenced by the run's steps. Cancelling SHALL stop the run before its next step, make the run `cancelled`, mark not-yet-finished steps `skipped`, and record the cancelling user. A cancelled run SHALL NOT be resumable.

#### Scenario: Cancel while waiting
- **WHEN** an authorized user cancels a run that is `waiting_manual`
- **THEN** the run SHALL be `cancelled` and its waiting and pending steps SHALL be `skipped`.

#### Scenario: Cancel while a step runs
- **WHEN** a run is cancelled while a `wait_until_healthy` step is polling
- **THEN** polling SHALL stop, no later step SHALL execute and the run SHALL be `cancelled`.

### Requirement: One active run per runbook
A runbook SHALL have at most one run in state `running`, `waiting_manual` or `failed` at a time. Different runbooks SHALL be able to run concurrently.

#### Scenario: Second start refused
- **WHEN** a run is started for a runbook that already has a `failed` run
- **THEN** the request SHALL be rejected with status 409 identifying the existing run.

#### Scenario: Simultaneous starts
- **WHEN** two start requests for the same runbook arrive at the same time
- **THEN** exactly one run SHALL be created and the other request SHALL receive status 409.

### Requirement: Interruption on restart
When the backend starts, every run found in state `running` SHALL become `failed` with the reason `interrupted`, and the step that was `running` SHALL become `unknown`. The system SHALL NOT execute any step of such a run until a user resumes it.

#### Scenario: Backend restarted mid-run
- **WHEN** the backend stops while a run's lifecycle step is running and then starts again
- **THEN** the run SHALL be `failed` with reason `interrupted`, that step SHALL be `unknown`, and no connector SHALL be mutated until an elevated user resumes the run.

#### Scenario: Waiting run survives restart
- **WHEN** the backend restarts while a run is `waiting_manual`
- **THEN** the run SHALL still be `waiting_manual` and confirmable.

### Requirement: Open run expiry and history retention
The retention settings SHALL include an open-run expiry in hours (default 24) and a run history period in days (default 90). A run in state `waiting_manual` or `failed` with no activity for longer than the expiry SHALL become `expired`, with its unfinished steps `skipped`, and SHALL no longer be resumable or confirmable. Runs in a terminal state older than the history period SHALL be deleted with their steps. A history period of 0 SHALL keep runs indefinitely.

#### Scenario: Forgotten manual step
- **WHEN** a run has been `waiting_manual` for longer than the configured expiry and the retention job runs
- **THEN** the run SHALL be `expired` and a new run of that runbook SHALL be startable.

#### Scenario: Old history pruned
- **WHEN** the retention job runs and a `succeeded` run finished more days ago than the history period
- **THEN** that run and its steps SHALL be deleted.

#### Scenario: Recent activity
- **WHEN** a `failed` run was resumed and failed again one hour ago
- **THEN** the retention job SHALL NOT expire it under a 24 hour expiry.

### Requirement: Run history visibility
Any user who can list runbooks SHALL be able to list a runbook's runs, newest first with pagination, and read a run's detail: state, reason, start and end times, the users who started, resumed, confirmed or cancelled it, and each step's state, times and error. Steps on connectors the caller cannot view SHALL be redacted in the same way runbook steps are redacted. API keys restricted to connectors SHALL see steps only within their restriction.

#### Scenario: Viewer reads history
- **WHEN** a user with viewer grants on all of a run's connectors reads the run
- **THEN** the response SHALL include every step with its state, times and error.

#### Scenario: Step on a hidden connector
- **WHEN** a user without any grant on one step's connector reads the run
- **THEN** that step SHALL be redacted while the run and its other steps remain visible.

### Requirement: Live run updates
The system SHALL publish an event to connected clients whenever a run or one of its steps changes state. An event concerning a step SHALL be delivered only to clients permitted to view that step's connector.

#### Scenario: Step transition
- **WHEN** a step of a run changes from `running` to `succeeded`
- **THEN** a client viewing that run with access to the step's connector SHALL receive an event identifying the run, the step and the new state.

### Requirement: Run notifications
The system SHALL emit a notification event through the existing notification rules when a run becomes `failed` and when a run becomes `waiting_manual`. It SHALL NOT emit one when a run succeeds or is cancelled.

#### Scenario: Run fails
- **WHEN** a run becomes `failed`
- **THEN** a notification naming the runbook, the failed step and the reason SHALL be dispatched to matching rules.

#### Scenario: Run succeeds
- **WHEN** a run becomes `succeeded`
- **THEN** no notification SHALL be dispatched.

### Requirement: Run audit
The system SHALL record an audit entry with the acting user for each of: run started, manual step confirmed, run resumed, run cancelled. Lifecycle steps executed by a run SHALL keep their existing audit action with the run and step identifiers added.

#### Scenario: Run started
- **WHEN** a run is started
- **THEN** an audit entry for the start SHALL exist with the runbook and run identifiers and the starting user.

### Requirement: Read-only run history over MCP
MCP clients SHALL be able to list a runbook's runs and read one run with the same visibility and redaction rules as the HTTP API. MCP SHALL NOT offer any way to start, resume, confirm or cancel a run.

#### Scenario: Agent reads a run
- **WHEN** an MCP client requests a run on behalf of a user who can view all of its connectors
- **THEN** the tool SHALL return the run's state and step outcomes.

#### Scenario: Agent cannot start a run
- **WHEN** the MCP tool list is inspected
- **THEN** it SHALL contain no tool that starts, resumes, confirms or cancels a run.

### Requirement: Single-step execution unchanged
Executing one lifecycle step directly SHALL keep its current behaviour: per-step dry-run preview, per-step elevation, operator grant on the step's connector, and an audit record. A single-step execution SHALL NOT create a run and SHALL NOT appear in run history.

#### Scenario: Single step during an active run
- **WHEN** a lifecycle step is executed directly while the runbook has no active run
- **THEN** the step SHALL execute as before and the runbook's run history SHALL be unchanged.
