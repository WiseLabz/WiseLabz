# Spec Delta

## MODIFIED Requirements

### Requirement: Step kinds
A runbook step SHALL have one of seven kinds: `lifecycle` (restart, start or stop on a connector, optionally on one entity), `sync_and_wait` (sync a connector and wait for the sync to finish), `wait_until_healthy` (wait until a connector's health check reports online), `manual` (a human confirms the step), `config_push` (write one whitelisted configuration field on a connector, optionally on one entity), `wait_for_entity` (wait until an attribute of one entity satisfies a condition) and `connector_action` (run a named action that a custom connector's recipe declares, on the service or on one entity). Existing steps SHALL be treated as `lifecycle`.

#### Scenario: Existing runbook keeps working
- **WHEN** a runbook created before this change is read
- **THEN** each of its steps SHALL be reported with kind `lifecycle` and its verb, connector and entity unchanged.

#### Scenario: New kinds are stored and returned
- **WHEN** a runbook is saved with one `config_push` step and one `wait_for_entity` step and then read
- **THEN** the steps SHALL be returned with their kinds and with the field key, target value, entity, attribute, operator and expected value as saved.

#### Scenario: Connector-action step is stored and returned
- **WHEN** a runbook is saved with a `connector_action` step and then read
- **THEN** the step SHALL be returned with its kind, connector, action name and entity as saved.

### Requirement: Step constraints
A `lifecycle`, `sync_and_wait`, `wait_until_healthy`, `config_push`, `wait_for_entity` or `connector_action` step SHALL reference an existing connector; a `manual` step SHALL NOT require one. A `sync_and_wait` or `wait_until_healthy` step SHALL have a timeout between 10 seconds and 30 minutes, defaulting to 5 minutes. A `wait_for_entity` step SHALL have a timeout between 1 minute and 30 minutes, defaulting to 5 minutes. A runbook SHALL hold at most 20 steps.

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

#### Scenario: Connector-action step without connector
- **WHEN** a runbook is saved with a `connector_action` step that names no connector
- **THEN** the save SHALL be rejected with a field error naming that step's connector.

### Requirement: Resuming a failed run
A `failed` run SHALL be resumable. Resuming SHALL require a fresh `runbook.run` elevation token for that runbook and an operator grant on every connector referenced by the run's steps; if no connector is referenced, it SHALL require an instance admin or operator access on at least one connector. Resume SHALL execute again from the first step that is not `succeeded`, including a step in state `failed` or `unknown`, and SHALL record the resuming user. When that first step is a `connector_action` step in state `unknown`, the resume request SHALL carry an explicit decision for it, either to send the action again or to mark the step as done; without a decision the resume SHALL be rejected and the run SHALL stay `failed`. A step marked as done SHALL become `succeeded` without any request being sent, and the run SHALL continue with the next step. The decision SHALL be recorded in the audit log with the resuming user. Runs in any other state SHALL NOT be resumable.

#### Scenario: Resume after failure
- **WHEN** an elevated, authorized user resumes a run whose third step failed
- **THEN** the run SHALL become `running`, steps one and two SHALL NOT execute again, and step three SHALL execute again.

#### Scenario: Resume without elevation
- **WHEN** a failed run is resumed without a valid elevation token
- **THEN** the request SHALL be rejected and the run SHALL stay `failed`.

#### Scenario: Resume a finished run
- **WHEN** a resume is requested for a `succeeded`, `cancelled` or `expired` run
- **THEN** the request SHALL be rejected with status 409.

#### Scenario: Unknown action step without a decision
- **WHEN** a run whose `connector_action` step is `unknown` is resumed without a decision for that step
- **THEN** the request SHALL be rejected with status 409 and an error stating that a decision is required, the run SHALL stay `failed`, and nothing SHALL be sent to the service.

#### Scenario: Send again
- **WHEN** the same run is resumed with the decision to send the action again
- **THEN** the action SHALL be sent once, and an audit entry SHALL record that the user chose to send it again.

#### Scenario: Mark as done
- **WHEN** the same run is resumed with the decision to mark the step as done
- **THEN** the step SHALL become `succeeded` without a request to the service, the next step SHALL execute, and an audit entry SHALL record that the user marked it done.

#### Scenario: Unknown lifecycle step is unaffected
- **WHEN** a run whose `lifecycle` step is `unknown` is resumed without any decision
- **THEN** the lifecycle step SHALL execute again as before this change.

## ADDED Requirements

### Requirement: Connector-action step authoring
A `connector_action` step SHALL name a connector, a named action and optionally one entity of that connector. Saving SHALL reject a name not declared for the service when no entity is given, or for some entity kind when one is given. Lifecycle verbs SHALL NOT be accepted as named actions; they remain `lifecycle` steps. A custom connector's lifecycle step SHALL be accepted only when its recipe declares the verb.

#### Scenario: Declared action
- **WHEN** a runbook is saved with a `connector_action` step naming `rescan` on a connector whose recipe declares a service action `rescan`
- **THEN** the save SHALL succeed.

#### Scenario: Undeclared action
- **WHEN** the step names an action the connector's recipe does not declare
- **THEN** the save SHALL be rejected with a field error naming that step's action.

#### Scenario: Lifecycle verb as an action
- **WHEN** a `connector_action` step names `restart`
- **THEN** the save SHALL be rejected with a field error naming that step's action.

#### Scenario: Lifecycle step on a recipe without the verb
- **WHEN** a `lifecycle` step with verb `stop` targets a custom connector whose recipe declares no `stop`
- **THEN** the save SHALL be rejected with a field error naming that step's verb.

### Requirement: Connector-action step execution
A `connector_action` step SHALL send the definition declared at run start to the frozen connector and entity, using direct named-action result rules, failure alert and audit detail plus run and step IDs. A changed or removed definition SHALL fail without sending. The single-step endpoint SHALL NOT execute a `connector_action` step.

#### Scenario: Step succeeds
- **WHEN** a run reaches a `connector_action` step and the service answers 204
- **THEN** the step SHALL become `succeeded`, the run SHALL continue, and the audit entry for the action SHALL carry the run and step identifiers.

#### Scenario: Action changed after the run started
- **WHEN** the recipe's definition of the action is changed while the run waits on an earlier manual step
- **THEN** the `connector_action` step SHALL fail with a reason stating that the action changed, and nothing SHALL be sent.

### Requirement: Connector-action step outcome
If the request was sent but no response status received, the step SHALL become `unknown` and the run `failed`. A non-2xx response or error before sending SHALL make the step `failed`.

#### Scenario: Connection lost after sending
- **WHEN** the request is sent and the connection is lost before a status arrives
- **THEN** the step SHALL become `unknown` and the run `failed`.

#### Scenario: Service refuses
- **WHEN** the service answers 500
- **THEN** the step SHALL become `failed` with the status in its reason, and the run SHALL become `failed`.

### Requirement: Connector-action step preview
The run preview SHALL show each `connector_action` step's method, URL, static headers and body, plus its label, description and downtime estimate, without sending to the service.

#### Scenario: Preview shows the request
- **WHEN** the dry-run preview of a runbook with a `connector_action` step is requested
- **THEN** the preview SHALL show that step's method, URL and body and SHALL send nothing to the service.
