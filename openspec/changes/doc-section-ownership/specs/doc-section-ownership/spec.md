# Spec Delta

## Purpose

Lets connector syncs refresh generated documentation without ever discarding text that a human wrote or edited. Conflicts between upstream changes and human edits go to a reviewable Change instead of being silently overwritten.

## ADDED Requirements

### Requirement: Generated content is delimited by ownership markers
Content the system generates for a doc SHALL be wrapped in generated blocks of the form
`<!-- wl:gen key="<key>" h="<hash>" -->` + newline + body + newline + `<!-- /wl:gen -->`.
- `<key>` SHALL be stable for the same upstream section across syncs.
- `<hash>` SHALL be a hash of the body as last generated.
- Any text outside generated blocks SHALL be treated as human-owned.

#### Scenario: Newly generated doc carries markers
- **WHEN** a doc is generated from a connector snapshot or a template
- **THEN** each generated section of its content is enclosed in a `wl:gen` block with a key and a hash of its body

#### Scenario: Markers are invisible when rendered
- **WHEN** a doc containing `wl:gen` markers is rendered in the web UI or a share link
- **THEN** the markers are not visible and only the block bodies are shown

### Requirement: Generated content excludes volatile fetch timestamps
Generated doc content SHALL NOT contain the snapshot fetch time. The system SHALL record the time of the last sync merge for each generated doc and expose it as `lastSyncedAt` on the doc.

#### Scenario: Sync with unchanged upstream data writes nothing
- **WHEN** a connector syncs and its snapshot data is identical to the previous sync apart from the fetch time
- **THEN** no new doc version is created, the doc's `updatedAt` is unchanged, and its `lastSyncedAt` is updated

### Requirement: Sync merges generated blocks without overwriting human edits
On sync, the system SHALL re-render each generated doc of the connector (through the doc's template when it has one) and merge block by block:
- A block whose body still matches its hash SHALL be replaced with the new body and hash.
- A block whose body no longer matches its hash (human-edited) SHALL be left unchanged.
- Human-owned text SHALL be left unchanged.
- A new upstream section that the doc has never contained SHALL be inserted after the block that precedes it in the new render, or appended at the end if there is none.
- An upstream section that has disappeared SHALL be removed if its block was unedited. If the block was human-edited, its markers SHALL be removed and its body kept as human-owned text.
- A generated block that a human deleted from the doc SHALL NOT be re-inserted.

#### Scenario: Unedited block is refreshed
- **WHEN** a doc block is unedited and its upstream section changed
- **THEN** after sync the block contains the new generated body and a new version is recorded with trigger `sync`

#### Scenario: Human text outside blocks survives sync
- **WHEN** a user adds a paragraph between two generated blocks and the connector then syncs with changed data
- **THEN** the paragraph is still present, unchanged, after the sync

#### Scenario: Deleted block stays deleted
- **WHEN** a user deletes a generated block (including its markers) and the connector syncs again
- **THEN** the block is not re-added

#### Scenario: Human-origin docs are never touched
- **WHEN** a doc has `origin` = `human` and its connector syncs
- **THEN** the doc's content and versions are unchanged

#### Scenario: Locked doc is deferred
- **WHEN** a user holds a live edit lock on a generated doc during sync
- **THEN** that doc is not modified by this sync and is merged on a later sync

#### Scenario: Concurrent human save wins
- **WHEN** a human saves the doc between the sync reading it and writing it
- **THEN** the sync write is discarded and the human's save is preserved

### Requirement: Conflicting edits raise a reviewable Change
When a human-edited block's upstream section also changed, the system SHALL leave the block unchanged and raise a Change. The Change SHALL have:
- `changeType` = `doc_conflict`, severity `info`, on the doc's connector;
- `affectedDocIds` containing the doc;
- a diff of format `doc` whose `baseText` is the human-edited body and whose `headText` is the newly generated body.

There SHALL be at most one open conflict Change per doc and block key. A newer upstream body SHALL replace the older open Change instead of adding another. A proposal identical to the latest Change for that block SHALL NOT be raised again, even if that Change was dismissed.

#### Scenario: Both sides changed
- **WHEN** a user edited a generated block and the upstream data for that block then changes
- **THEN** the block is unchanged after sync and one `doc_conflict` Change exists for that doc and block

#### Scenario: Repeated syncs do not duplicate the conflict
- **WHEN** the connector syncs again with the same upstream data while the conflict Change is still open
- **THEN** no additional Change is created

#### Scenario: Edited block with unchanged upstream raises nothing
- **WHEN** a user edited a generated block and the upstream data for it is unchanged
- **THEN** no Change is raised and the block is unchanged

### Requirement: Doc Changes can be resolved by accept or keep
`POST /api/changes/{id}/resolve-doc` with body `{"action": "accept" | "keep"}` SHALL resolve open `doc_conflict` and `doc_adopt` Changes.
- It SHALL require an operator grant on the Change's connector (403 otherwise), and SHALL return 404 to callers without a viewer grant.
- It SHALL return 409 when the Change is not an open doc Change, or when the targeted block no longer exists.
- `accept` on a conflict SHALL replace the block body with the generated body and a fresh hash.
- `keep` on a conflict SHALL remove the block's markers so its text becomes human-owned permanently.
- `accept` on an adopt SHALL replace the doc content with the generated render and set the doc's origin to `generated`.
- `keep` on an adopt SHALL leave the doc unchanged.
- Content changes SHALL be saved as a new doc version with trigger `sync-resolve`, authored by the caller. The Change SHALL then be marked `acknowledged`, and an audit record SHALL be written.

#### Scenario: Accept generated body
- **WHEN** an operator resolves a `doc_conflict` Change with `accept`
- **THEN** the doc's block contains the generated body, a `sync-resolve` version exists, and the Change is acknowledged

#### Scenario: Keep human edit
- **WHEN** an operator resolves a `doc_conflict` Change with `keep`
- **THEN** the block's text is unchanged but no longer inside markers, and later syncs never modify or flag it

#### Scenario: Viewer cannot resolve
- **WHEN** a user with only a viewer grant calls resolve-doc
- **THEN** the response is 403 and the doc is unchanged

### Requirement: Existing docs are upgraded by provenance
On the first sync after upgrade, each generated doc that has never been rendered with markers SHALL be classified by its latest version:
- If that version was authored by the system (no user author), the doc SHALL be re-rendered with markers.
- If it was authored by a user, the doc SHALL be set to `origin` = `human`, its content left unchanged, and one `doc_adopt` Change raised. That Change's `baseText` is the current content and its `headText` is the generated render.

#### Scenario: System-authored doc gains markers
- **WHEN** a pre-upgrade doc whose latest version has trigger `sync` and no author is synced
- **THEN** its content is replaced by a marked render

#### Scenario: Human-edited legacy doc is preserved
- **WHEN** a pre-upgrade doc whose latest version was saved by a user is synced
- **THEN** its content is unchanged, its origin becomes `human`, and exactly one `doc_adopt` Change is raised

### Requirement: Docs expose provenance fields
Doc API responses SHALL include `origin` (`generated` | `human`), `templateId` (nullable) and `lastSyncedAt` (nullable). Generating a doc from a template SHALL record that template on the doc.

#### Scenario: Template generation records the template
- **WHEN** a doc is generated with `POST /api/docs/generate` and a template
- **THEN** `GET /api/docs/{id}` returns that template's id as `templateId` and `origin` = `generated`
