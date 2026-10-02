# Design

## Context

Human docs (PR 2) give `origin=human`, `parent_id` nesting with a depth limit of five, and origin-aware lab ACLs. Attachments (PR 3) give the sniffing blobstore, `doc_attachments` rows and `attachment:<id>` links. The import maps an uploaded vault onto those primitives; it adds no schema.

## Goals / Non-Goals

Goals: import a Markdown or Obsidian vault zip as human docs, with a preview before anything is written and one atomic commit. Non-goals: BookStack and Wiki.js (follow-up issue), Obsidian canvas/dataview/plugins, transclusion of note content.

## Decisions

- **Two requests, staged on disk.** `POST /api/docs/import` streams the upload (100 MB cap) to `<attachments.import_dir>/<id>/upload.zip`, analyses it and writes `plan.json`. The plan already holds allocated doc and attachment IDs and rewritten content, so the commit only publishes blobs and inserts rows. Commit claims the staging directory with an atomic rename so a plan commits once; a failed commit releases the claim.
- **Zip guards** (in `docimport.Open`, before any content is used): entry names are normalised to forward slashes and rejected when absolute, drive-qualified, or containing `..`; more than 2000 entries is rejected; any entry whose declared ratio exceeds 100:1 (above 1 MiB) is rejected; bytes are counted while reading and the walk stops past 500 MB. Limits are fields of `Limits` so tests can shrink them.
- **File selection.** `.md`/`.markdown` notes (5 MiB each) and attachment extensions png/jpg/jpeg/gif/webp/pdf/txt are read; the attachment's first 512 bytes must pass `blobstore.Allowed`, so SVG and spoofed extensions are skipped. Hidden paths (`.obsidian/`, `.trash/`, `__MACOSX`) and every other file are skipped and reported, as are attachments that no note references.
- **Front-matter and titles.** A leading YAML block is parsed (`connector:` by connector ID or case-insensitive name, `title:`) and stripped. Title order: front-matter, first H1, file or folder name.
- **Folders become parent docs.** `index.md`, `README.md` or `<folder>.md` inside the folder supplies its content and front-matter; otherwise the folder doc is empty. Scope is inherited from the parent; a doc whose own `connector:` differs from its parent's scope is placed at its connector's root with a warning. Docs deeper than five levels are attached to their deepest allowed ancestor with a warning.
- **Link resolution follows Obsidian:** a target with a path matches the vault path, then the path relative to the note, then the shortest path ending with it; a bare name matches by basename, preferring the note's own folder, then the unique shortest path. Anything else is ambiguous or unresolved, left as written and reported. Code fences and inline code are not rewritten.
- **Rewrites:** `![[x.png]]`, `![[x.png|300]]`, `![](rel/x.png)` become `![x.png](attachment:<id>)`; `![[file.pdf]]` becomes `[file.pdf](attachment:<id>)`; `[[Note]]`, `[[Note|alias]]`, `[[Note#H]]` and relative `.md` links become `[text](/docs/<id>)`. Each doc gets its own attachment row per referenced file; identical bytes share one blob.
- **Commit** holds `blobstore.PublicationMu`, publishes blobs, then runs one `WithinTransaction` that locks the doc hierarchy, validates each parent, suffixes sibling title collisions with " (imported)" (then " (imported 2)", ...), creates docs with trigger `import` and inserts attachment rows. Embeddings sync in the background afterwards.
- **Expiry.** Plans carry `createdAt`; commit refuses plans older than one hour and a `docImportSweep` job every ten minutes removes expired staging directories.
- **Authz:** both routes use `auth.RequireInstanceAdmin`.

## Risks / Trade-offs

A malicious zip can still cost CPU up to the byte caps; the ratio check and counted reads bound memory and disk. Plans are trusted only because they live in the server's staging directory and are re-validated against the store in the commit transaction. Large vaults hold all note content in memory during analysis (bounded by 2000 notes of 5 MiB).

## Migration Plan

No migration. Deploy, then point `attachments.import_dir` at persistent or temporary storage; the sweep removes leftovers.
