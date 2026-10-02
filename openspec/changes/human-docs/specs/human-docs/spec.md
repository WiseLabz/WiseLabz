# Spec Delta

## Purpose

Allow people to author and organize durable documentation with scope-aware access and recoverable deletion alongside generated service inventory.

## ADDED Requirements

### Requirement: Human document creation
The API SHALL create human-origin docs with a nonempty title, optional content, service and parent, record the creator, and write revision one with trigger create. No service means lab kind; a service means service kind. Sync SHALL leave human docs untouched.

#### Scenario: Create a lab note
- **WHEN** an instance administrator creates a lab note
- **THEN** its origin is human and revision one records the creator and content

### Requirement: Scope-aware access
Human lab docs SHALL be viewable by authenticated users and mutable only by instance admins. Generated lab docs SHALL be admin-only. Service docs SHALL require connector viewer for reads and operator for mutations. Hidden docs SHALL return 404 on reads and SHALL never leak titles in trees, search or retrieval.

#### Scenario: Ordinary user reads lab docs
- **WHEN** an ordinary authenticated user lists docs
- **THEN** human lab notes are visible and generated lab inventory is absent

### Requirement: Bounded hierarchy
Rename and re-parent SHALL preserve scope, reject cycles and limit hierarchy depth to five including the moved subtree. Tree responses SHALL nest children and include a Lab branch.

#### Scenario: Invalid move
- **WHEN** a doc is moved below its descendant, into another scope or beyond depth five
- **THEN** the operation fails without changing the hierarchy

### Requirement: Recoverable subtree deletion
Deletion SHALL atomically soft-delete the active subtree with one shared timestamp. Administrator trash SHALL list deleted docs. Restore SHALL recover the selected doc and its descendants from that deletion batch, preserving older separately deleted descendants and leaving no active doc under a deleted parent.

#### Scenario: Restore a deletion batch
- **WHEN** an administrator restores a deleted subtree root
- **THEN** docs deleted with that root return and earlier independently deleted descendants remain in trash

### Requirement: Deleted docs are invisible
Every ordinary doc read, list, version, share, quality, export, chat, MCP, full-text search and embedding retrieval SHALL exclude deleted docs. Backups SHALL carry parent, creator and deletion metadata in the existing v1 JSON format.

#### Scenario: Search after deletion
- **WHEN** a doc is soft-deleted
- **THEN** its title, content and embedded sections disappear from normal discovery

### Requirement: Deleted document retention
The scheduled retention job SHALL permanently purge docs older than retention.deleted_docs_days, default thirty days, cascading versions, locks and embeddings.

#### Scenario: Purge old trash
- **WHEN** cleanup runs with thirty-day retention
- **THEN** older trash and its dependent records are removed while recent trash and live docs survive

### Requirement: Human document user interface
The web app SHALL provide a New doc dialog with permitted scopes and parent selection, a palette action, nested doc navigation with drag re-parenting, editor deletion, and administrator trash restoration.

#### Scenario: Author and recover a note
- **WHEN** an authorized user creates, nests and deletes a note and an administrator restores it
- **THEN** the tree reflects each operation and the restored note can be opened
