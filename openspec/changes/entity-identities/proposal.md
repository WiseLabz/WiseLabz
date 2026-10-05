# Proposal

## Why

Connector snapshots currently identify objects only within each reporting connector. Persisted entity identities give the topology, findings, and search results a stable cross-connector reference that can survive syncs and later support entity pages (#502).

## What Changes

- Persist identity rows and connector-local members, assigning an identity to every observed snapshot entity.
- Merge only transitive strong matches: same-kind external IDs and hostname/alias matches. IP matches remain weak topology links.
- Keep the oldest entity ID through merges, retain losing IDs as redirects, create a new ID when an observation splits away, and retain gone identities until snapshot retention expires.
- Rebuild identities after successful sync and backfill them on leadership acquisition.
- Attach entity kind/ref to entity-specific compliance findings and entity search hits.

## Capabilities

### New Capabilities
- `entity-identities`: persisted identity lifecycle and references from findings and entity search.

## Impact

- Backend store migration 000058 for SQLite and PostgreSQL, identity reconciliation, retention cleanup, sync wiring, leadership backfill, findings, and search.
- No entity detail endpoint or web page is included; those are the follow-up PR for #502.
