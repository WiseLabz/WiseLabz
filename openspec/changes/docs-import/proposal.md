# Proposal

## Why

Issue #514 asks for importing existing notes. Admins arriving from Obsidian or a folder of Markdown files currently have to recreate every doc by hand. PR 2 (#600) added human docs with nesting and PR 3 (#605) added attachments, so an import can now map a vault onto docs, parents and attachments without loss.

## What Changes

- New `internal/docimport` package: a guarded zip walker (clean relative paths, at most 2000 entries, at most 500 MB read, compression-ratio check), front-matter parsing (`connector:`, `title:`), folder-to-parent-doc mapping and a two-pass link rewrite (Obsidian embeds and wikilinks, relative Markdown links).
- Admin-only `POST /api/docs/import` stages an uploaded zip (100 MB) and returns a preview; `POST /api/docs/import/{id}/commit` creates every doc in one transaction (origin `human`, version trigger `import`), suffixing title collisions with " (imported)" and syncing embeddings afterwards.
- Staged imports expire after one hour and a scheduled sweep removes them.
- Web: an admin-only Import dialog on the docs page (upload, preview tree with warnings, confirm, navigate).
- BookStack and Wiki.js imports are out of scope and filed as a follow-up issue.

## Capabilities

### New Capabilities

- `docs-import`: Markdown/Obsidian vault import with preview and commit.

### Modified Capabilities

None.

## Impact

New `backend/internal/docimport`, docs API handlers and routes, store import transaction, `attachments.import_dir` config (default `/data/imports`), a scheduler job, `docs/openapi.yaml` and the regenerated web client, and the React docs page. No migration: imports reuse the PR 2 and PR 3 schema.
