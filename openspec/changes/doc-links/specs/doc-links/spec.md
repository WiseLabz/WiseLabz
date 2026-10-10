# Spec Delta

## ADDED Requirements

### Requirement: Wikilink resolution on save
Saving a doc SHALL rewrite `[[target]]` and `[[target|label]]` into Markdown links to `/docs/<id>` or `/entities/<id>`. A bare name SHALL match a visible doc by exact case-insensitive title, then a visible entity by display name; `[[kind:ref]]` SHALL match an entity by kind and connector reference. With several matches or none the text SHALL stay unchanged and a warning SHALL be returned. Code, code spans, `![[...]]` embeds and generated marker sections SHALL NOT be touched.

#### Scenario: Ambiguous title
- **WHEN** two visible docs share a title and a doc contains `[[That Title]]`
- **THEN** the text is kept and `linkWarnings` names the ambiguity

#### Scenario: Hidden target
- **WHEN** the only match is a doc the saver cannot view
- **THEN** the link is not resolved and the response does not reveal the doc

### Requirement: Link index
Every write of doc content SHALL keep a `doc_links` index of `/docs/<id>` and `/entities/<id>` targets in the same transaction, and deleting a source doc SHALL remove its rows.

#### Scenario: Restore a version
- **WHEN** a doc version is restored
- **THEN** the index matches the restored body

### Requirement: Backlinks
`GET /api/docs/{id}/backlinks` and `GET /api/entities/{id}/backlinks` SHALL list `(id, title)` of source docs the caller may view, excluding soft-deleted docs; the entity variant SHALL include links to entities merged into it. Share routes SHALL NOT expose them.

#### Scenario: Hidden source
- **WHEN** a non-admin reads backlinks and one source doc is outside their grants
- **THEN** that doc is not listed

### Requirement: Editor completion and rendering
Typing `[[` in the editor SHALL offer docs and entities from search and insert the Markdown link; internal links SHALL render as router links outside share pages; doc and entity pages SHALL show a "Referenced by" panel.

#### Scenario: Pick a completion
- **WHEN** the user types `[[prox` and picks a doc
- **THEN** a Markdown link to that doc is inserted
