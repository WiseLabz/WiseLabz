# connectors-in-config Specification

## Purpose
Lets operators declare connectors in the config file so a lab's integrations can be kept in Git, reviewed and reproduced, with the server reconciling the declared list against its database at startup.

## Requirements

### Requirement: Declaring connectors in the config file
The config file SHALL accept a `connectors` list. Each entry SHALL have a `name` and a `type`. An entry SHALL require `url` only when its connector type schema requires a top-level URL; connector types without a top-level URL (such as `tlsprobe`) SHALL NOT accept `url`. Each entry MAY have `verify_tls`, `enabled` (default true), `schedule_seconds`, `owner`, `user_expires_at` (RFC3339), `rotation_max_age_days` (positive integer), a type-specific `config` map and a `grants` list of `user` (username) and `role` (`viewer` or `operator`). The connector category SHALL be derived from the type and SHALL NOT be declared.

#### Scenario: Ownership and rotation settings
- **WHEN** an entry sets `owner`, `user_expires_at` and `rotation_max_age_days`
- **THEN** the connector is created or updated with those values, a later change to any of them is applied on the next start, and removing them clears the stored values
- **AND** a `user_expires_at` that is not RFC3339, or a non-positive `rotation_max_age_days`, makes the entry invalid, exactly as the API rejects the same values

#### Scenario: Minimal entry
- **WHEN** the config declares a connector with only `name`, `type`, `url` and the type's required `config` fields
- **THEN** after startup a connector with that name exists, enabled, with no sync schedule

#### Scenario: Scheduled entry
- **WHEN** an entry sets `schedule_seconds: 900`
- **THEN** the connector is picked up by the next scheduled sync poll without any further action

#### Scenario: URL-less connector entry
- **WHEN** the config declares a `tlsprobe` connector with no `url`
- **THEN** the entry is accepted and loads with an empty URL

#### Scenario: URL-less connector with URL rejected
- **WHEN** the config declares a `tlsprobe` connector with a `url`
- **THEN** the entry is invalid and rejected with an error stating that tlsprobe does not accept a url

### Requirement: Secret sources for declared connectors
Within the `connectors` list, a string value SHALL have `${VAR}` references replaced from the environment, and any field `<field>` MAY instead be given as `<field>_file` naming a file whose trimmed content becomes the value. Values outside the `connectors` list SHALL NOT be interpolated. Declared secret fields SHALL be stored encrypted like secrets entered in the UI, and `server config print --redacted` SHALL mask them.

#### Scenario: Secret from environment
- **WHEN** an entry sets `token: ${PVE_TOKEN}` and `PVE_TOKEN` is set
- **THEN** the connector is stored with the value of `PVE_TOKEN` as its token

#### Scenario: Secret from file
- **WHEN** an entry sets `token_file: /run/secrets/pve` and the file exists
- **THEN** the connector is stored with the file's trimmed content as its token

#### Scenario: Other config untouched
- **WHEN** a value outside `connectors` contains `${NAME}`
- **THEN** it is used literally, as before

#### Scenario: Redacted print
- **WHEN** `server config print --redacted` runs on a config with declared connector secrets
- **THEN** no secret value appears in the output

### Requirement: Startup reconciliation by name
At startup, after migrations and before scheduled work begins, the server SHALL reconcile each declared entry by `name`. It SHALL use the config-managed or orphaned connector with that name; otherwise it SHALL adopt the single UI-managed connector with that name, keeping its ID and history; otherwise it SHALL create a connector. A reconciled connector SHALL be marked as managed by config and its declared fields SHALL match the entry. Reconciliation SHALL be idempotent: a second start with the same config SHALL change nothing, and a secret's rotation timestamp SHALL move only when the declared secret value actually changed.

#### Scenario: Create
- **WHEN** no connector named `pve` exists and the config declares one
- **THEN** a connector named `pve` exists after startup with `managedBy` set to `config`

