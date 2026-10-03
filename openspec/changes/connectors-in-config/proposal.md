# Proposal

## Why

Issue #500: connectors can only be created in the web UI, so operators who keep their lab configuration in Git cannot declare them, review them or rebuild an instance from a file. The README even claims `deploy/config.example.yaml` holds examples for every supported service, which it does not.

## What Changes

- New `connectors:` list in `config.yaml`. Each entry declares `name`, `type`, `url`, `verify_tls`, `enabled`, `schedule_seconds`, a type-specific `config` map and optional `grants`.
- Secrets inside `connectors:` can come from the environment (`${VAR}`) or from a file (`<field>_file`). No other part of the config file gains interpolation.
- At startup the server reconciles the list against the database by connector name: it creates missing connectors, adopts a UI-created connector of the same name (keeping its ID and history) and updates drifted ones. A connector whose entry was removed is disabled and marked `config-orphaned`.
- Connectors gain a `managedBy` field (`ui`, `config`, `config-orphaned`). Config-managed connectors cannot be edited, renamed, enabled/disabled, rescheduled or deleted through the API or UI; operational actions (sync, test, health and the like) keep working. An orphaned connector can only be deleted or released back to the UI.
- A created or adopted connector is granted to every instance admin as operator, and declared `grants` are reconciled under their own grant source so manual and SSO grants are untouched.
- An invalid entry (unknown type, missing required field, unknown key, unresolved secret, ambiguous name) is skipped with an error log and an admin notification; the server and the remaining connectors still start.
- `server config validate` checks connector entries against the connector type schemas and exits non-zero on the same problems. `server config print --redacted` masks connector secrets.
- Web: a "Managed by config" badge, locked edit controls for managed connectors, and delete/release actions for orphaned ones.
- Docs: correct the README, add a `connectors:` example to `deploy/config.example.yaml` and fix its stale keys.

## Capabilities

### New Capabilities

- `connectors-in-config`: declaring connectors in the config file, startup reconciliation, the managed and orphaned states, and validation of declared connectors.

### Modified Capabilities

None.

## Impact

`backend/internal/config` (struct, loader pass, validation, schema generator, redaction), `backend/internal/connector` (strict schema check, new reconciler), `backend/internal/store` (migration `000053` adding `connectors.managed_by` and a `config` grant source, connector and grant queries), `backend/cmd/server` (startup hook, `config validate`), `backend/internal/api/connectors` (guards, release action), `docs/openapi.yaml` and the regenerated web client, the connector pages in `web/src/features`, and README, `deploy/config.example.yaml`, `docs/ARCHITECTURE.md` and the connector guide. No new dependency.
