# Spec Delta

## Purpose

Give lab members a chronological history that combines automated activity with durable manual context while respecting connector permissions.

## ADDED Requirements

### Requirement: Merged chronological timeline
The system SHALL expose changes, alerts, document edits, manual entries, significant sync runs and permitted lab audit actions in descending timestamp, kind and id order using a paginated envelope and opaque nextCursor. Timestamp precision SHALL be normalized before ordering. Date range, connector, source and all-sync-run filters SHALL apply before pagination.

#### Scenario: Mixed precision and paging
- **WHEN** a member pages through whole-second and fractional-second events
- **THEN** events appear chronologically without missing or repeated rows

#### Scenario: Quiet successful runs
- **WHEN** a member opens the default timeline
- **THEN** syncs without failures, changes or alerts are omitted until all-sync-runs is enabled

### Requirement: Source visibility
The system SHALL limit connector-linked sources to viewer grants and API-key connector restrictions. Lab-wide manual entries SHALL be readable by signed-in users. Audit rows SHALL be instance-admin only and limited to lab connector, change, alert, doc and runbook actions; security events SHALL remain on Audit. Document edits SHALL omit soft-deleted documents.

#### Scenario: Restricted member
- **WHEN** a viewer with one connector grant requests a timeline
- **THEN** other connectors and all audit rows are absent, and lab-wide notes remain readable

#### Scenario: Admin audit
- **WHEN** an instance admin requests audit activity
- **THEN** allowed lab actions are returned and authentication/security events are absent

### Requirement: Manual entry lifecycle
The system SHALL allow connector operators to create notes and instance admins to create lab-wide notes. Entries SHALL support body, occurred_at defaulting to now, optional connector/doc and optional entity kind, name and external ref text. Authors and admins SHALL edit or delete entries they can view. Scope changes SHALL require destination write access and document links SHALL respect scope and read permissions.

#### Scenario: Backdated entity note
- **WHEN** an operator creates a backdated entry linked to an entity
- **THEN** it sorts at its occurred time among real changes and sync runs and retains entity context

#### Scenario: Write authorization
- **WHEN** a viewer creates an entry or a non-author non-admin edits one
- **THEN** the request is rejected without changing the note

### Requirement: Durable manual context
The system SHALL include journal entries in backup export/import and SHALL exempt them from operational retention. Deleting a linked connector or document SHALL retain notes with the deleted link cleared. Mutations SHALL record an audit action.

#### Scenario: Backup recovery and deletion
- **WHEN** notes are exported and restored or their linked resource is deleted
- **THEN** body, occurrence time, author and entity text survive

### Requirement: Journal interface
The system SHALL offer a Journal route, navigation and command palette entry with infinite paging, URL filters, links to source pages and a dialog supporting Markdown preview, datetime, scope, entity and document selection. English and Brazilian Portuguese translations SHALL cover new strings.

#### Scenario: Entry management
- **WHEN** an authorized author saves, edits or deletes a note
- **THEN** the interface refreshes the timeline and displays the resulting chronological state

### Requirement: Accurate change and alert filtering
The Changes and Alerts lists SHALL return the connector serviceName and Changes/Alerts severity filtering SHALL happen before server pagination. Changing a severity filter SHALL reset the displayed page.

#### Scenario: Critical page
- **WHEN** a member selects critical severity
- **THEN** the first page contains matching critical rows and totals reflect the server filter

### Requirement: Window narration
The system SHALL let a signed-in user, on demand, request an AI narration of the Journal window selected by the same connector, date range, source and all-sync-run filters as the timeline. Only events returned by the caller's own visibility-scoped timeline query SHALL enter the prompt, at most the 100 newest events and 48 KB of prompt with the oldest dropped first, and event content SHALL be treated as untrusted data. The response SHALL cite events by number, list the numbered source events, and state when events were left out. An empty window SHALL return an empty narration without calling the provider, a read-only API key SHALL be refused with 403, a disabled AI module SHALL return 409 ai_disabled, and a provider failure SHALL return 502 without the provider's error text. Narrations SHALL NOT be cached, persisted or audited, and SHALL be shown as plain text. Disabled or failing AI SHALL leave the ordinary Journal unaffected.

#### Scenario: Member with mixed grants
- **WHEN** a member with a grant on one of two connectors requests a narration of the whole window
- **THEN** only events from the granted connector and lab-wide entries enter the prompt and the sources

#### Scenario: Large window
- **WHEN** the window holds more than 100 visible events
- **THEN** only the newest events are narrated and the response and interface say that older events were left out

#### Scenario: Provider failure
- **WHEN** the AI provider fails or AI is disabled
- **THEN** the interface shows a message for that case and the timeline stays usable

#### Scenario: Disabled AI
- **WHEN** an administrator has not enabled AI and a user requests a narration
- **THEN** the response is 409 ai_disabled and no provider is called

#### Scenario: Empty window
- **WHEN** no visible event falls in the selected window
- **THEN** the response has an empty narration and no sources, and no provider is called

#### Scenario: Read-only API key
- **WHEN** a read-only API key requests a narration
- **THEN** the request is refused with 403

