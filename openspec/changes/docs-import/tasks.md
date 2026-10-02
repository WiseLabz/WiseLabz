# Tasks

## 1. Import engine

- [ ] 1.1 Add `internal/docimport` zip walker with path, entry-count, counted-bytes and compression-ratio guards; zip-slip and zip-bomb tests.
- [ ] 1.2 Select notes and sniffed attachments, parse and strip front-matter, derive titles, map folders to parent docs with connector scoping and depth clamping.
- [ ] 1.3 Two-pass link rewrite: embeds to attachments, wikilinks and relative links to `/docs/<id>` with Obsidian resolution; report ambiguous and unresolved links; vault fixture tests.

## 2. API and storage

- [ ] 2.1 Store `ImportDocs` transaction with sibling collision suffixing and trigger `import`; tests.
- [ ] 2.2 Admin-only stage/preview and commit handlers with 100 MB upload cap, single-commit claim, background embedding sync; `attachments.import_dir` config.
- [ ] 2.3 One-hour expiry on commit and scheduled staging sweep; expiry and authz tests.
- [ ] 2.4 Update OpenAPI, regenerate the client, keep `TestOpenAPIMatchesRouter` green.

## 3. Web

- [ ] 3.1 Admin-only Import dialog on the docs page (upload, preview tree, warnings, confirm, navigate) with i18n strings.
- [ ] 3.2 Vitest coverage for the dialog with MSW.

## 4. Delivery

- [ ] 4.1 Update ARCHITECTURE.md and the roadmap PR 4 status; full Go and frontend checks; strict OpenSpec validation.
- [ ] 4.2 Refresh and commit graphify output; open the PR with `Closes #514`; file the BookStack/Wiki.js follow-up issue; fix CI until green.
