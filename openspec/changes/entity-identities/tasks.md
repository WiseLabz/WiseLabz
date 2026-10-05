# Tasks

- [x] Add SQLite and PostgreSQL migration 000058 for entities, memberships, and quality-finding entity references; cover migration behavior.
- [x] Reconcile snapshot entities into transitive strong-match clusters while preserving IDs, redirects, split behavior, and gone rows.
- [x] Run identity reconciliation after successful sync and backfill on leadership acquisition.
- [x] Purge gone and merged identity rows using snapshot retention.
- [x] Persist entity references on entity-specific compliance findings and include `entityId` in search hits.
- [x] Verify store, doc, sync, quality, retention, server lifecycle, and migration tests; run required backend tests and lint.
- [x] Update graphify output and open a ready-for-review PR referencing #502 as “Part of #502”.
- [x] Round 4: notify once per rule per connector across new, flapping and escalating entities; deterministic ID assignment by oldest identity; `merged_at` column and merged-row purge keyed on it (#632).
