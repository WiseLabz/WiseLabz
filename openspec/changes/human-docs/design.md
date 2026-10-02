# Design

## Context

See proposal.md for motivation. The store already models origin and generated ownership, uses transaction-scoped Store instances and shared doc scanners. Routes enforce connector grants in handlers. Retention is initialized through settings from cmd/server, and backups serialize DocRecord in v1 JSON.

## Goals / Non-Goals

Goals: keep ACL and visibility at shared read seams; atomically validate hierarchy and lifecycle writes across both databases; reuse scheduler and native browser drag/drop.
Non-Goals: attachments, backup ZIP, Markdown import and roadmap PR3/4.

## Decisions

- Add nullable parent_id (self FK SET NULL), deleted_at and created_by text with scope-parent and deletion indexes in migration 000051. SET NULL avoids accidentally purging newer independently deleted descendants through a self cascade.
- Serialize hierarchy writes in a transaction (Postgres table lock, SQLite existing single-writer transaction) and validate active ancestry plus subtree height. Validate lab parent visibility by origin to prevent generated inventory nesting from leaking ancestry.
- Preserve physical DeleteDoc for internal callers; add explicit subtree soft-delete and restore operations for API lifecycle. Restore only matching timestamp descendants, and detach the restored root when its parent remains deleted.
- Use shared scanners and active filters for ordinary reads. Backups use a dedicated all-state listing and a two-pass parent import to preserve references regardless of bundle order.
- Add deleted-doc retention to existing settings and scheduled cleanup, rather than a separate competing scheduler job.
- Tree nodes expose branch metadata to distinguish connector/lab grouping from real docs. Native drag/drop calls PATCH, with keyboard-accessible parent editing alongside it.

## Risks / Trade-offs

- Concurrent hierarchy changes → transaction-level serialization before ancestry validation.
- Deleted content leaks through raw SQL consumers → audit joins, FTS, embeddings, proposals, versions and existing public handlers, then exercise visibility tests.
- Timestamp batch collisions → use UTC nanosecond timestamps and traverse only the selected subtree.
- Backup order breaks parent FK → insert new docs without parents then assign parent IDs after all docs exist, validating bundle ancestry.

## Migration Plan

Deploy paired migration 000051 and regenerate the OpenAPI client. Existing rows remain live roots with no creator metadata. Down migration removes only new columns/indexes and retention setting; run both rollback passes and Postgres parity tests.
