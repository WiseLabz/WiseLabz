# Spec Delta

## MODIFIED Requirements

### Requirement: Starting a run
Starting a run SHALL require a valid elevation token for the action `runbook.run` scoped to that runbook and an operator grant on every connector referenced by the runbook's steps. When the runbook references no connectors, starting SHALL instead require an instance admin or an operator grant on at least one connector. For runbooks without second-operator approval, a successful start SHALL create a run in state `running`, record the starting user, respond without waiting for the run to finish, and return the run's identifier. A runbook with no steps SHALL NOT be startable.

#### Scenario: Successful start
- **WHEN** an elevated user holding operator on every referenced connector starts a run without the approval opt-in
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

#### Scenario: Approval token cannot start a run
- **WHEN** instance step-up is enabled and a caller starts or resumes a run using a `runbook.approve` elevation token
- **THEN** the request SHALL be refused and no execution SHALL start.

### Requirement: Frozen steps
A run SHALL execute and report the steps as they were when the run was requested or started. Editing or deleting the runbook or its steps afterwards SHALL NOT change a run's steps, its progress or its history.

#### Scenario: Runbook edited during a run
- **WHEN** a runbook's steps are replaced while one of its runs is waiting on a manual step
- **THEN** resuming that run SHALL execute the steps recorded at request or start, not the edited ones.

#### Scenario: Runbook deleted
- **WHEN** a runbook with finished runs is deleted
- **THEN** its runs SHALL remain readable with the runbook title and steps recorded at request or start.

#### Scenario: Runbook edited before approval
- **WHEN** an authored runbook is edited after requesting approval
- **THEN** the approver SHALL see and approve the original frozen steps and the executor SHALL use those same steps.

### Requirement: Single-step execution unchanged
On runbooks without the approval opt-in, executing one lifecycle step directly SHALL keep its current behaviour: per-step dry-run preview, per-step elevation, operator grant on the step's connector, and an audit record. A single-step execution SHALL NOT create a run and SHALL NOT appear in run history.

#### Scenario: Single step during an active run
- **WHEN** a lifecycle step is executed directly while the runbook has no active run
- **THEN** the step SHALL execute as before and the runbook's run history SHALL be unchanged.

#### Scenario: Single step on approval-required runbook
- **WHEN** the single-step execute endpoint is called for an opted-in runbook
- **THEN** the response SHALL be 409 `approval_required` and nothing SHALL execute.

## ADDED Requirements

### Requirement: Second-operator approval opt-in
Runbook create, update, read and backup restore SHALL preserve a `requiresApproval` boolean defaulting to false. Preview SHALL report `requiresApproval` and `approverAvailable`. Ordinary runbooks SHALL retain their start response shape and execution behavior.

#### Scenario: Restore an opted-in runbook
- **WHEN** an opted-in runbook is backed up and restored from JSON or ZIP
- **THEN** it SHALL still require a second operator's approval.

#### Scenario: No approver in preview
- **WHEN** an opted-in runbook has no eligible other operator
- **THEN** its preview and start dialog SHALL identify the unavailable approver and disable the request button.

### Requirement: Requesting approval
Starting an opted-in runbook SHALL require current operator authorization, an existing nonempty runbook, another eligible approver, then `runbook.run` elevation on the runbook. It SHALL create the frozen run in `awaiting_approval`, return 202 and execute no steps. No eligible approver SHALL return 409 `no_eligible_approver` before elevation is consumed.

#### Scenario: Approval requested
- **WHEN** an elevated authorized initiator requests an opted-in runbook with another eligible operator
- **THEN** one awaiting run with frozen pending steps SHALL be created and no step SHALL execute.

#### Scenario: No eligible approver
- **WHEN** a request has no other eligible operator
- **THEN** 409 `no_eligible_approver` SHALL be returned with no run created and the elevation token SHALL remain unconsumed.

#### Scenario: One active request
- **WHEN** a runbook already has an open run, including an unanswered approval request
- **THEN** a second start SHALL return 409 conflict.

### Requirement: Approver authorization
Approval and rejection SHALL require an enabled user other than the initiator with operator access to every frozen connector, applying API-key scope and refusing read-only keys. For connectorless runs, instance admin or operator on any connector SHALL be required. When instance step-up is enabled, approval SHALL require the approver's own `runbook.approve` elevation on the run ID. With step-up disabled, approval SHALL follow the existing elevation bypass; rejection SHALL require no elevation.

#### Scenario: Forbidden approver
- **WHEN** the initiator, a disabled user, a partial-grant operator, or a read-only API key attempts approval or rejection
- **THEN** the response SHALL be 403 and the request SHALL remain awaiting approval.

#### Scenario: Approval elevation boundary
- **WHEN** instance step-up is enabled and an eligible approver presents no elevation, a `runbook.run` token, another user's token, or a token for another run ID
- **THEN** approval SHALL be refused and no step SHALL execute.

