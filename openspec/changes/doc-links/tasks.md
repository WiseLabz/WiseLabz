# Tasks

## 1. Index and resolver

- [x] 1.1 Migration `000069_doc_links` for postgres and sqlite (up and down) with cascade and target index.
- [x] 1.2 Package `doclink` with `Extract` and `Resolve`; move the code-skipping scanner out of `docimport` and reuse it there; unit tests.
- [x] 1.3 Store index sync in `CreateDoc`, `UpdateDocWithVersion`, `UpdateDoc`, startup backfill, and lookups by title, display name and kind/ref; tests per writer and cascade.

## 2. API

- [x] 2.1 Resolve wikilinks in the save handler (and creation with content) under the saver's visibility; return `linkWarnings`.
- [x] 2.2 `GET /api/docs/{id}/backlinks` and `GET /api/entities/{id}/backlinks` with visibility tests; OpenAPI, regenerated client, `TestOpenAPIMatchesRouter` green.
- [x] 2.3 Lab Book HTML drops `/entities/` destinations.

## 3. Web

- [x] 3.1 Router links for internal hrefs in `Markdown.tsx` (not on share pages).
- [x] 3.2 "Referenced by" panel on the doc page and entity detail page.
- [x] 3.3 `[[` completion in the doc editor; buffer replaced by saved content, warnings shown; en and pt-BR strings; vitest.

## 4. Delivery

- [x] 4.1 Full checks green; strict OpenSpec validation.
- [ ] 4.2 Manual: type `[[title]]`, `[[vm:103]]`, an ambiguous name and a completion pick; rename the target; open the panels as a non-admin member and confirm hidden docs do not appear.
