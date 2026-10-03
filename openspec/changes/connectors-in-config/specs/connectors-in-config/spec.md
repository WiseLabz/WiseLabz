# Spec Delta

## Purpose

Lets operators declare connectors in the config file so a lab's integrations can be kept in Git, reviewed and reproduced, with the server reconciling the declared list against its database at startup.

## ADDED Requirements

### Requirement: Declaring connectors in the config file
The config file SHALL accept a `connectors` list. Each entry SHALL have a `name`, a `type` and a `url`, and MAY have `verify_tls`, `enabled` (default true), `schedule_seconds`, a type-specific `config` map and a `grants` list of `user` (username) and `role` (`viewer` or `operator`). The connector category SHALL be derived from the type and SHALL NOT be declared.

#### Scenario: Minimal entry
- **WHEN** the config declares a connector with only `name`, `type`, `url` and the type's required `config` fields
- **THEN** after startup a connector with that name exists, enabled, with no sync schedule

#### Scenario: Scheduled entry
- **WHEN** an entry sets `schedule_seconds: 900`
- **THEN** the connector is picked up by the next scheduled sync poll without any further action

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