#### Scenario: Reject without elevation
- **WHEN** an eligible other operator rejects without an elevation token
- **THEN** the run SHALL become rejected, record the rejecting user and finish time, and skip its pending steps.

### Requirement: Atomic approval transitions
Approve, reject, withdraw and approval-expire SHALL transition only from `awaiting_approval` using a conditional state change. A competing or repeated transition SHALL return 409 conflict. Approval SHALL record approver and time, publish the run and start execution as the initiator with existing per-step grant rechecks.

#### Scenario: Repeated or competing approval
- **WHEN** two approval transitions target the same awaiting run
- **THEN** at most one SHALL succeed and the other SHALL conflict without starting another execution.

#### Scenario: Initiator grant revoked
- **WHEN** approval succeeds after the initiator lost access to a frozen connector
- **THEN** the affected step SHALL fail the initiator's grant recheck without mutating that connector.

#### Scenario: Withdrawal
- **WHEN** the initiator cancels an awaiting request with existing cancel authorization
- **THEN** the run SHALL become cancelled, pending steps SHALL be skipped and another request SHALL be allowed.

### Requirement: Awaiting execution boundary
An awaiting run SHALL never execute a step through start, resume, manual confirm, startup recovery or single-step execution. Resume and confirm on an awaiting run SHALL return 409 conflict. Startup recovery and existing open-run expiry SHALL leave awaiting requests unchanged.

#### Scenario: Resume or confirm awaiting request
- **WHEN** an operator resumes or confirms a step in an awaiting run
- **THEN** 409 conflict SHALL be returned and all steps SHALL remain pending.

#### Scenario: Restart while awaiting
- **WHEN** the server restarts with an awaiting request
- **THEN** it SHALL remain awaiting and no execution SHALL start automatically.

### Requirement: Approval request expiry
A separate approval retention setting SHALL default to 24 hours. A per-minute leader job SHALL expire only awaiting requests whose last update is older than this setting, mark reason `approval_expired`, skip pending steps and publish the run update. Rejected runs SHALL be terminal and subject to finished-run pruning.

#### Scenario: Stale request expires
- **WHEN** the minute job runs with a stale awaiting request and a recent awaiting request
- **THEN** only the stale request SHALL become expired with `approval_expired`, a finish time and skipped steps.

#### Scenario: Rejected history pruned
- **WHEN** a rejected run is older than the history retention cutoff
- **THEN** it and its frozen steps SHALL be pruned.

### Requirement: Approval request notifications and audit
The system SHALL send `runbook.run_approval_requested` only to eligible approvers through notification rules, sending external channels once per request. Request, approval and rejection SHALL audit the acting user under `runbook.run.approval_requested`, `runbook.run.approved` and `runbook.run.rejected`. These actions SHALL NOT enter the journal allowlist.

#### Scenario: Eligible-only recipients
- **WHEN** a request has two eligible operators and other ineligible users
- **THEN** only the eligible operators SHALL receive inbox notifications and matching external channels SHALL receive one request notification.

#### Scenario: Approval actions audited
- **WHEN** a request is created, approved or rejected
- **THEN** the corresponding audit entry SHALL identify the run and the acting user.

### Requirement: Approval history and controls
Run detail SHALL expose `canApprove` and `approvalExpiresAt` and show frozen pending steps, requester, request time and expiry for awaiting requests. Eligible operators SHALL see approve/reject controls; the initiator SHALL see cancel request. Finished runs SHALL show rejection, approval expiry and approved-by identity; history SHALL label both new states in English and pt-BR.

#### Scenario: Awaiting detail
- **WHEN** an eligible other operator opens an awaiting run
- **THEN** approve with `runbook.approve` elevation and reject SHALL be available, and resume and confirm SHALL be absent.

#### Scenario: Initiator detail
- **WHEN** the initiator opens their awaiting run
- **THEN** cancel request SHALL be available and approve/reject SHALL be absent.

### Requirement: Instance step-up setting compatibility
Approval SHALL use the same instance step-up setting as run start. Disabling step-up SHALL bypass elevation only; enabled-user, different-operator, every-frozen-connector, API-key and conditional-state guards SHALL remain enforced. With step-up enabled, the two elevation actions SHALL remain distinct.

#### Scenario: Approval with step-up disabled
- **WHEN** instance step-up is disabled and an eligible different operator approves without an elevation token
- **THEN** the awaiting run SHALL transition once and execute as the initiator, and a repeated approval SHALL conflict.

#### Scenario: Ineligible operator with step-up disabled
- **WHEN** instance step-up is disabled and the initiator, a disabled or partial-grant user, or a restricted or read-only key attempts approval or rejection
- **THEN** the request SHALL be refused with 403 and no execution SHALL start.
