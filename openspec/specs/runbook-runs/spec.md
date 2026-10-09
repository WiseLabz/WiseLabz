# runbook-runs Specification

## Purpose
Lets operators execute a whole runbook as one approved, recorded run with verification and manual steps, resume it after a failure, and review past runs.

## Requirements

### Requirement: Step kinds
A runbook step SHALL have one of six kinds: `lifecycle` (restart, start or stop on a connector, optionally on one entity), `sync_and_wait` (sync a connector and wait for the sync to finish), `wait_until_healthy` (wait until a connector's health check reports online), `manual` (a human confirms the step), `config_push` (write one whitelisted configuration field on a connector, optionally on one entity) and `wait_for_entity` (wait until an attribute of one entity satisfies a condition). Existing steps SHALL be treated as `lifecycle`.

#### Scenario: Existing runbook keeps working
- **WHEN** a runbook created before this change is read
- **THEN** each of its steps SHALL be reported with kind `lifecycle` and its verb, connector and entity unchanged.

#### Scenario: New kinds are stored and returned
- **WHEN** a runbook is saved with one `config_push` step and one `wait_for_entity` step and then read
- **THEN** the steps SHALL be returned with their kinds and with the field key, target value, entity, attribute, operator and expected value as saved.

### Requirement: Step constraints
A `lifecycle`, `sync_and_wait`, `wait_until_healthy`, `config_push` or `wait_for_entity` step SHALL reference an existing connector; a `manual` step SHALL NOT require one. A `sync_and_wait` or `wait_until_healthy` step SHALL have a timeout between 10 seconds and 30 minutes, defaulting to 5 minutes. A `wait_for_entity` step SHALL have a timeout between 1 minute and 30 minutes, defaulting to 5 minutes. A runbook SHALL hold at most 20 steps.

#### Scenario: Timeout out of range
- **WHEN** a runbook is saved with a `wait_until_healthy` step whose timeout is 45 minutes
- **THEN** the save SHALL be rejected with a field error naming that step's timeout.

#### Scenario: Manual step without connector
- **WHEN** a runbook is saved with a `manual` step that has a title and no connector
- **THEN** the save SHALL succeed.

#### Scenario: Non-lifecycle step executed singly
- **WHEN** the single-step execute endpoint is called for a step whose kind is not `lifecycle`
- **THEN** the request SHALL be rejected and nothing SHALL be executed.

#### Scenario: Entity wait timeout below one minute
- **WHEN** a runbook is saved with a `wait_for_entity` step whose timeout is 30 seconds
- **THEN** the save SHALL be rejected with a field error naming that step's timeout.

### Requirement: Run preview
The system SHALL provide a dry-run preview of a run that changes nothing. The preview SHALL list the runbook's steps in order with kind, title and target, SHALL include for each lifecycle step the same affected-entity preview a direct lifecycle dry-run returns, and SHALL state for each step whether the caller may execute it and why not. For each `config_push` step the preview SHALL show the field, the target value and the field's current value, or SHALL state that the current value is unknown when the connector cannot report it or cannot be reached. For each `wait_for_entity` step the preview SHALL show the entity, attribute, operator and expected value. Steps on connectors the caller cannot view SHALL be redacted.

#### Scenario: Preview before elevation
- **WHEN** a user with operator grants on all of a runbook's connectors requests a run preview
- **THEN** the response SHALL list every step in order with its lifecycle previews, no run SHALL be created and no connector SHALL be mutated.

#### Scenario: Preview with a missing grant
- **WHEN** the caller lacks an operator grant on one step's connector
- **THEN** the preview SHALL mark that step as not executable with the reason, and SHALL indicate that the run cannot be started.

#### Scenario: Config push shows current and target
- **WHEN** a preview is requested for a runbook with a `config_push` step on a connector that can report the field's current value
- **THEN** the step SHALL show the current value and the target value, and the connector SHALL NOT be written to.

#### Scenario: Current value unavailable
- **WHEN** a preview is requested for a `config_push` step on a connector that cannot report the field's current value
- **THEN** the step SHALL show the target value and state that the current value is unknown, and the run SHALL still be startable.

#### Scenario: Config push step no longer valid
- **WHEN** a preview is requested and a `config_push` step's field is no longer writable on its connector
- **THEN** the preview SHALL mark that step as not executable with the reason, and SHALL indicate that the run cannot be started.

### Requirement: Starting a run
Starting a run SHALL require a valid elevation token for the action `runbook.run` scoped to that runbook and an operator grant on every connector referenced by the runbook's steps. When the runbook references no connectors, starting SHALL instead require an instance admin or an operator grant on at least one connector. A successful start SHALL create a run in state `running`, record the starting user, respond without waiting for the run to finish, and return the run's identifier. A runbook with no steps SHALL NOT be startable.

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

#### Scenario: Start a connectorless run
- **WHEN** an instance admin or a user with operator access on any connector starts a runbook whose steps reference no connectors
- **THEN** the request SHALL be authorized after validating the runbook elevation token.

#### Scenario: Start a connectorless run without fallback access
- **WHEN** a caller with no operator grant on any connector starts a runbook whose steps reference no connectors
- **THEN** the request SHALL be rejected with status 403 before elevation is checked.

### Requirement: Frozen steps
A run SHALL execute and report the steps as they were when the run started. Editing or deleting the runbook or its steps afterwards SHALL NOT change a run's steps, its progress or its history.

#### Scenario: Runbook edited during a run
- **WHEN** a runbook's steps are replaced while one of its runs is waiting on a manual step
- **THEN** resuming that run SHALL execute the steps recorded at start, not the edited ones.

#### Scenario: Runbook deleted
- **WHEN** a runbook with finished runs is deleted
- **THEN** its runs SHALL remain readable with the runbook title and steps recorded at start.

### Requirement: Sequential execution
A run SHALL execute its steps one at a time in order. Each step SHALL be in exactly one state: `pending`, `running`, `waiting`, `succeeded`, `failed`, `skipped` or `unknown`. A lifecycle step SHALL perform the same operation, failure alert and audit record as a direct lifecycle operation on that connector, with the run and step identified in the audit detail, and SHALL NOT require a further elevation. A `config_push` step SHALL write its field as described in "Config-push step execution" and SHALL NOT require a further elevation. A `wait_for_entity` step SHALL wait as described in "Entity wait step execution". When the last step succeeds the run SHALL become `succeeded`.

#### Scenario: All steps succeed
- **WHEN** every step of a run completes successfully
- **THEN** the run SHALL be `succeeded`, every step SHALL be `succeeded` with start and end times, and each lifecycle step SHALL have an audit record carrying the run identifier.

#### Scenario: Sync step waits
- **WHEN** a `sync_and_wait` step runs
- **THEN** the following step SHALL NOT start until that connector's sync has finished successfully.

#### Scenario: Health step waits
- **WHEN** a `wait_until_healthy` step runs against a connector that is offline and comes online before the timeout
- **THEN** the step SHALL become `succeeded` once a health check reports online, and the run SHALL continue.

#### Scenario: Config push followed by entity wait
- **WHEN** a run holds a `config_push` step followed by a `wait_for_entity` step and both complete
- **THEN** the wait SHALL NOT start until the push has succeeded, and the run SHALL be `succeeded`.

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
When a run reaches a `manual` step, the run SHALL become `waiting_manual` and the step `waiting`, and nothing further SHALL execute until a user confirms the step. Confirming SHALL require an operator grant on every connector referenced by the run's steps; if no connector is referenced, it SHALL require an instance admin or operator access on at least one connector. Confirmation SHALL NOT require elevation. On confirmation the step SHALL become `succeeded`, the confirming user SHALL be recorded, and the run SHALL continue.

#### Scenario: Run pauses on manual step
- **WHEN** a run reaches a manual step
- **THEN** the run SHALL be `waiting_manual` and no following step SHALL have started.

#### Scenario: Another operator confirms
- **WHEN** a user other than the initiator, holding operator on all of the run's connectors, confirms the waiting step
- **THEN** the step SHALL be `succeeded` with that user recorded and the run SHALL resume executing.

#### Scenario: Confirm without grants
- **WHEN** a user lacking an operator grant on one of the run's connectors confirms the waiting step
- **THEN** the request SHALL be rejected with status 403 and the run SHALL stay `waiting_manual`.

#### Scenario: Confirm a connectorless run
- **WHEN** an instance admin or a user with operator access on any connector confirms a manual step in a run whose steps reference no connectors
- **THEN** the step SHALL be confirmed and the run SHALL continue.

#### Scenario: Confirm a step that is not waiting
- **WHEN** a confirmation is sent for a step that is not in state `waiting`
- **THEN** the request SHALL be rejected with status 409.

### Requirement: Resuming a failed run
A `failed` run SHALL be resumable. Resuming SHALL require a fresh `runbook.run` elevation token for that runbook and an operator grant on every connector referenced by the run's steps; if no connector is referenced, it SHALL require an instance admin or operator access on at least one connector. Resume SHALL execute again from the first step that is not `succeeded`, including a step in state `failed` or `unknown`, and SHALL record the resuming user. Runs in any other state SHALL NOT be resumable.

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
A run in state `running`, `waiting_manual` or `failed` SHALL be cancellable by any user holding an operator grant on every connector referenced by the run's steps that still exists. If no connector remains to check, cancellation SHALL require an instance admin or operator access on at least one connector. Cancelling SHALL stop the run before its next step, make the run `cancelled`, mark not-yet-finished steps `skipped`, and record the cancelling user. A cancelled run SHALL NOT be resumable.

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
The system SHALL record an audit entry with the acting user for each of: run started, manual step confirmed, run resumed, run cancelled. Lifecycle steps executed by a run SHALL keep their existing audit action with the run and step identifiers added. A `config_push` step that writes SHALL record the same audit action as a direct config push, with the field key, the entity, and the run and step identifiers.

#### Scenario: Run started
- **WHEN** a run is started
- **THEN** an audit entry for the start SHALL exist with the runbook and run identifiers and the starting user.

#### Scenario: Config push in a run is audited
- **WHEN** a `config_push` step of a run writes its field successfully
- **THEN** a config-push audit entry SHALL exist for that connector with the field key, the acting user and the run and step identifiers.

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

### Requirement: Config-push step authoring
A `config_push` step SHALL name a connector, a field key and a target value. Saving SHALL be rejected when the connector does not support config push, when the field key is not one of the connector's writable fields, when the field is entity-scoped and the step names no entity, when the field is of a password or secret type, or when the target value does not fit the field's type. The target value SHALL be visible to every user who can view the step.

#### Scenario: Field not writable
- **WHEN** a runbook is saved with a `config_push` step whose field key is not in the connector's writable fields
- **THEN** the save SHALL be rejected with a field error naming that step's field.

#### Scenario: Connector without config push
- **WHEN** a runbook is saved with a `config_push` step on a connector that does not support config push
- **THEN** the save SHALL be rejected with a field error naming that step's connector.

#### Scenario: Entity-scoped field without entity
- **WHEN** a runbook is saved with a `config_push` step for an entity-scoped field and no entity
- **THEN** the save SHALL be rejected with a field error naming that step's entity.

#### Scenario: Valid config-push step
- **WHEN** a runbook is saved with a `config_push` step naming a writable toggle field, an entity and the value `true`
- **THEN** the save SHALL succeed.

### Requirement: Config-push step execution
A `config_push` step SHALL write the target value that was recorded when the run started, to the field and entity recorded when the run started. Before writing, the system SHALL verify the field is still writable on the connector and SHALL fail the step without writing if it is not. When the connector reports that the field already holds the target value, the step SHALL succeed without writing. After writing, the system SHALL verify the write took effect: it took effect when the connector's documented state changed, or, when that state did not change, when the connector can report the field's value and reports the target value. When it did not and the value held before the write is known, the system SHALL write that previous value back, raise the same alert a direct config push raises for a mismatch, and fail the step. When it did not and the previous value is not known, the system SHALL raise the alert and fail the step without writing again.

#### Scenario: Push succeeds
- **WHEN** a `config_push` step runs and the write is verified
- **THEN** the step SHALL be `succeeded` and the run SHALL continue with the next step.

#### Scenario: No visible change but the connector confirms the target
- **WHEN** a `config_push` step's write changes nothing in the connector's documented state and the connector then reports the target value for the field
- **THEN** the step SHALL be `succeeded`, no alert SHALL be raised and no further write SHALL be made.

#### Scenario: Value frozen at start
- **WHEN** a runbook's `config_push` step is edited to a different target value while a run started earlier is waiting on a manual step before it
- **THEN** the run SHALL write the value recorded at start.

#### Scenario: Already at target
- **WHEN** a `config_push` step runs and the connector reports the field already equals the target value
- **THEN** the step SHALL be `succeeded`, the connector SHALL NOT be written to and no config-push audit entry SHALL be recorded for the step.

#### Scenario: Write does not verify, previous value known
- **WHEN** a `config_push` step's write is not verified and the previous value was read before the write
- **THEN** the previous value SHALL be written back, a critical alert describing the mismatch SHALL be raised, the step SHALL be `failed` and the run SHALL be `failed`.

#### Scenario: Write does not verify, previous value unknown
- **WHEN** a `config_push` step's write is not verified on a connector that cannot report the field's current value
- **THEN** no revert SHALL be attempted, a critical alert SHALL be raised, the step SHALL be `failed` with a reason saying the value could not be verified and the run SHALL be `failed`.

#### Scenario: Field withdrawn before execution
- **WHEN** a `config_push` step is reached and its field is no longer writable on the connector
- **THEN** the step SHALL be `failed` with that reason and the connector SHALL NOT be written to.

#### Scenario: Resume after interruption
- **WHEN** a run is resumed whose `config_push` step was left `unknown` by a backend restart
- **THEN** the step SHALL execute again, succeeding without a write if the field already holds the target value.

### Requirement: Entity wait step authoring
A `wait_for_entity` step SHALL name a connector, one entity of that connector, an attribute name, an operator and an expected value. The operator SHALL be one of the operators compliance rules support: `eq`, `neq`, `contains`, `regex`, `gt`, `lt`. Saving SHALL be rejected when the entity, attribute or operator is missing, when the operator is not supported, when a `regex` value does not compile, or when a `gt` or `lt` value is not a number. The system SHALL expose the attribute names a connector type declares so that authoring can offer them; an attribute name outside that list SHALL still be accepted.

#### Scenario: Valid entity wait
- **WHEN** a runbook is saved with a `wait_for_entity` step for a virtual machine, attribute `status`, operator `eq` and value `running`
- **THEN** the save SHALL succeed.

#### Scenario: Unsupported operator
- **WHEN** a runbook is saved with a `wait_for_entity` step whose operator is `startswith`
- **THEN** the save SHALL be rejected with a field error naming that step's operator.

#### Scenario: Invalid regular expression
- **WHEN** a runbook is saved with a `wait_for_entity` step with operator `regex` and a value that does not compile
- **THEN** the save SHALL be rejected with a field error naming that step's value.

#### Scenario: Attribute not in the declared list
- **WHEN** a runbook is saved with a `wait_for_entity` step whose attribute is not among those the connector type declares
- **THEN** the save SHALL succeed.

### Requirement: Entity wait step execution
A `wait_for_entity` step SHALL sync its connector, then evaluate the condition against the named entity in the result, and SHALL repeat every 30 seconds until the condition holds or the step timeout elapses. Conditions SHALL be evaluated with the same comparison rules compliance rules use. Each sync SHALL be an ordinary recorded sync of that connector. An entity absent from a sync, or a sync that fails, SHALL count as the condition not holding yet. On timeout the step SHALL fail with a reason stating either that the entity was not found or the attribute's last observed value. Cancelling the run SHALL stop the wait.

#### Scenario: Condition already holds
- **WHEN** a `wait_for_entity` step runs and the first sync shows the entity's attribute satisfying the condition
- **THEN** the step SHALL be `succeeded` without further syncs.

#### Scenario: Condition holds later
- **WHEN** the entity's `status` is `stopped` on the first sync and `running` on the third, with a condition `status eq running` and a 5 minute timeout
- **THEN** the step SHALL be `succeeded` after the third sync and the run SHALL continue.

#### Scenario: Entity temporarily absent
- **WHEN** the entity is absent from the first sync and present with a matching attribute on a later sync before the timeout
- **THEN** the step SHALL be `succeeded`.

#### Scenario: Timeout with last value
- **WHEN** the timeout elapses while the entity's attribute is `starting` and the condition is `status eq running`
- **THEN** the step SHALL be `failed` with a timeout reason that names the attribute and the value `starting`, and the run SHALL be `failed`.

#### Scenario: Timeout with entity never found
- **WHEN** the timeout elapses and the entity was absent from every sync
- **THEN** the step SHALL be `failed` with a timeout reason stating that the entity was not found.

#### Scenario: Sync fails during the wait
- **WHEN** one sync during the wait fails and a later one succeeds with the condition holding before the timeout
- **THEN** the step SHALL be `succeeded`.

#### Scenario: Cancel during the wait
- **WHEN** a run is cancelled while a `wait_for_entity` step is waiting between syncs
- **THEN** no further sync SHALL be started for the step and the run SHALL be `cancelled`.
