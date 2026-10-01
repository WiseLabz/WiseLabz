# Spec Delta

## Purpose

Allows AI clients to suggest documentation changes that are never applied directly and are always reviewed and approved by a human doc operator.

## ADDED Requirements

### Requirement: Propose a doc edit via MCP
The MCP server SHALL provide a `propose_doc_edit` tool creating a pending proposal containing the target doc, base version and proposed content. It MUST NOT modify the doc.

#### Scenario: Proposal created
- **WHEN** a full-scope key holder with operator access to the doc calls the tool
- **THEN** a pending proposal is stored, an audit entry is written, and the doc is unchanged

#### Scenario: Read-only key rejected
- **WHEN** a read-only API key calls the tool
- **THEN** the call is rejected and no proposal is created

#### Scenario: Insufficient access
- **WHEN** the caller lacks operator access to the doc (or is not instance admin for a lab-wide doc)
- **THEN** the call is rejected

### Requirement: Review proposals
Doc operators SHALL be able to list, approve and reject proposals via REST and a review UI.

#### Scenario: Approve applies edit
- **WHEN** an operator approves a proposal whose base version equals the doc's current version
- **THEN** the content is saved as a new doc version and the proposal becomes approved

#### Scenario: Stale base version
- **WHEN** the doc changed since the proposal's base version
- **THEN** approval fails with a conflict and the proposal stays pending

#### Scenario: Reject
- **WHEN** an operator rejects a proposal
- **THEN** it becomes rejected and the doc is unchanged
