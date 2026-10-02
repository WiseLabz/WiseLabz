# Design: doc section ownership (#478)

## Context
- `RegenerateForConnector` (`backend/internal/doc/engine.go`) overwrites every doc of a connector with `renderSnapshot` output.
  - The content always differs because of `**Fetched:** <time>`.
  - It ignores edit locks and `expectedVersion`, and it never re-applies templates, because docs don't store `template_id`.
- `chat.SplitSections` loses information, so it can't be used to reassemble a doc.
- The `changes` table already has `affected_doc_ids`. The OpenAPI `Diff` schema and the web DiffViewer already support `format: doc` (`baseText`/`headText`), but nothing emits it yet.

## Goals / Non-goals
- **Goals:** never lose human text on sync; route real conflicts to review; stop writing a version on every sync; keep templates applied.
- **Non-goals:** creating human docs or nesting them (#494), three-way text merge inside a block, AI-assisted conflict resolution.

## Decisions

### 1. Marker format and hashing
```
<!-- wl:gen key="snap.containers" h="3fa9c0d1e2b4" -->
…body…
<!-- /wl:gen -->
```
- Each marker sits on its own line.
  - Open marker regex: `^<!-- wl:gen key="([A-Za-z0-9._-]+)" h="([0-9a-f]{12})" -->$`
  - Close marker: `^<!-- /wl:gen -->$`
- The body is the text between the two marker lines, excluding the newline after the open marker and the one before the close marker.
- `h` is the first 12 hex characters of sha256(body). An open marker with no matching close, or one nested inside another block, is parsed as plain human text (fail safe: never delete).
- `doc/blocks.go` provides `ParseBlocks(string) []Segment` (each segment is `{Text}` or `{Block{Key, Hash, Body}}`) and `RenderSegments([]Segment) string`. It guarantees `RenderSegments(ParseBlocks(x)) == x` for every input, including malformed markers and CRLF. This is enforced by a fuzz/property test.
- HTML comments keep the doc as valid Markdown in exports, git and backups. The web renderer drops them.

### 2. Block keys
| Render path | Blocks |
|---|---|
| Snapshot (`renderSnapshot`) | `head` (`# Name` + `**Type:**`), one `snap.<slug(section.Title)>` per snapshot section (duplicate slugs get `-2`, `-3`, …), `deps`, `related` |
| Template (`render`) | `head` (`# Name` + description quote), one `tpl.<slug(section.Title)>` per template section (including its `## Title` line) |

Keys come from section titles rather than template section row IDs, so they survive template re-saves. `GeneratedAt` stays available to templates. A template that prints it will refresh its block on every sync; this is documented in the template schema help text.

### 3. Schema (migration `000050_doc_section_ownership`, sqlite + postgres)
- `docs.origin TEXT NOT NULL DEFAULT 'generated' CHECK (origin IN ('generated','human'))`
- `docs.template_id TEXT NULL REFERENCES templates(id) ON DELETE SET NULL`
- `docs.last_synced_at TEXT NULL`
- `docs.gen_keys TEXT NULL`: a JSON array of the block keys in the last applied render. `NULL` means the doc has never been rendered with markers (a legacy doc).

On SQLite, `ADD COLUMN` works for the nullable columns and for `origin` with its constant default. Use the table-rebuild pattern only if the FK on `ADD COLUMN` is rejected (see `000011`/`000024` for the pattern). Down migrations drop the columns.

`DocRecord` gains `Origin`, `TemplateID`, `LastSyncedAt`, `GenKeys`, and every doc SELECT and scan includes them. A new store method `ApplyGeneratedRender(ctx, id, content, genKeys, expectedVersion, trigger)` writes content, `gen_keys`, `last_synced_at` and a version in one transaction. `TouchDocsSynced(ctx, ids)` handles the no-content-change case.

### 4. Merge algorithm (`doc/merge.go`, pure function)
```
Merge(existing []Segment, prevKeys set, fresh []Block) (out []Segment, conflicts []Conflict, changed bool)
for seg in existing:
  text  → keep
  block k (edited := hash(body) != h):
    fresh has k:
      !edited            → replace body/hash with fresh
      hash(fresh)==h     → keep (upstream unchanged)
      else               → keep, conflict{k, human: body, generated: fresh.body}
    fresh lacks k:
      !edited → drop
      edited  → detach (emit body as Text)
for fresh block f not present in existing and f.Key ∉ prevKeys:   # truly new upstream
  insert after the block (in out) of the nearest preceding fresh key, else append
```
`prevKeys` comes from `gen_keys`, so a block the user deleted is never re-inserted. After a merge, `gen_keys` is set to the keys of `fresh`. The function is pure and tested with a table-driven matrix.

### 5. `RegenerateForConnector`
For each doc in `ListDocsByService` with `origin='generated'`:
1. Skip it if `GetDocLock` returns a live lock.
2. Render: `render(templateID)` if `template_id` is set and the template exists; otherwise `renderSnapshot`. Both return `[]Block`, and `RenderSegments` builds the content.
3. If `gen_keys IS NULL`, run the legacy upgrade (§7). Otherwise run `Merge`.
4. If the content changed, call `ApplyGeneratedRender(expected=CurrentVersion, trigger="sync", author="")`. On `ErrVersionConflict`, log it and skip; the next sync retries. If it didn't change, call `TouchDocsSynced`.
5. Raise or refresh conflict Changes (§6).
6. After a content change, call the optional `Engine.OnDocUpdated(ctx, docID)` hook. It is wired in `cmd/server/main.go` to `chat.SyncDocEmbeddings` to refresh embeddings; the hook avoids a doc→chat import cycle.

One doc failing does not abort the others. Errors are collected with `errors.Join`.

`GenerateFromTemplate` and `GenerateFromSnapshot` (explicit user actions) still write the whole doc. They now produce marked content, set `gen_keys` and `origin='generated'`, and set `template_id` (or clear it for snapshot generation).

### 6. Conflict and adopt Changes
- Stored `changes.diff` is a JSON **object** for doc Changes, which tells it apart from the infra `[]DiffPatch` array:
  `{"format":"doc","docId":…,"key":…,"human":…,"generated":…,"genHash":…}`. For adopt, `key` is `""` and `human`/`generated` hold whole-doc content.
- `pattern_id` = `doc_conflict:<docId>:<key>` or `doc_adopt:<docId>`. Dedup uses a new store method, `GetOpenChangeByPattern(patternID)` (status `new`):
  - Same `genHash`: skip.
  - Different `genHash`: dismiss the old Change and create a new one.
- Fields: `change_type` = `doc_conflict` | `doc_adopt`, `severity` = `info`, `service_id` = connector, `affected_doc_ids` = `[docId]`. No AlertRecord is created.
- `diffToSpec` in `api/changes/handlers.go` detects the object form and returns:
  `{format:"doc", baseText: human, headText: generated, baseLabel:"Current doc", headLabel:"Generated", headTrigger:"sync", language:"md"}`.

### 7. Legacy upgrade
When `gen_keys IS NULL`, look at the latest `doc_versions` row (via `GetDocVersions`):
- **No versions, or `author == ""`** (sync, template, generate): replace the content with the fresh marked render and set `gen_keys`.
- **`author != ""`** (PUT saves carry the user id): set `origin='human'` and leave the content as-is. Raise a `doc_adopt` Change whose `generated` holds the full marked render.
  - Because the doc is now human, sync never looks at it again, so the Change can't be duplicated.

### 8. `POST /api/changes/{id}/resolve-doc`
Handler `ResolveDoc` in `api/changes`:
1. Load the Change and check the grants: viewer, otherwise 404; operator, otherwise 403.
2. Require `change_type ∈ {doc_conflict, doc_adopt}` and `status == new`, otherwise 409 `not_resolvable`.
3. Decode `{action}` (400 if it is not `accept`/`keep`) and load the doc.
4. Conflict:
   - Parse the doc and find the block by key (409 `block_gone` if it's missing).
   - `accept` sets the body to `generated` with a fresh hash.
   - `keep` replaces the block with a plain text segment holding its body.
   - Save with `UpdateDocWithVersion(expected=current, author=user, trigger="sync-resolve")`.
5. Adopt:
   - `accept` writes the `generated` content, sets `origin='generated'` and `gen_keys`, in one transaction, then saves the version.
   - `keep` changes nothing.
6. Set the status to `acknowledged`, record the audit `change.resolve_doc` with `{action, docId}`, broadcast the doc-updated WS event the editor already listens for if one exists, and return `changeDetail`.

Route: `r.Post("/{id}/resolve-doc", d.changeH.ResolveDoc)` in `routes_workflow.go`, plus an OpenAPI entry (`TestOpenAPIMatchesRouter`).

### 9. Web
- `components/docs/Markdown.tsx`: a small remark plugin removes `html` nodes matching `/^<!-- \/?wl:gen/`. This is needed whether raw HTML is escaped or ignored. It covers the share-link view too.
- `features/docs/DocEditorPage.tsx`:
  - A CodeMirror `ViewPlugin` adds a line class (subtle tint) to the lines inside generated blocks, and dims the marker lines.
  - Tooltip: "Generated from <connector>: edits here are reviewed on the next sync".
  - Header shows "Synced <relative time>" from `lastSyncedAt`.
- `features/changes/ChangeDetailPage.tsx`: for `doc_conflict`/`doc_adopt`, show **Accept generated** / **Keep mine** buttons that call the generated `resolve-doc` mutation, then invalidate the change and doc queries.
- Regenerate the client with `npm run gen:api`.

## Risks / Trade-offs
- **A user mangles a marker line in the editor.** It parses as human text and is never deleted. The block effectively detaches and its key stays in `gen_keys`, so it isn't re-inserted. This is acceptable and documented in the tooltip.
- **Volatile values inside connector section content** (counts, statuses) still cause version churn when they really change. That is intended: it's real data.
- **Templates printing `GeneratedAt`** churn every sync. This is documented.
- **Sections without headings** (Docker) still render as before; only the markers are added.

## Migration / Rollback
- The up migration adds columns. The first sync does the upgrade per doc.
- Rollback with the down migration drops the columns. Content keeps the marker comments, which render harmlessly (the old renderer escapes or ignores them). Old sync logic would overwrite the content as before.
