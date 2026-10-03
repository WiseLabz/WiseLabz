# Design

## Context

Docs already have permission grants, parent IDs, origins, marker stripping and blob metadata. Reports select connector IDs and notification channel types. Reuse those paths rather than exposing the unscoped backup lister to the download route.

## Goals / Non-Goals

Goals: readable offline HTML and Markdown vault zip for any signed-in user, plus optional scoped report delivery. Non-goals: server PDF, pre-rendered SVG, Slack/webhook file upload.

## Decisions

- Page a with-content variant of ListViewableDocs with the same ACL and stable ordering. Reports select connector docs plus lab-wide docs, or everything for an empty connector filter.
- A new labbook package accepts docs, attachment metadata and a blob opener, writing to io.Writer for reuse by endpoints and reports.
- HTML uses Goldmark GFM and html/template with embedded CSS, print rules and the Mermaid bundle from web/package.json. No network is required. ID-derived anchors avoid non-ASCII slug collisions. Hierarchy is Lab then connector groups, with siblings sorted by title.
- Markdown zip uses archive/zip, hierarchy folders and index.md for parent docs, attachment-relative links and doc-relative links that docimport can resolve. Front matter preserves titles and connector scopes.
- HTML inlines images and lists other attachment filenames; active content is escaped. Internal links only resolve to exported docs.
- GET /api/docs/export?format=html|md.zip is static before /{id}, authenticated and streams a dated Content-Disposition download.
- DocsPage offers both formats using the shared blob download convention, for all signed-in users.
- report_definitions gains attach_lab_book (false by default) in SQLite and PostgreSQL. API/form use attachLabBook. Report channel email maps to the configured SMTP channel.
- Report downloads use the persisted snapshot slug and period-end UTC date (`report-<slug>-<date>.<format>`), surviving definition deletion. Legacy snapshots missing a valid slug use their report ID.
- Optional notification attachments travel through NotifyReport and senders. Email is multipart mixed, Discord multipart upload; other channels ignore attachments. Conservative per-channel size caps fall back to text with a note.

## Risks / Trade-offs

Mermaid increases HTML size and executes only offline rendering with strict security. Building report attachments needs temporary storage and bounded channel payloads; sender errors preserve existing delivery tracking. Missing blobs fail exports rather than silently losing attachments.

## Migration Plan

Use the next free migration on origin/main at PR creation, identical filenames across dialects; record it in the PR for renumbering after rebase with #500.