#### Scenario: Adopt
- **WHEN** a UI-created connector named `pve` exists with sync history and the config declares `pve`
- **THEN** the same connector ID is kept with its history, its fields match the entry and `managedBy` is `config`

#### Scenario: Ambiguous name
- **WHEN** two UI-created connectors are named `pve` and the config declares `pve`
- **THEN** neither is changed and the entry is reported as invalid

#### Scenario: Unchanged restart
- **WHEN** the server restarts with an unchanged config
- **THEN** no connector is modified and no secret rotation timestamp changes

### Requirement: Access to reconciled connectors
When a connector is created or adopted from config, every instance admin SHALL be granted the operator role on it. Declared `grants` SHALL be applied under a dedicated `config` grant source: grants no longer declared SHALL be removed, and manual and SSO grants SHALL NOT be changed. A grant naming an unknown user SHALL be skipped with a warning and applied on a later start once the user exists.

#### Scenario: Admin sees a new connector
- **WHEN** a connector is created from config
- **THEN** an instance admin sees it in the connector list

#### Scenario: Declared grant removed
- **WHEN** a grant for `alice` is removed from an entry and the server restarts
- **THEN** alice's config-sourced grant is gone and any manual grant she holds remains

### Requirement: Config-managed connectors are locked
For a connector managed by config, the API SHALL reject edits, renames, enable/disable, schedule changes and deletion with a conflict error, individually and in bulk actions. Operational actions that do not change its declared fields (sync, connection test, health check and similar) SHALL remain available to operators. Connector responses SHALL include `managedBy`, and the UI SHALL show a "Managed by config" indicator and not offer the locked controls.

#### Scenario: Edit rejected
- **WHEN** an operator updates a config-managed connector through the API
- **THEN** the response is 409 and the connector is unchanged

#### Scenario: Sync allowed
- **WHEN** an operator triggers a sync of a config-managed connector
- **THEN** the sync runs

### Requirement: Orphaned connectors
A config-managed connector whose entry is no longer declared SHALL be disabled and marked `config-orphaned` at the next startup, keeping its data and history, with an audit record and an admin notification. An orphaned connector SHALL only allow deletion or release; release SHALL return it to UI management, still disabled. Declaring its name again SHALL return it to config management.

#### Scenario: Entry removed
- **WHEN** the `pve` entry is removed and the server restarts
- **THEN** `pve` is disabled, `managedBy` is `config-orphaned` and its sync history remains

#### Scenario: Release
- **WHEN** an operator releases an orphaned connector
- **THEN** `managedBy` is `ui`, it is still disabled and it can be edited normally

#### Scenario: Declared again
- **WHEN** the `pve` entry is restored and the server restarts
- **THEN** `pve` is managed by config again with the declared `enabled` value

### Requirement: Invalid entries do not block startup
An entry SHALL be invalid when its type is unknown, a required field is missing, its `config` holds a key the type does not define, a `${VAR}` or `_file` source cannot be resolved, its name is duplicated in the list or its name matches more than one existing connector. An invalid entry SHALL be skipped with an error log and an admin notification; the server, the valid entries and existing connectors SHALL be unaffected, and a connector matching an invalid entry SHALL NOT be orphaned.

#### Scenario: Unset variable
- **WHEN** an entry references `${MISSING}` and another entry is valid
- **THEN** the server starts, the valid connector is reconciled and the invalid entry is reported

#### Scenario: Previously reconciled connector
- **WHEN** an entry for an existing config-managed connector becomes invalid
- **THEN** that connector keeps its last reconciled state and is not orphaned

### Requirement: Validating declared connectors offline
`server config validate` SHALL check every declared entry against its connector type's schema without opening the database, report each problem with the entry's name, and exit non-zero when any entry is invalid.

#### Scenario: Unknown key
- **WHEN** an entry's `config` holds a key its type does not define
- **THEN** `server config validate` names the entry and the key and exits non-zero

