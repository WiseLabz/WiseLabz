# Tasks

## 1. Config

- [x] 1.1 Add `Connectors []ConnectorEntry` (name, type, url, verify_tls, enabled, schedule_seconds, config, grants) to `Config`; `Load()` tests parse a YAML list and default `enabled` to true.
- [x] 1.2 Add the connectors-only secret pass (`${VAR}` and `<field>_file`) that records failures on the entry instead of failing `Load()`; tests cover env, file, unset variable, missing file and a `$` value outside `connectors` staying literal.
- [x] 1.3 Teach `schema.go` the free-form `config` map and extend `Redacted()` to mask connector secret fields; `TestSchemaMatchesConfig` and a redaction leak test pass.

## 2. Validation and CLI

- [x] 2.1 Add `connector.ValidateDeclared` (unknown type, missing required fields, unknown keys, then the existing field checks); table tests against a registered type.
- [x] 2.2 Make `server config validate` check declared entries (including duplicate names and unresolved secrets) with `internal/connector/all` imported explicitly; `config_cmd_test.go` asserts exit codes and that the entry name is reported.
- [x] 2.3 Mask connector secrets in `server config print --redacted`; the existing secret-leak test covers a declared connector.

## 3. Storage

- [x] 3.1 Add migration `000053_connector_managed_by` in both trees (`connectors.managed_by`, `config` grant source, with the SQLite table rebuild); migration parity and column parity tests pass, and up/down round-trips.
- [x] 3.2 Carry `managed_by` through `ConnectorRecord`, `connectorColumns`, the INSERT, all three scan sites and `UpdateConnector`; add `ListConnectorsByName`; store tests cover both.
- [x] 3.3 Add a config-sourced grant sync modelled on `SyncOIDCConnectorGrants` that leaves manual and oidc grants alone; store tests cover add, remove and untouched sources.

## 4. Reconciler

- [x] 4.1 Add `internal/connector/reconcile` with create, adopt, update and orphan in per-entry transactions; tests on a migrated SQLite store cover create, adopt keeping ID and history, ambiguous name, and re-adopting an orphan.
- [x] 4.2 Compute drift on declared fields only and move `secret_rotated_at` only on a real secret change; tests cover an unchanged restart, a changed secret, preserved undeclared keys and a credential-refresher type.
- [x] 4.3 Apply admin operator grants on create/adopt and reconcile declared grants; tests cover admin visibility, a removed grant and an unknown user being skipped.
- [x] 4.4 Skip invalid entries without orphaning their connector, write audit rows and notify admins for skipped and orphaned entries; tests cover each.
- [x] 4.5 Call the reconciler from `cmd/server/main.go` after `s.Init` and before `lifecycle.Start()`; verify by starting the server with a declared connector and seeing it listed.

## 5. API

- [x] 5.1 Expose `managedBy` on connector responses and add it to `docs/openapi.yaml`; `AssertMatchesSpec` passes.
- [x] 5.2 Reject Update, ToggleEnabled, Delete and their bulk forms with 409 for config-managed connectors while leaving operational actions open; handler tests cover each guarded route and an allowed sync.
- [x] 5.3 Allow only delete and the new release action on orphaned connectors; add `POST /connectors/{id}/release` with audit; handler tests and `TestOpenAPIMatchesRouter` pass.

## 6. Web

- [x] 6.1 Regenerate the client and show a "Managed by config" / "Orphaned" tag on the services list and detail page, with `en` and `pt-BR` strings; vitest covers the tag.
- [x] 6.2 Hide edit, enable toggle, schedule, delete and bulk selection for config-managed connectors, and show delete plus release for orphaned ones; vitest covers `ServiceDetailPage` and `ConnectorEditPage` in each state.

## 7. Documentation

- [x] 7.1 Fix the README claim about the example config and the config search path; add a `connectors:` example to `deploy/config.example.yaml`, fix its stale keys, and verify the example passes `server config validate`.
- [x] 7.2 Document declared connectors, adoption, the orphaned state and secret sources in `docs/ARCHITECTURE.md` and the connector guide, including the compose mount at `/etc/wiselabz/config.yaml`.

## 8. Delivery

- [x] 8.1 Run the full Go and frontend checks with the low-memory settings and `openspec validate connectors-in-config --strict`.
- [x] 8.2 Refresh graphify output, open the PR with `Closes #500`, and file the two follow-up issues (connector form `verifyTls`/`toggle` mismatch; OIDC role mappings by connector name).
