# Tasks

## 1. Store, reconciliation and backup (PR 1)

- [x] 1.1 Add migration `000061_entity_identity_overrides` (up and down) for PostgreSQL and SQLite with the table, connector foreign keys and the unique index; verify up, down, up succeeds on both drivers
- [x] 1.2 Add `backend/internal/store/entity_identity_overrides.go` with create (validation, sorted merge pair, conflict error), list (member names, connector names, state, current identity IDs) and delete; verify store tests cover unknown member, same member twice, cross-kind merge, reversed duplicate and connector-delete cascade
- [x] 1.3 Make `identityClusters` in `backend/internal/doc/identities.go` take overrides and apply the detach, automatic, merge precedence; load overrides inside the reconcile callback of `BackfillEntityIdentities`; verify unit tests for detach leaving a hostname cluster, merge joining unmatched members, detach plus merge moving a member, and no-override output unchanged
- [x] 1.4 Add reconciliation tests through the store: a manual merge leaves a `merged_into` redirect with `merged_at`, a detach gives one side a new identity and keeps the old ID on the other, removing an override restores the automatic result, a dormant override applies again when the member returns; verify they pass on both drivers
- [x] 1.5 Purge overrides without any remaining member row in `DeleteExpiredEntityIdentities`; verify a retention test shows the override deleted in the same run as the member history
- [x] 1.6 Add the override table to backup export and import and describe it in `docs/BACKUP.md`; verify a backup round-trip test restores the overrides

## 2. Admin API, audit and OpenAPI (PR 2)

- [ ] 2.1 Add list, create and delete handlers in `backend/internal/api/entities/` that reconcile after a mutation and return the resulting identity IDs; register them in `routes_workflow.go` under `auth.RequireInstanceAdmin`; verify handler tests for 403 on non-admin, 400 on each validation failure, 409 on duplicate and the identity IDs in the create and delete responses
- [ ] 2.2 Record `entity.override.create` and `entity.override.delete` with `RecordAuditFromContext` and document both in `docs/AUDIT.md`; verify a handler test finds one audit row per mutation with the override ID and members
- [ ] 2.3 Specify the three endpoints and their schemas in `docs/openapi.yaml`; verify the OpenAPI lint and the generated-client check used by CI pass

## 3. Web workflow and translations (PR 3)

- [ ] 3.1 Regenerate the web API client from `docs/openapi.yaml`; verify the frontend type check passes with the new hooks
- [ ] 3.2 Add admin-only detach and merge actions to `web/src/features/entities/EntityDetailPage.tsx`, with confirmation, the entity picker restricted to the same kind, and navigation to the returned identity; verify Vitest covers both flows and that non-admins see no action
- [ ] 3.3 Add the admin overrides list with state and remove action, linked from the entity detail page; verify Vitest covers listing, the dormant state and removal
- [ ] 3.4 Add all new strings to `web/src/i18n/en.ts` and `web/src/i18n/locales/pt-BR.ts`; verify the i18n catalog tests and `bun run lint` pass

## 4. Integration check

- [ ] 4.1 With `make dev` and two connectors sharing a hostname, detach one member, merge two unrelated members, then remove both overrides; verify the entity pages, redirects, topology graph and audit page match the spec scenarios