#### Scenario: Valid config
- **WHEN** every entry is valid
- **THEN** `server config validate` prints its success message and exits zero

### Requirement: Connector names in OIDC group mappings
`auth.oidc[].group_connector_roles` keys SHALL accept a connector name as well as a UUID or `*`. Names SHALL be resolved at login, case-insensitively. A name matching no connector, or more than one, SHALL be skipped with a warning and SHALL NOT affect the other keys.

#### Scenario: Name key
- **WHEN** a group maps `Plex: viewer` and exactly one connector is named Plex
- **THEN** a user in that group is granted `viewer` on it at login

#### Scenario: Ambiguous or unknown name
- **WHEN** a key names no connector or two connectors
- **THEN** it is skipped with a warning and the remaining keys still apply

### Requirement: Import reference forms for TLS probe connectors
A declared `tlsprobe` entry SHALL accept `targets`, `import_port`, and an import reference in one of two mutually exclusive forms: `import_connector` with the name of another connector declared in the same config file, or `import_connector_id` with the UUID of an existing Traefik connector. The server SHALL reject entries that set both keys simultaneously. Reconcile SHALL verify that the referenced connector exists and is of type `traefik`, skipping the probe entry with an error if the target does not exist or is not a Traefik connector.

#### Scenario: Import by declared connector name
- **WHEN** a `tlsprobe` entry sets `import_connector: traefik` naming a declared Traefik connector
- **THEN** reconcile resolves the name to that connector's ID and stores it as `import_connector_id`

#### Scenario: Import by raw connector ID
- **WHEN** a `tlsprobe` entry sets `import_connector_id` with the UUID of an existing Traefik connector
- **THEN** reconcile verifies the connector exists and is a Traefik connector, storing the ID

#### Scenario: Mutually exclusive import keys
- **WHEN** a `tlsprobe` entry specifies both `import_connector` and `import_connector_id`
- **THEN** the entry is invalid and rejected

#### Scenario: Non-Traefik reference rejected
- **WHEN** a `tlsprobe` entry references a connector whose type is not `traefik`
- **THEN** reconcile skips the probe entry with an error stating it must be a Traefik connector

#### Scenario: Unknown referenced name
- **WHEN** a `tlsprobe` entry sets `import_connector` to a name not declared in the config file
- **THEN** reconcile skips the probe entry with an error

### Requirement: Reconciliation order and stability for dependent connectors
At startup reconciliation, the server SHALL reconcile any referenced Traefik connector before the TLS probe connector that names it, including on an initial run against an empty database. Resolving `import_connector` to the referenced connector's ID SHALL never store `import_connector` in the database; the stored configuration SHALL hold `import_connector_id`. Reconciliation SHALL be stable: an unchanged configuration SHALL make no update to the probe row and write no audit row on subsequent runs. Changing the named target in configuration SHALL update the stored `import_connector_id`. A probe naming itself or an entry that failed to reconcile SHALL fail without affecting other entries.

#### Scenario: Reconcile order on empty database
- **WHEN** a probe naming a Traefik connector by `import_connector` is declared before the Traefik entry in configuration on an empty database
- **THEN** the Traefik entry is reconciled first, and the probe is created with the Traefik connector's generated ID

#### Scenario: Stable reconciliation without audit rows
- **WHEN** the server restarts with an unchanged configuration declaring a TLS probe and its referenced Traefik connector
- **THEN** the probe is reported unchanged and no audit record is created

#### Scenario: Changing named target updates stored ID
- **WHEN** a probe's `import_connector` is changed from one declared Traefik connector to another
- **THEN** the probe is updated with the new connector's ID and an audit record is created

#### Scenario: Self-referencing probe rejected
- **WHEN** a probe sets `import_connector` to its own name
- **THEN** the probe is skipped with an error and other connectors reconcile normally
