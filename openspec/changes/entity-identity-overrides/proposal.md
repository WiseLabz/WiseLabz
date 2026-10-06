# Proposal

## Why

Entity identities are clustered only by automatic strong matches (same-kind external ID, hostname/alias). When those rules merge two distinct systems, or fail to join two observations of the same system, an administrator has no way to correct the result (#634). The identity schema already keeps identity metadata apart from connector-local members, so the correction can be stored as its own record.

## What Changes

- Add persisted identity overrides, each referring to connector-local members by `(connector, kind, ref)`:
  - **detach**: one member takes no part in automatic matching and gets its own identity.
  - **merge**: two members always share one identity.
- Apply overrides during reconciliation with a fixed precedence over automatic clustering. Existing redirect (`merged_into`, `merged_at`) and gone/return behaviour is unchanged.
- Define the override lifecycle: validation on creation, removal with a deleted connector, a dormant state while a member is not observed, purge together with the member history, and inclusion in backups.
- Add an instance-admin API to list, create and remove overrides. Each mutation reconciles immediately and is audited.
- Add an admin workflow in the web UI: detach and merge actions on the entity detail page and an overrides list with removal.

No breaking changes: without overrides, clustering behaves exactly as before.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `entity-identities`: strong-match clustering now yields to manual overrides; new requirements cover the override record, its lifecycle, the admin API and the admin workflow.

## Impact

- **Database**: new table `entity_identity_overrides` (migration `000061`, PostgreSQL and SQLite). No change to `entities` or `entity_members`.
- **Backend**: `internal/doc/identities.go` (clustering), `internal/store` (override storage, retention purge), `internal/api/entities` and `internal/api/routes_workflow.go` (admin endpoints), `internal/backup` (export/import), audit actions `entity.override.create` and `entity.override.delete`.
- **API**: `GET/POST /api/entity-overrides`, `DELETE /api/entity-overrides/{id}` in `docs/openapi.yaml`; generated web client.
- **Web**: `web/src/features/entities/`, i18n catalogs `en.ts` and `locales/pt-BR.ts`.
- **Docs**: `docs/AUDIT.md`, `docs/BACKUP.md`.
