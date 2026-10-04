# Design

## Context

The store already models each activity source and connector grants. MergedAttentionItems demonstrates SQL union filtering; the shared pagination envelope and cursor helpers provide the transport contract. Docs permission helpers distinguish lab-wide and connector scope.

## Goals / Non-Goals

Goals: preserve notes independently of source lifecycles, enforce permissions before pagination and make chronological paging stable. Non-goals: AI narration and security audit events in the journal.

## Decisions

- `journal_entries` stores id, body, occurred_at, created_at, updated_at, created_by (text without FK), nullable connector_id and doc_id (ON DELETE SET NULL), entity_kind, entity_name and entity_ref. Index occurred_at/id and connector_id/occurred_at/id descending. Notes are excluded from retention and included in backup import/export.
- Run a purpose-built UNION ALL on s.reader() across changes, sync_runs, alerts, doc_versions joined to live docs, journal_entries and admin-only audit_log. Push viewer grants and API-key connector restrictions into SQL; use EXISTS to avoid duplicates from manual/OIDC grants. Human lab-wide docs and lab-wide notes are visible to all signed-in users. Generated lab-wide docs follow existing admin visibility.
- Normalize timestamps to fixed UTC precision inside SQL before sorting on (timestamp, kind, id) descending. Use a composite opaque cursor and fetch an extra row to determine whether another page exists. Date range, connector, source kinds and all-sync-runs filters run in SQL. Default sync branch includes errors or positive change/alert counts.
- Include audit rows only for instance admins and an explicit allowlist of connector, change, alert, doc and runbook actions documented in docs/AUDIT.md. Security actions remain on Audit. Connector-scoped audit events obey grants/API-key restrictions; unscoped lab actions are admin-only.
- GET /api/timeline uses the shared paginated envelope with nextCursor. POST /api/journal and PUT/DELETE /api/journal/{id} perform in-handler checks and RecordAuditFromContext. Connector operators create; lab-wide creation requires instance admin. Authors and admins edit/delete after read access; moving an entry requires write permission on the destination. Optional docs must be visible and match entry scope, so links cannot disclose another connector.
- JournalPage uses useInfiniteQuery, URL filters and rail-and-dot activity rows linking to source pages. The entry dialog uses a native datetime input, scope select, EntityPicker, optional doc and Markdown preview, with edit/delete affordances for authors/admins.

- Owner follow-up decision: include serviceName in the Changes/Alerts list responses and send Changes/Alerts severity (and pending alert status) to server filters before pagination, with page reset on changes.

## Risks / Trade-offs

Mixed timestamp formats can break ordering → normalize inside SQL and test fractional and whole-second rows with cursor boundaries. Missing grant filters can leak source data → test all source branches and restricted keys. Deleted connectors/docs can lose context → nullable SET NULL links retain note content and entity text. Concurrent new events do not repeat prior pages; editing occurred_at can move a note during paging, requiring refresh.

## Migration Plan

Use 000054 while #500 reserves 000053, then recheck origin/main before PR creation and document numbering. Apply identical migration filenames in SQLite/PostgreSQL. Rollback drops journal entries and loses manual notes, so export first.
