# Proposal

## Why

Issue #494 enables maintained documentation alongside connector-generated docs. The agreed docs roadmap requires creation, nesting, soft deletion and a recoverable trash without exposing generated lab inventory to ordinary users.

## What Changes

- Human-origin docs with creator metadata, same-scope parents and maximum depth five.
- Create, rename, re-parent, subtree delete, administrator trash and batch restore APIs.
- Origin-aware lab ACLs and deleted-doc filtering across every consumer.
- Thirty-day configurable purge integrated with retention and portable v1 backup metadata.
- Creation dialog, palette action, nested draggable tree, editor deletion and trash UI.

## Capabilities

### New Capabilities

- `human-docs`: human authoring, hierarchy, lifecycle, access and recovery.

### Modified Capabilities

None. Generated ownership remains governed by doc-section-ownership.

## Impact

SQLite/Postgres migration 000051; store, docs API, chat/search, backups and retention; OpenAPI and generated web client; docs UI and architecture documentation. Attachments and import remain subsequent roadmap PRs.
