# Docs roadmap: section ownership, human docs, attachments, import

> **Handoff note.** This plan comes from a grilling session (2026-10-01). It covers GitHub issues
> #478 → #494 → #519 → #514, shipped as **4 sequential PRs**. Each PR gets its own OpenSpec change under `openspec/changes/`.
>
> Status:
> - PR 1 (#478): **merged as #598, archived.** Spec: `openspec/specs/doc-section-ownership/spec.md`;
>   change history: `openspec/changes/archive/2026-10-02-doc-section-ownership/`.
> - PR 2 (#494): **merged as #600, archived.** Spec: `openspec/specs/human-docs/spec.md`; change history: `openspec/changes/archive/2026-10-02-human-docs/`. Review follow-ups filed as #601, #602, #603.
> - PR 3 (#519): **in review as [PR #605](https://github.com/WiseLabz/WiseLabz/pull/605), targeting `main`;** change: `openspec/changes/doc-attachments/`.
> - PR 4 (#514): **in review, targeting `main`;** change: `openspec/changes/docs-import/`. BookStack/Wiki.js import is a follow-up issue.
> - Each change was created with `/opsx:propose` (or by hand under `openspec/changes/<name>/`)
>   from the matching section below, then implemented with `/opsx:apply`.
>
> Every decision in the table below was made explicitly by the maintainer. Don't re-litigate them.
>
> Gotchas found while building PR 1:
> - **API client regeneration.** Before editing `docs/openapi.yaml`, run `cd web && bun install --frozen-lockfile`.
>   A stale `node_modules` (orval 8.19, or the `typescript` npm alias) makes `gen:api` fail *after* it has
>   already cleaned `web/src/api`. The pre-commit hook then stages the wiped folder.
>   Always check `git show --stat HEAD` after a commit that touches the spec.
> - **No nested transactions.** `store.WithinTransaction` doesn't nest: inside a tx, call the tx-scoped
>   helpers (`updateDocRev`, `CreateDocVersion`) directly.
> - **New migrations need two test updates.** Each new latest migration needs a step in `TestRunMigrationsDown`
>   (`backend/internal/store/migrations_test.go`), in both the first rollback pass and the reapply pass.
> - **Doc content now carries `wl:gen` markers.** Code that compares or displays raw doc content should use
>   `doc.StripMarkers` (backend) or `remarkStripGenMarkers` / `findGenBlocks` (`web/src/lib/genMarkers.ts`).
> - **PR 2 must set `origin=human` on created docs.** Human docs (#494) must be created with
>   `Origin: store.DocOriginHuman`, or sync will treat them as legacy generated docs.

# Plan: Human docs, attachments, Markdown import (#478 → #494 → #519 → #514)

## Context
Issues #519 (doc attachments) and #514 (Markdown/Obsidian import) both depend on things that don't exist yet:
- **No user-created docs.** Docs come only from generation or backup import (#494 is still open).
- **Sync clobbers human edits.** `Engine.RegenerateForConnector` (`backend/internal/doc/engine.go:288`) rewrites every connector doc on every sync. The volatile `**Fetched:**` line (l.221) means the "unchanged" check never matches. #478 was closed, but the fix never landed.
- **No file storage or uploads.** Auth is a Bearer header only, so a plain `<img>` can't authenticate.
- **Backups are one JSON bundle** capped at 10 MB.

Result: 4 sequential PRs. Each is its own OpenSpec change (`/opsx:propose`), and each builds on the previous one. Reopen #478 first.

## Decisions (from the grilling session)
| Area | Decision |
|---|---|
| Sync ownership (#478) | Inline markers `<!-- wl:gen key="…" h="<sha12>" -->…<!-- /wl:gen -->`, plus `docs.template_id`. Text outside markers is human-owned. |
| Conflict | Human-edited block whose upstream also changed: leave it, raise a Change (`diff.format="doc"`, `affected_doc_ids`) with **Accept generated** / **Keep mine** (detach). |
| Fetched line | Removed from content. New column `docs.last_synced_at`, shown in the doc header. |
| Existing docs | First sync after upgrade: latest version generated → rewrite with markers. Latest version human → doc stays as-is, and one "adopt generated blocks" Change is raised. |
| Human docs (#494) | Full scope: `origin` (generated/human), `parent_id` nesting, soft delete, trash with 30-day purge. |
| ACL | Human lab docs: view by any authenticated user, edit by instance admin. Generated lab docs (Topology): admin only. Service-scoped docs: connector viewer reads; operator creates/edits/deletes/uploads. |
| #494 UI | "New doc" dialog, palette action, trash page, drag to re-parent in tree. |
| Attachments (#519) | Belong to one doc and inherit its ACL. Bytes on disk, content-addressed by sha256, metadata in `doc_attachments`. |
| Allowlist / size | png, jpeg, gif, webp, pdf, text/markdown. Magic-byte sniffed. No SVG. 25 MB, configurable. |
| Serving | Short-lived HMAC-signed URLs, key derived from `auth.secret` via HKDF label `wiselabz-attachment-url`. |
| Link format | `attachment:<id>` in the Markdown. |
| Editor UX | Drag-drop and paste upload, attachments side panel, inline PDF viewer, image lightbox. |
| Share links | Attachments visible, scoped to docs inside the shared tree. |
| Backups | v2 = zip (`bundle.json` + `attachments/<sha256>`). v1 JSON import still supported. |
| Doc export | Writes attachment files and rewrites links to relative paths. Flag `doc_export.include_attachments` (default true). |
| Import (#514) | Markdown/Obsidian zip only; BookStack and Wiki.js become a follow-up issue. Front-matter `connector:`/`title:`, folders become parent docs. Image embeds become attachments, `[[wikilinks]]` become `/docs/<id>`. Preview, then commit. Admins only. Limits 100 MB zip / 2000 entries / 500 MB unzipped. |

> **Migration numbering:** the latest migration is `000045`, and the `mcp-search-topology-proposals` change expects `000046+`. Take the next free number when implementing. Every migration needs a sqlite **and** postgres pair (`migrations_parity_test.go`).

---

## PR 1: #478 Section ownership in sync
**Schema:** `docs.template_id` (nullable, FK templates `ON DELETE SET NULL`), `docs.last_synced_at`, `docs.origin TEXT NOT NULL DEFAULT 'generated'` (CHECK generated|human; introduced here so sync can skip human docs).

**Backend**
- New `backend/internal/doc/blocks.go`: a lossless `ParseBlocks(content) []Segment` (human text | gen block {Key, Hash, Body}) and `RenderBlocks`. Round-trip property test: `Render(Parse(x)) == x`. Do not reuse the lossy `chat.SplitSections`.
- Render paths wrap their output in blocks:
  - `render()`: one block per template section, key `tpl.<sectionID>`.
  - `renderSnapshot()`: key `snap.<slug(section title)>`, plus `meta` (Type line), `deps` and `related`. Drop the Fetched line.
  - `GenerateFromTemplate` persists `template_id`. Sync re-renders through the template when it is set.
- `RegenerateForConnector` becomes a merge:
  - Skip `origin=human` docs.
  - Skip docs with a live `doc_lock`; they are retried next sync.
  - Unedited block (body hash == h): replace it. Edited block where the new hash == h: no-op. Edited block where both sides changed: conflict Change.
  - New keys are inserted after the preceding key, or appended at the end. Removed keys are deleted when unedited and kept (detached) when edited.
  - Save through `UpdateDocWithVersion(expected=CurrentVersion)`; on `ErrVersionConflict`, skip and retry next sync.
  - Only bump `updated_at` and write a version when content changed. Always set `last_synced_at`.
  - Call `chat.SyncDocEmbeddings` after an update (currently missing).
- Conflict Change:
  - `change_type="doc_conflict"`, diff `{format:"doc", hunks:[{path:key, before:human, after:generated}]}`, `affected_doc_ids=[docID]`.
  - Dedup: skip if an open Change already exists for the same doc+key+new hash.
  - Built in `backend/internal/store/change.go` (`CreateChange`). `diffToSpec` in `backend/internal/api/changes/handlers.go:43` must pass the doc format through.
- New `POST /api/changes/{id}/resolve-doc {action: accept|keep}` (operator on the connector). `accept` swaps in the generated body with a fresh hash; `keep` strips the markers (detach). Both save as a versioned write with trigger `sync-resolve`. Then the Change is acknowledged.
- Upgrade path: a doc with no markers is classified by its latest `doc_versions.trigger`/`author`. Verify which triggers the PUT client sends; `manual` is ambiguous with `GenerateFromSnapshot`, so author non-empty means human.
  - Generated: re-render with markers.
  - Human: leave it, and raise one `doc_adopt` Change whose accept action replaces the doc with the marked render.

**Web**
- `web/src/components/docs/Markdown.tsx`: a remark plugin drops the `wl:gen` HTML comment nodes.
- `DocEditorPage.tsx`: a CodeMirror decoration subtly tints generated ranges, with a tooltip "Generated: edits will be reviewed on sync". The header shows `last_synced_at`.
- `ChangeDetailPage.tsx`: Accept / Keep buttons for `doc_conflict` and `doc_adopt`.

**Tests**
- Update `engine_test.go` (l.344, 375, 433), plus new merge-matrix tests: unedited, edited-only, both-changed, new key, removed key, locked doc, version race.
- Resolve-endpoint authz tests (403 vs 404).
- Update `openapi.yaml` and keep `TestOpenAPIMatchesRouter` green.

## PR 2: #494 Human-written docs
**Schema:** `docs.parent_id` (FK docs), `docs.deleted_at`, `docs.created_by`. Index `(service_id, parent_id)`.

**Backend** (`backend/internal/api/routes_docs.go`, `backend/internal/api/docs/`, `backend/internal/store/doc.go`)
- `POST /docs {title, serviceId?, parentId?, content?}`: origin=human, kind=`lab` when there is no serviceId, otherwise `service`. Writes a v1 version with trigger `create`.
- `PATCH /docs/{id} {title?, parentId?}` for rename and re-parent. The parent must be in the same scope; reject cycles; max depth 5.
- `DELETE /docs/{id}`: soft-deletes the subtree with one shared `deleted_at`.
- Admin-only `GET /docs/trash` and `POST /docs/{id}/restore` (restores that batch).
- `permissions.go`: split the lab rule by origin as in the decisions table. Service docs keep `requireDocViewer`/`requireDocOperator`.
- Filter `deleted_at IS NULL` everywhere: `GetDoc`, `ListDocsByService`, `ListViewableDocs`, `ListDocsGroupedByService`, `ListAllDocsWithContent`, chat/MCP/embeddings, docexport, share links, quality checks.
- Tree: nested children, plus a "Lab" branch. Fix the leak where generated lab doc titles are listed to non-admins, and update the stale comment.
- Purge job (scheduler in `cmd/server/main.go`): hard-delete docs where `deleted_at` is older than `retention.deleted_docs_days` (30). Cascades versions, locks and embeddings.
- Backup bundle carries the new columns (still v1 JSON in this PR).

**Web:** NewDocDialog (title, scope picker filtered to connectors the user operates plus lab for admins, parent), command palette "New doc", nested `DocTree.tsx` with drag to re-parent (use an existing DnD dependency if one is present, otherwise native HTML5 DnD → PATCH), delete action in the editor, `TrashPage` at `/docs/trash`. Regenerate the orval client.

**Tests:** handler authz matrix (lab human/generated × admin/user; service × viewer/operator/none), cycle/scope validation, soft-delete visibility in every list path, purge job.

## PR 3: #519 Attachments
**Schema:** `doc_attachments(id, doc_id FK ON DELETE CASCADE, sha256, filename, content_type, size, created_by, created_at)`, indexed on doc_id and sha256.

**Backend**
- New `backend/internal/blobstore`:
  - `Put(r)` streams to a temp file under `attachments.dir` (default `/data/attachments`) while hashing.
  - It enforces `attachments.max_bytes` (25 MB) and sniffs the type with `http.DetectContentType` against the allowlist.
  - It then atomically renames the file to `ab/cd/<sha>`. Also provides `Open` and `Delete`.
- Routes:
  - `POST /docs/{id}/attachments` (multipart, `http.MaxBytesReader`, doc operator).
  - `GET /docs/{id}/attachments` (viewer).
  - `DELETE /docs/{id}/attachments/{aid}` (operator).
  - Unauthenticated `GET /api/attachments/{aid}/raw?exp=&sig=`: HMAC over aid+exp, 15-minute TTL. Headers: `nosniff`, `CSP: sandbox`, `Cache-Control: private`. Disposition is inline for images and PDF, otherwise attachment. `frame-ancestors 'self'` so the PDF iframe works.
- `GET /docs/{id}` and the share doc endpoint include `attachments[{id, filename, contentType, size, url}]` with signed URLs. Share links only sign attachments of docs inside the shared tree.
- `secheaders.go`: add `frame-src 'self'` for the PDF viewer.
- GC sweep (runs with the PR 2 purge job): delete a blob when no `doc_attachments` row references its sha.
- **Backup v2:**
  - `backend/internal/backup` writes a `.zip` (`bundle.json` + `attachments/<sha>`) with manifest checksums.
  - `Import` sniffs the zip magic number vs JSON and streams to a temp file. The import cap moves to config (default 1 GB).
  - Update `verify.go` and `cmd/backup`.
- **Doc export** (`backend/internal/docexport/export.go`): write `attachments/<sha>.<ext>` and rewrite `attachment:` links to relative paths. Git mode skips files over `doc_export.max_attachment_bytes`.

**Web**
- `Markdown.tsx`: a custom `urlTransform` allows `attachment:` and maps it to the signed URL. An unknown id shows a placeholder. Images open a lightbox. PDF links render as a card with Open and Preview (iframe).
- Query `staleTime` is shorter than the URL TTL so URLs refresh before they expire.
- `DocEditorPage.tsx`: CodeMirror `domEventHandlers` for drop and paste insert an `![Uploading…]()` placeholder and replace it with `![name](attachment:id)`.
- `AttachmentsPanel`: list, insert, delete, and an "unused" badge.

**Tests:** sniff/allowlist/size limits (including a spoofed extension), signature expiry and tampering, ACL inheritance, share scoping, backup v2 round-trip plus v1 compatibility, export link rewrite, and a Vitest test for the drop/paste upload with MSW.

## PR 4: #514 Markdown/Obsidian import
**Backend**
- New `backend/internal/docimport`:
  - Zip walker with guards: clean paths with no absolute paths or `..`, ≤2000 entries, ≤500 MB counted while reading, compression-ratio check.
  - Only `.md`/`.markdown` files and allowlisted attachment types are read; everything else is skipped and reported.
  - Parse YAML front-matter (`connector:` matched by name or id, `title:`), then strip it.
  - Title order: front-matter, first H1, filename.
- Folders become parent docs. `index.md`, `README.md` or `<folder>.md` supplies the folder doc's content; otherwise the folder doc is empty. A connector doc under a lab-scoped folder is placed at its connector's root, with a warning.
- Two passes: allocate doc IDs, then rewrite content.
  - `![[x.png]]`, `![[x.png|300]]`, relative `![](…)` and `![[file.pdf]]` become blobstore attachments with `attachment:` links.
  - `[[Note]]`, `[[Note|alias]]`, `[[Note#H]]` and relative `.md` links become `/docs/<id>`. Resolution follows Obsidian's basename-then-shortest-path rule. Ambiguous or unresolved links are left as text and reported.
- Routes (admin only):
  - `POST /docs/import` (multipart, 100 MB) stages the zip under `/data/imports/<id>` and returns a preview: tree, mappings, attachment count, warnings, skipped files, collisions.
  - `POST /docs/import/{id}/commit` creates all docs in one transaction with origin=human and trigger `import`. A title collision in the same parent gets the suffix " (imported)". Embeddings sync afterwards.
  - Staged imports expire after 1 hour (sweep job).
- Afterwards, file a follow-up issue for BookStack and Wiki.js.

**Web:** an Import dialog on the docs page (admins): upload, preview tree with warnings, confirm, then navigate to the new docs.

**Tests:** a fixture Obsidian vault zip (nested folders, front-matter, embeds, aliases, ambiguous names), zip-slip and zip-bomb fixtures, collision suffixing, admin-only authz, staged-import expiry.

---

## Verification (per PR)
- Backend: `cd backend && go test ./...`, plus Postgres parity with `WISELABZ_TEST_POSTGRES_DSN` set (migration schema parity and integration paths).
- Frontend: `cd web && npm run lint && npm test`. Regenerate orval after each `docs/openapi.yaml` change; `TestOpenAPIMatchesRouter` must pass.
- Manual run of the app (`/run`):
  - **PR 1:** sync a connector twice; there should be no new version. Edit a generated block, change the upstream, sync, and resolve the resulting Change both ways.
  - **PR 2:** create, nest, drag, delete and restore a lab doc as admin and as a non-admin.
  - **PR 3:** drop a photo and a PDF into the editor; preview, lightbox, PDF iframe, open via a share link, signed URL expiry, backup export and re-import on a fresh DB, doc export folder.
  - **PR 4:** import a real Obsidian vault zip; check the preview, commit, then images and wikilinks.
- After code changes, run `graphify update .`.
