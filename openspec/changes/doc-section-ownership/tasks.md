# Tasks

> Source plan: `docs/plans/docs-roadmap.md` (PR 1). Branch: `feat/doc-section-ownership`.
> Tick each box as you finish it, so whoever picks this up next (Claude or Codex) can resume.

## 1. Schema and store
- [x] 1.1 Add migration `000050_doc_section_ownership` (up/down, sqlite + postgres): `docs.origin`, `docs.template_id`, `docs.last_synced_at`, `docs.gen_keys`
- [x] 1.2 Extend `DocRecord` and every doc SELECT/scan in `backend/internal/store/doc.go` with Origin, TemplateID, LastSyncedAt, GenKeys. `CreateDoc` persists them.
- [x] 1.3 Add store methods `ApplyGeneratedRender`, `TouchDocsSynced`, `SetDocOrigin` and `GetLatestChangeByPattern`, with store tests
- [x] 1.4 Run `go test ./internal/store/...`, including the migration parity tests (sqlite only; Postgres parity needs WISELABZ_TEST_POSTGRES_DSN, not yet run)

## 2. Block model
- [x] 2.1 `backend/internal/doc/blocks.go`: `Block`, `Segment`, `ParseBlocks`, `RenderSegments`, `hashBody`
- [x] 2.2 Round-trip tests plus a fuzz test (malformed, nested, unclosed and CRLF markers)
- [x] 2.3 `backend/internal/doc/merge.go`: pure `Merge` with a table-driven test matrix (unedited, edited-only, both-changed, new key, removed unedited, removed edited, user-deleted not re-added, insertion order)

## 3. Render paths
- [x] 3.1 `renderSnapshot` returns blocks (`head`, `snap.<slug>`, `deps`, `related`) and drops the Fetched line
- [x] 3.2 Template `render` returns blocks (`head`, `tpl.<slug>`). `PreviewFromTemplate` still returns plain content (markers are OK).
- [x] 3.3 `GenerateFromTemplate` and `GenerateFromSnapshot` write marked content and set `template_id`, `gen_keys` and `origin`

## 4. Sync merge
- [x] 4.1 Rewrite `RegenerateForConnector`: skip human or locked docs, pick the template path, merge, use `ApplyGeneratedRender` with an expected version, handle `ErrVersionConflict`, `TouchDocsSynced`
- [x] 4.2 Legacy upgrade path (`gen_keys IS NULL`), classified by the latest version's author, plus a `doc_adopt` Change
- [x] 4.3 Conflict Changes with pattern dedup and supersede
- [x] 4.4 `Engine.OnDocUpdated` hook wired to `chat.SyncDocEmbeddings` in `cmd/server/main.go`
- [x] 4.5 Update `engine_test.go` (the existing RegenerateForConnector tests) and add sync merge integration tests (locked doc, version race, human origin, legacy author vs system)

## 5. Changes API
- [x] 5.1 `diffToSpec` handles the doc object form
- [x] 5.2 `ResolveDoc` handler and route `POST /api/changes/{id}/resolve-doc`, with audit
- [x] 5.3 Handler tests: accept and keep for conflict and adopt, 403 viewer, 404 no grant, 409 not resolvable, 409 block gone, 400 bad action
- [x] 5.4 `docs/openapi.yaml`: Doc fields (`origin`, `templateId`, `lastSyncedAt`), the resolve-doc path, `changeType` doc values. `TestOpenAPIMatchesRouter` passes.

## 6. Web
- [x] 6.1 `npm run gen:api`
- [x] 6.2 `Markdown.tsx`: remark plugin strips the wl:gen markers, plus a test
- [x] 6.3 `DocEditorPage.tsx`: generated-range tint plugin, plus "Synced <time>" in the header
- [ ] 6.4 `ChangeDetailPage.tsx`: Accept generated / Keep mine for doc Changes, plus tests with MSW handlers in `src/mocks`
- [ ] 6.5 `npm run lint && npm test`

## 7. Wrap-up
- [ ] 7.1 `cd backend && go vet ./... && go test ./...`
- [ ] 7.2 Update `docs/ARCHITECTURE.md` and `docs/DOC_EXPORT.md` notes on markers and ownership
- [ ] 7.3 `graphify update .`
- [ ] 7.4 Commit (no Claude attribution) and open the PR referencing #478
