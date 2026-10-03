# Connectors in config.yaml

Connectors can be declared in `config.yaml` instead of (or alongside) the web
UI, so a lab's integrations can live in Git. The server applies the list to its
database every time it starts.

```yaml
connectors:
  - name: pve
    type: proxmox
    url: https://pve.lan:8006/api2/json
    schedule_seconds: 900
    config:
      token_id: root@pam!wiselabz
      token_secret: ${PVE_TOKEN_SECRET}
    grants:
      - user: alice
        role: viewer
```

## Entry fields

| Key | Required | Meaning |
|---|---|---|
| `name` | yes | Connector name. This is how an entry is matched to a connector, so it must be unique in the list. |
| `type` | yes | Connector type, e.g. `proxmox`. `GET /api/connectors/schema` lists the types and their fields. |
| `url` | yes | Base URL. Use `url_file` instead to read it from a file. |
| `verify_tls` | no | Defaults to `true`. |
| `enabled` | no | Defaults to `true`. |
| `schedule_seconds` | no | Auto-sync interval. Omitted or `0` means manual sync only. |
| `config` | depends on type | The type's own fields (tokens, usernames and so on). |
| `grants` | no | Users (`user` is the username) and their `role`, `viewer` or `operator`. |

The category is taken from the type and is not declared.

## Secrets

Inside `connectors` only, two sources keep secrets out of the file:

- `${VAR}` is replaced with the environment variable `VAR`. An unset variable
  makes the entry invalid; a variable set to an empty string is allowed.
- `<field>_file: /path` reads the field's value from a file and trims
  surrounding whitespace, which fits Docker and Kubernetes secrets. For example
  `token_secret_file: /run/secrets/pve`.

Nothing else in `config.yaml` is interpolated, so existing values that contain
`$` keep their meaning. Secret fields are encrypted in the database exactly like
secrets entered in the UI. `server config print --redacted` masks every literal
string under `config` and keeps `${VAR}` references and `_file` paths, since
those name a source rather than hold a secret.

## What happens at startup

Each entry is matched by `name`:

1. A connector already managed by config (or orphaned from it) with that name is
   updated if the entry changed.
2. Otherwise a single UI-created connector with that name is **adopted**: it
   keeps its ID, history, docs and grants, and its settings are overwritten by
   the entry.
3. Otherwise a new connector is created.

If more than one connector could match, the entry is skipped; rename or delete
the extras.

A created or adopted connector is granted to every instance admin as operator,
once. Declared `grants` are reconciled on every start under their own `config`
source: removing one from the file removes that grant, and grants made in the UI
or through SSO are never touched. A grant for a username that does not exist yet
is skipped with a warning and applied on a later start.

An unchanged entry writes nothing. In particular a connector that refreshes its
own credentials (OAuth-style types) keeps the refreshed values across restarts;
they are only replaced when the entry itself changes.

### Invalid entries

An entry with an unknown type, a missing required field, an unknown `config`
key, an unresolved `${VAR}` or `_file`, or a duplicated name is **skipped**. The
server still starts, the other connectors are unaffected, the problem is logged
and instance admins get a notification. A connector whose entry became invalid
keeps its last applied state.

Check the file before deploying:

```sh
server config validate
```

It runs the same checks without touching the database and exits non-zero when
any entry is invalid, naming the entry and the field.

## Managed connectors in the UI

A connector declared in config shows a `config` tag. Its settings come from the
file, so editing, renaming, enabling or disabling, changing the schedule and
deleting are refused (`409 connector_managed`). Sync, connection test, health
check, restart/start/stop, maintenance windows and golden snapshots still work.

## Removing an entry

Removing an entry does not delete anything. At the next start the connector is
disabled and marked `orphaned`, an audit record is written and instance admins
are notified. From there an operator can either:

- **Release to UI**: it becomes a normal UI-managed connector, still disabled; or
- **Remove** it, which deletes the connector and its history as usual.

Putting the entry back adopts the connector again.

Note that a missing or unmounted `config.yaml` is read as an empty list, so
every config-managed connector is orphaned (disabled) until the file is back.

## Docker Compose

The server looks for `config.yaml` in `/etc/wiselabz/`, the working directory
and `./deploy/`. The provided compose files configure everything through
environment variables, so add a mount to use this feature:

```yaml
services:
  wiselabz:
    volumes:
      - ./config.yaml:/etc/wiselabz/config.yaml:ro
    environment:
      PVE_TOKEN_SECRET: ${PVE_TOKEN_SECRET}
```
