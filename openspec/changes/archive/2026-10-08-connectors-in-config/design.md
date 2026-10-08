# Design

## Context

See proposal.md for motivation. Constraints found in the code:

- `config.Load()` (viper) has no interpolation or `_FILE` support and is also called by `cmd/migrate`, `cmd/backup` and the Docker healthcheck. The only slice in `Config` is `Auth.OIDC`, which is file-only.
- `config/schema.go` reflects over `Config` and rejects `map[string]any`; `Redacted()` is a hand-maintained list.
- Connector names are not unique (no constraint, no index) and operators can rename via `PUT`. Identity everywhere is the UUID, referenced by cascading FKs and by un-keyed JSON (API keys, report definitions, OIDC role mappings).
- `connector.ValidateConfig` checks only present string fields; it does not enforce `Required` or reject unknown keys, and the API wrapper accepts unknown types.
- Secrets are encrypted per field with a random nonce, so stored ciphertext cannot be compared; `store.SecretFieldsChanged` decrypts and compares and drives `secret_rotated_at`. `url` and `verify_tls` are columns, not `config_data` keys.
- Connector visibility is grant-only: a row with no `user_connector_roles` entry is invisible to everyone, admins included. `user_connector_roles.source` has `CHECK(source IN ('manual','oidc'))`.
- Sync scheduling is polled from row state (`ListDueConnectors`), and credential-refresher types rewrite `config_data` at runtime.
- With `ha.leader_election`, every replica runs `main` up to `lifecycle.Start()`.

## Goals / Non-Goals

**Goals:**
- Reconcile is safe to run on every start and on every replica.
- A mistake in the file never destroys data and never takes the app down.
- UI-only installs see no behaviour change.

**Non-Goals:**
- Global `${VAR}` interpolation for the rest of the config file.
- A unique constraint on connector names.
- Live reload of `connectors:` without a restart.

## Decisions

**Entry shape and loading.** `Connectors []ConnectorEntry` on `Config`, following the OIDC precedent (file-only, no `BindEnv`). `config` is `map[string]any`; the schema generator and `TestSchemaMatchesConfig` learn a free-form object for it. *Alternative:* typed per-connector structs, rejected because the 16 types already describe themselves through the registry.

**Secret resolution is a connectors-only pass that never runs in `Load()`.** `Config.Connectors` keeps values exactly as written. `Config.ResolveConnectors()` returns each entry with `${VAR}` expanded and `<field>_file` (plus `url_file`) substituted, carrying a per-entry error instead of failing. Only the reconciler and `server config validate` call it, because `Load()` is shared with migrate, backup and the healthcheck, and the agreed behaviour is skip-and-start. A side benefit: `config print --redacted` shows `${VAR}` references rather than resolved values. *Alternative:* viper-level or global expansion, rejected because existing DSNs and passwords may contain `$`.

**Redaction without the registry.** The config package cannot import connector types, so `Redacted()` masks every literal string under an entry's `config` and keeps `${VAR}` references and `_file` paths. This over-masks non-secret fields such as usernames, which is the safe direction.

**Strict validation lives next to the registry.** A new `connector.ValidateDeclared(type, url, config)` reports unknown type, missing `Required` fields and unknown keys, then delegates to the existing `ValidateConfig` for length, pattern and options. The lenient API path is left alone to avoid changing UI behaviour. `config` cannot import `connector` cleanly from `Validate()`, so `server config validate` calls it from `cmd/server`, which imports `internal/connector/all` explicitly instead of relying on the `api` import.

**`managed_by` and `config_hash` columns.** Migration `000053` adds `connectors.managed_by TEXT NOT NULL DEFAULT 'ui'` checked against `ui`, `config`, `config-orphaned`, adds `connectors.config_hash`, and widens the grant `source` check to include `config`. SQLite needs the `_new` + rename rebuild for `user_connector_roles` (as in `000041`); Postgres drops and re-adds the constraint. *Alternative:* releasing removed connectors straight to `ui`, rejected by the user in favour of a visible orphaned state.

**Matching by name, in this order:** rows with `managed_by` in (`config`, `config-orphaned`) and that name; else UI rows with that name. Exactly one candidate proceeds; more than one is an invalid entry. Rename is blocked for managed rows, so the match is stable after adoption.

