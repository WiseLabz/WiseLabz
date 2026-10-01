# Spec Delta

## Purpose

Gives operators a quick starting point for a runbook from an alert without requiring an AI provider or persisting unreviewed content.

## ADDED Requirements

### Requirement: Draft runbook from alert
`POST /alerts/{id}/draft-runbook` SHALL return a deterministic draft runbook (title, body, suggested target) built from the alert, its linked change and any existing runbook for the same target. It MUST NOT persist anything and MUST require operator access to the alert's connector.

#### Scenario: Draft returned
- **WHEN** an operator requests a draft for an existing alert
- **THEN** 200 with a draft body referencing the alert and change summary, and no runbook is created

#### Scenario: Unknown alert
- **WHEN** the alert id does not exist
- **THEN** 404

#### Scenario: Viewer denied
- **WHEN** a viewer-only user requests a draft
- **THEN** 403

#### Scenario: Works without AI
- **WHEN** AI is disabled
- **THEN** the draft is still returned

### Requirement: Safe diff quoting
The draft SHALL truncate a quoted change diff on UTF-8 boundaries and fence it so the diff's own content cannot close the Markdown block.

#### Scenario: Diff contains a code fence
- **WHEN** the change diff contains triple backticks
- **THEN** the draft wraps it in a longer fence and the diff stays inside the block
