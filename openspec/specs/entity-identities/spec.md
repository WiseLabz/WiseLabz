# Entity identities Specification

## Requirements

### Requirement: Persisted connector-independent identities
The system SHALL assign a persisted identity to every entity in each readable latest connector snapshot, including entities with no cross-connector match. A member SHALL be identified by connector ID, entity kind, and the existing `entityRef` rule (external ID when present, otherwise name).

#### Scenario: Unmatched entity receives identity
- **WHEN** a connector snapshot contains an entity with no strong match
- **THEN** the system SHALL persist one entity identity and one connector membership for it.

#### Scenario: Identity membership is unique
- **WHEN** the same connector, kind, and reference is reconciled more than once
- **THEN** the membership SHALL remain unique and its latest name SHALL be stored.

### Requirement: Strong-match clustering
The system SHALL merge memberships transitively only for same-kind external-ID matches and hostname/alias matches. IP-only matches SHALL NOT merge identities and SHALL remain weak topology relationships.

#### Scenario: Transitive strong matches
- **WHEN** observations are connected through a chain of strong matches
- **THEN** all observations in that connected component SHALL share one identity.

#### Scenario: IP-only match
- **WHEN** two observations share an IP but no external ID or hostname/alias
- **THEN** they SHALL retain separate identities.

### Requirement: Stable identity lifecycle
The system SHALL retain the oldest existing identity ID when memberships merge and SHALL set each losing identity's `merged_into` to the winner and its `merged_at` to the merge time (kept when the redirect is later re-pointed, cleared with `merged_into`). When an observation leaves a cluster, it SHALL receive a new identity. When an identity loses its last member, it SHALL receive `gone_at` and remain stored until snapshot retention expires.

#### Scenario: Merge retains oldest ID
- **WHEN** two existing identities become one strong-match component
- **THEN** the older identity ID SHALL remain active and the other row SHALL redirect to it.

#### Scenario: Last member disappears
- **WHEN** a reconciliation removes an identity's final member
- **THEN** the identity SHALL be marked gone without immediate deletion.

#### Scenario: Retention purges identities
- **WHEN** a gone identity (by `gone_at`) or a merged redirect (by `merged_at`, not `last_seen_at`) is older than `retention_settings.snapshot_days`
- **THEN** the retention job SHALL purge it.

#### Scenario: Recently merged identity survives
- **WHEN** an identity unobserved for longer than `snapshot_days` is merged into another identity inside the retention window
- **THEN** its redirect SHALL survive until `merged_at` passes the cutoff.

### Requirement: Sync-time and startup reconciliation
The system SHALL rebuild identities after a successful connector sync and SHALL backfill identities for existing snapshots when a server instance acquires leadership. An unreadable snapshot SHALL NOT erase the last known-good memberships.

### Requirement: Entity-bound findings and search
Entity-specific compliance findings SHALL persist the entity kind and connector-local ref they describe. Entity search hits SHALL include the persisted identity UUID for their member.

### Requirement: One notification per rule per connector
Entity-specific compliance findings SHALL notify at most once per rule and connector at a given severity. A new or returning entity on a rule that already notified at that severity or higher SHALL NOT notify; a severity escalation SHALL notify exactly once; and a rule whose open findings on a connector have all resolved SHALL notify once when it fires again.

### Requirement: Grant-filtered entity detail
The system SHALL expose `GET /api/entities/{id}` and a web detail page for visible persisted identities. The endpoint SHALL follow flattened merge redirects and return the same 404 response for missing or invisible identities unless the redirect target is itself visible. A caller SHALL be able to view at least one member connector; API-key connector restrictions SHALL be applied. Members, topology links, history, findings, and runbooks SHALL include only data from granted connectors. The displayed kind, name, and gone state SHALL be derived from visible members.

#### Scenario: Viewer has access to one member connector
- **WHEN** an identity has members on two connectors and the caller can view one
- **THEN** the endpoint SHALL return only that member and data derived from granted connectors.

#### Scenario: No visible member or hidden redirect target
- **WHEN** the caller cannot view any member of the identity or its redirect target
- **THEN** the endpoint SHALL return 404 without revealing the identity or redirect target.

#### Scenario: Gone identity
- **WHEN** every visible member is marked gone
- **THEN** the detail page SHALL show that the entity is no longer observed.

### Requirement: Entity attribute history
The system SHALL derive entity attribute history from existing snapshot diffs and SHALL NOT add a separate history table.

#### Scenario: Changed attributes
- **WHEN** successive retained snapshots contain a field change for a visible member
- **THEN** the entity detail response SHALL include the corresponding `EntityChange` with its snapshot time.