**Reconciler package `internal/connector/reconcile`.** `Run` returns one result per entry (created, adopted, updated, unchanged, skipped with its reason) followed by one per orphaned connector. Each entry is applied in its own transaction so one bad entry cannot roll back the others.

**Drift is detected with a fingerprint, not by comparing live values.** `config_hash` stores an HMAC-SHA256 (keyed with the encryption key, since it covers secrets) of the entry's type, URL, flags, schedule and config as last applied. A config-managed row whose hash matches is left untouched. *Alternative considered:* decrypting and comparing the stored config on every start. Rejected because credential-refresher types rewrite declared secrets at runtime, so every restart would see drift and overwrite a live token with the stale declared one. When the entry does change, declared fields are applied; undeclared keys are kept only for credential-refresher types (their self-obtained tokens live beside the declared fields) and dropped for every other type, so removing a key from the file removes it from the connector. `SecretFieldsChanged` decides whether `secret_rotated_at` moves. Orphaning considers only names absent from the whole declared list, valid or not, so an entry that became invalid does not orphan its connector.

**Startup placement.** In `cmd/server/main.go` after `s.Init` (the first admin exists) and before `lifecycle.Start()`. All replicas run it; idempotent per-entry transactions make the race benign (the loser sees no drift). Audit rows carry `actorRole` `system` and no actor user. Skipped and orphaned entries are summarised in one notification sent to instance admins only (a new `Dispatcher.NotifyAdmins`), reusing the existing `system.job_failed` event type so no WebSocket contract or routing change is needed.

**Grants.** Admin operator grants are written once, on create or adopt, with source `manual`, mirroring what the API does for the creating user; they are not re-applied on later starts, so an admin's access can still be revoked in the UI. Declared `grants` use source `config` and are diffed like `SyncOIDCConnectorGrants`. Users are matched by username.

**API guard.** A `RequireManagedBy` middleware on the connector routes returns 409 `connector_managed` for `config` rows on Update, ToggleEnabled and Delete, and 409 `connector_orphaned` for everything except Delete and the new `POST /connectors/{id}/release` on `config-orphaned` rows. Operational routes stay open for `config` rows. The bulk routes are all operational (sync, reauth, restart); they report an orphaned ID as an `orphaned` item error.

**Owner and rotation fields.** `owner`, `user_expires_at` and `rotation_max_age_days` are declared per entry, validated with the API's rules (RFC3339, positive days), written on create and update, and included in the fingerprint. Empty/zero clears the column. YAML reads an unquoted timestamp as a time value, so `Load()` turns it back into an RFC3339 string before decoding.

**OIDC role mappings by name.** `syncOIDCConnectorGrants` resolves any key that is neither `*` nor a UUID against connector names at login (case-insensitive, since viper lowercases map keys). A name matching zero or several connectors is skipped with a warning; config validation only rejects empty keys.

## Risks / Trade-offs

- [Adoption silently overwrites a UI connector's settings] → logged, audited as `connector.adopt` with the changed field names, and documented.
- [Two replicas reconcile at once] → per-entry transactions and idempotent comparison; the name lookup and write share a transaction.
- [Credential-refresher types rewrite a declared secret at runtime] → the fingerprint compares the entry with what was last applied, not with the live value; covered by a test.
- [A missing or unmounted config file reads as an empty list and orphans every managed connector] → orphaning only disables, restoring the file adopts them back, and the behaviour is documented.
- [Owner and rotation settings of a config-managed connector cannot be edited in the UI, since the whole update route is locked] → the entry declares them (`owner`, `user_expires_at`, `rotation_max_age_days`, validated like the API and part of the fingerprint), so they are managed from the file.
- [A skipped entry goes unnoticed] → error log, admin notification, and `server config validate` for CI.
- [Stricter validation rejects entries the UI would accept] → intended; it applies only to declared connectors.

## Migration Plan

Additive migration; existing rows default to `ui` and behave as before. Rollback: the down migration drops the column and restores the grant source check after deleting `config`-sourced grants. Removing the `connectors:` key after use orphans the managed connectors on the next start, which is the documented exit path.

## Open Questions

None.
