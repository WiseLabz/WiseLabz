# Spec Delta

## MODIFIED Requirements

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

### Requirement: Run audit
The system SHALL record an audit entry with the acting user for each of: run started, manual step confirmed, run resumed, run cancelled. Lifecycle steps executed by a run SHALL keep their existing audit action with the run and step identifiers added. A `config_push` step that writes SHALL record the same audit action as a direct config push, with the field key, the entity, and the run and step identifiers.

#### Scenario: Run started
- **WHEN** a run is started
- **THEN** an audit entry for the start SHALL exist with the runbook and run identifiers and the starting user.

#### Scenario: Config push in a run is audited
- **WHEN** a `config_push` step of a run writes its field successfully
- **THEN** a config-push audit entry SHALL exist for that connector with the field key, the acting user and the run and step identifiers.

## ADDED Requirements

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
A `config_push` step SHALL write the target value that was recorded when the run started, to the field and entity recorded when the run started. Before writing, the system SHALL verify the field is still writable on the connector and SHALL fail the step without writing if it is not. When the connector reports that the field already holds the target value, the step SHALL succeed without writing. After writing, the system SHALL verify the write took effect. When it did not and the value held before the write is known, the system SHALL write that previous value back, raise the same alert a direct config push raises for a mismatch, and fail the step. When it did not and the previous value is not known, the system SHALL raise the alert and fail the step without writing again.

#### Scenario: Push succeeds
- **WHEN** a `config_push` step runs and the write is verified
- **THEN** the step SHALL be `succeeded` and the run SHALL continue with the next step.

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
- **THEN** resume SHALL require an explicit `resend` or `mark_done` decision tied to the step ID and current run `updatedAt`; missing decisions SHALL be rejected and stale or concurrent decisions SHALL change nothing.
- **AND** `resend` SHALL execute the frozen step again, succeeding without a write if the field already holds the target value; `mark_done` SHALL mark it succeeded after manual verification without writing and continue to the next step.

#### Scenario: Successful write followed by failed verification fetch
- **WHEN** a `config_push` write returns success but the following snapshot fetch fails
- **THEN** exactly one audit entry marked `verification: unverified` and one critical alert SHALL be attempted using separate detached contexts bounded to 15 seconds each, preserving field/entity and run/step identifiers without configuration values, credentials or raw connector errors.
- **AND** no verification retry, fallback read or rollback SHALL be attempted; the step SHALL be `unknown`, the run SHALL be `failed`, and later steps SHALL NOT execute until an operator resumes with an explicit decision.

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
