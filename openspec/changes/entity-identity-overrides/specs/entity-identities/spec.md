## MODIFIED Requirements

### Requirement: Strong-match clustering
The system SHALL merge memberships transitively only for same-kind external-ID matches and hostname/alias matches. IP-only matches SHALL NOT merge identities and SHALL remain weak topology relationships.

Manual identity overrides SHALL take precedence over automatic clustering in this order: a detached member SHALL NOT be joined to any other member by an automatic match; automatic matches SHALL then cluster the remaining members; each merge override whose two members are both present SHALL then place those members in one identity, including when either member is detached. With no overrides the result SHALL be identical to automatic clustering alone.

#### Scenario: Transitive strong matches
- **WHEN** observations are connected through a chain of strong matches
- **THEN** all observations in that connected component SHALL share one identity.

#### Scenario: IP-only match
- **WHEN** two observations share an IP but no external ID or hostname/alias
- **THEN** they SHALL retain separate identities.

#### Scenario: Detached member leaves a strong-match cluster
- **WHEN** a member that shares a hostname with two other members has a detach override
- **THEN** the detached member SHALL have its own identity and the other two SHALL still share one.

#### Scenario: Merge override joins unmatched members
- **WHEN** two members with no strong match have a merge override
- **THEN** they SHALL share one identity.

#### Scenario: Detach and merge move a member
- **WHEN** a member has a detach override and a merge override with a member of another identity
- **THEN** it SHALL belong to that other identity and not to the identity its automatic matches point to.

## ADDED Requirements

### Requirement: Manual identity overrides
The system SHALL persist manual identity overrides separately from identities and memberships. An override SHALL be either a detach of one member or a merge of two members, SHALL identify each member by connector ID, entity kind and connector-local ref, and SHALL record who created it, when, and an optional note. Creating or removing an override SHALL NOT change any connector-local ref.

Creation SHALL be rejected when a referenced member has never been recorded, when a merge names the same member twice, when a merge names members of different kinds, or when an equivalent override already exists (a merge of A with B is equivalent to a merge of B with A).

#### Scenario: Unknown member
- **WHEN** an override is created for a connector, kind and ref with no recorded membership
- **THEN** creation SHALL be rejected and nothing SHALL be stored.

#### Scenario: Merge across kinds
- **WHEN** a merge override names two members of different kinds
- **THEN** creation SHALL be rejected.

#### Scenario: Duplicate merge in reverse order
- **WHEN** a merge of A with B exists and a merge of B with A is created
- **THEN** creation SHALL be rejected as a duplicate.

### Requirement: Override effect on identity lifecycle
An identity change caused by an override SHALL follow the existing lifecycle: when a merge override joins two existing identities, the oldest identity ID SHALL remain active and the other SHALL redirect to it with `merged_at` set; when a detach override separates a member, exactly one resulting cluster SHALL keep the previous identity ID and the others SHALL receive new identities. Removing an override SHALL restore the automatic result at the next reconciliation.

#### Scenario: Manual merge leaves a redirect
- **WHEN** a merge override joins members of two existing identities
- **THEN** the older identity ID SHALL stay active and the other SHALL resolve to it.

#### Scenario: Removing a detach restores the automatic cluster
- **WHEN** a detach override on a member with a hostname match is removed
- **THEN** after reconciliation the member SHALL again share an identity with its hostname matches.

### Requirement: Override lifecycle
Deleting a connector SHALL delete every override that references one of its members. While a referenced member is not currently observed, the override SHALL be retained, SHALL be reported as dormant, and SHALL apply again without further action when the member returns. The retention job SHALL delete an override once a member it references no longer has any stored membership row. Backups SHALL include overrides, and a restore SHALL reinstate them.

#### Scenario: Connector deleted
- **WHEN** a connector referenced by an override is deleted
- **THEN** the override SHALL no longer exist.

#### Scenario: Member disappears and returns
- **WHEN** a detached member is missing from a snapshot and present again in a later one
- **THEN** the override SHALL be dormant while it is missing and the member SHALL be detached again once it returns.

#### Scenario: Member history expires
- **WHEN** retention purges the last membership row for a member referenced by an override
- **THEN** the override SHALL be deleted in the same retention run.

#### Scenario: Backup round trip
- **WHEN** a backup taken with overrides present is restored
- **THEN** the same overrides SHALL exist after the restore.

### Requirement: Override administration API
The system SHALL expose `GET /api/entity-overrides`, `POST /api/entity-overrides` and `DELETE /api/entity-overrides/{id}` to instance administrators only; any other caller SHALL receive 403. The list SHALL return every override with its action, note, creator, creation time, state (`active` when every referenced member is currently observed, otherwise `dormant`), and for each member its connector ID and name, kind, ref, name and current identity ID. A successful create or delete SHALL reconcile identities before responding and SHALL return the identity ID that each affected member now belongs to. A rejected creation SHALL return 400, or 409 for a duplicate. Each successful create and delete SHALL write an audit record (`entity.override.create`, `entity.override.delete`) naming the override and its members.

#### Scenario: Non-admin caller
- **WHEN** a user who is not an instance administrator calls any override endpoint
- **THEN** the response SHALL be 403 and no override SHALL be read or changed.

#### Scenario: Create returns the new identities
- **WHEN** an administrator who can view the member's connector creates a detach override for a member of a two-member identity
- **THEN** the response SHALL contain the override and the member's new identity ID, and a following `GET /api/entities/{id}` for that ID SHALL return the member.

#### Scenario: Administrator without a grant on the member's connector
- **WHEN** an administrator with no viewer grant on the member's connector creates a detach override
- **THEN** the response SHALL still contain the member's new identity ID, and `GET /api/entities/{id}` for that ID SHALL return 404, because instance administrators have no implicit entity access.

#### Scenario: Duplicate
- **WHEN** an administrator creates an override equivalent to an existing one
- **THEN** the response SHALL be 409.

#### Scenario: Audited
- **WHEN** an override is created and then deleted
- **THEN** the audit log SHALL contain one `entity.override.create` and one `entity.override.delete` record for it.

### Requirement: Override admin workflow
For instance administrators the entity detail page SHALL offer a detach action on each member and a merge action that selects another entity's member of the same kind, and the web UI SHALL provide a list of all overrides with their state and a remove action. After a create or remove the UI SHALL show the identity returned by the API for the affected member; when that identity is not visible to the administrator, the UI SHALL stay on the current page and show a confirmation instead of navigating to a 404. Callers who are not instance administrators SHALL NOT see these actions or the list. All new strings SHALL be translatable and present in the English and pt-BR catalogs.

#### Scenario: Admin detaches a member
- **WHEN** an administrator uses the detach action on a member and confirms
- **THEN** the UI SHALL create the override and open the detail page of the member's new identity.

#### Scenario: Non-admin view
- **WHEN** a non-admin user opens an entity detail page
- **THEN** no detach, merge or override-list control SHALL be rendered.

#### Scenario: Removing an override
- **WHEN** an administrator removes an override from the list
- **THEN** the override SHALL disappear from the list after the API confirms the deletion.
