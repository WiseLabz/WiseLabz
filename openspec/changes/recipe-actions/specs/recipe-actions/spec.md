# Spec Delta

## Purpose

Lets an administrator declare, in a custom connector's recipe, the requests that restart, start, stop or otherwise operate the documented service, so that operators can run them with the same preview, step-up and audit as built-in lab-mutating operations.

## ADDED Requirements

### Requirement: Action declaration
Format SHALL remain version 1. Actions MAY be declared under an endpoint's entity mapping (for its kind) or at the root (for the service). Names SHALL be 1–32 characters, start with a lowercase letter and contain only lowercase letters, digits, `_` or `-`. `restart`, `start` and `stop` SHALL be lifecycle verbs; other names SHALL be named actions. Each place SHALL hold at most 10 actions; one kind SHALL declare actions on at most one endpoint. Recipes without actions SHALL behave as before.

#### Scenario: Recipe without actions
- **WHEN** a recipe that declares no actions is saved and synced
- **THEN** it SHALL validate, sync and report capabilities exactly as it did before this change.

#### Scenario: Entity and service actions together
- **WHEN** a recipe declares `restart` and `rescan` under an endpoint whose entity kind is `container`, and `restart` at the root
- **THEN** the recipe SHALL be valid, containers SHALL support `restart` and `rescan`, and the service SHALL support `restart`.

#### Scenario: Invalid action name
- **WHEN** a recipe declares an action named `Re Scan`
- **THEN** validation SHALL fail with an error located at that action.

#### Scenario: Same kind on two endpoints
- **WHEN** two endpoints map the entity kind `container` and both declare actions
- **THEN** validation SHALL fail with an error located at the second declaration.

### Requirement: Action request
An action SHALL define a method of POST, PUT, PATCH or DELETE and a path relative to the connector's URL, and MAY define static query parameters, static headers and a static JSON body. No other method SHALL be accepted. An action request SHALL obey every same-origin, redirect and network-target rule that applies to recipe endpoint requests, and SHALL use the connector's configured authentication. An action SHALL NOT accept any value supplied by the operator at the time it is triggered.

#### Scenario: Method not allowed
- **WHEN** a recipe declares an action with method `GET`
- **THEN** validation SHALL fail with an error located at that action's method.

#### Scenario: Absolute path
- **WHEN** a recipe declares an action whose path is `https://other.example/api/restart`
- **THEN** validation SHALL fail with an error located at that action's path.

#### Scenario: Redirect to another origin
- **WHEN** an action is run and the service answers with a redirect
- **THEN** the redirect SHALL NOT be followed and the action SHALL fail.

#### Scenario: Extra request fields are ignored
- **WHEN** an action is triggered with a request that carries fields other than the entity reference
- **THEN** those fields SHALL have no effect on the request sent to the service.

### Requirement: Target placeholders
Entity actions MAY use `{external_id}` and `{attr.<name>}` in paths, query values and body string values. Values SHALL resolve from the target entity in the latest stored snapshot. Attributes SHALL be mapped by that endpoint; an unmapped attribute placeholder SHALL be a validation error. Service actions SHALL NOT use placeholders.

#### Scenario: Unknown entity
- **WHEN** an entity action is triggered with an entity reference that matches no entity in the latest snapshot
- **THEN** the action SHALL be rejected and no request SHALL be sent.

#### Scenario: Attribute missing on the entity
- **WHEN** an action uses `{attr.node}` and the target entity has no `node` attribute
- **THEN** the action SHALL fail before any request is sent, naming the missing attribute.

#### Scenario: Placeholder in a service action
- **WHEN** a root-level action's path contains `{external_id}`
- **THEN** validation SHALL fail with an error located at that action's path.

### Requirement: Path placeholder values
Path placeholder values SHALL be one escaped segment accepted under built-in entity-reference rules. Empty values, `.`, `..`, embedded `..`, separators, `?`, `#`, `%`, `;`, whitespace, control characters and non-ASCII SHALL fail before any request is sent.

#### Scenario: External ID in the path
- **WHEN** the action path is `/api/containers/{external_id}/restart` and the target entity's external ID is `db|1`
- **THEN** the request path SHALL be `/api/containers/db%7C1/restart`.

#### Scenario: Whitespace in a path value
- **WHEN** the target entity external ID is `web 1`
- **THEN** the action SHALL fail with a validation error naming the value and no request SHALL be sent.

#### Scenario: Value that would change the path
- **WHEN** the target entity's external ID is `../admin`
- **THEN** the action SHALL fail with a validation error and no request SHALL be sent.

### Requirement: Action metadata
An action MAY declare a label of at most 60 characters, a description of at most 300 characters and a downtime estimate between 0 and 3600 seconds. When no estimate is declared it SHALL default to 30 seconds for `restart` and 0 for every other action. Label and description SHALL be treated as plain text wherever they are shown.

#### Scenario: Defaults
- **WHEN** a recipe declares `restart` and `rescan` without metadata
- **THEN** their previews SHALL report 30 and 0 seconds of estimated downtime, and their buttons SHALL be labelled with the action names.

#### Scenario: Declared metadata
- **WHEN** an action declares label `Rescan library`, a description and a downtime of 5 seconds
- **THEN** the connector's action list and the action's preview SHALL report that label, description and estimate.

### Requirement: Per-connector capabilities
A custom connector SHALL support only lifecycle verbs and named actions its recipe declares. Each connector SHALL report its supported lifecycle and configuration operations and declared actions: name, entity/service target, entity kind when applicable, label, description and downtime. Other connector types' capabilities SHALL NOT change.

#### Scenario: Recipe without a restart action
- **WHEN** a restart is requested for a custom connector whose recipe declares no `restart` action
- **THEN** the request SHALL be rejected as an unsupported operation.

#### Scenario: Two custom connectors differ
- **WHEN** one custom connector's recipe declares `restart` and another's declares no actions
- **THEN** the first SHALL report restart as supported and the second SHALL NOT.

#### Scenario: Built-in connector unchanged
- **WHEN** capabilities are read for a connector of a built-in type
- **THEN** they SHALL be the same as before this change.

### Requirement: Enabling actions on save
API creation with actions, or saving changed actions to a non-empty set, SHALL require an instance admin and valid recipe-action-change elevation (bound to the existing connector). Comparison SHALL be semantic: comments, whitespace and key order SHALL NOT require elevation. Removing all actions SHALL NOT require elevation. Missing elevation SHALL reject the save without changing the stored recipe. With instance step-up disabled the elevation check SHALL pass as for other elevated actions.

#### Scenario: Adding an action without elevation
- **WHEN** an instance admin saves a recipe that adds an action and sends no elevation token
- **THEN** the save SHALL be rejected with the elevation-required error and the stored recipe SHALL be unchanged.

#### Scenario: Adding an action with elevation
- **WHEN** an instance admin saves the same recipe with a valid elevation token for that connector
- **THEN** the save SHALL succeed and an audit entry SHALL record that the connector's actions changed.

#### Scenario: Editing only the mapping
- **WHEN** an instance admin changes an attribute mapping and leaves the actions as they were
- **THEN** the save SHALL NOT require elevation.

#### Scenario: Reformatting the actions
- **WHEN** an instance admin reorders keys and adds comments inside the actions without changing any value
- **THEN** the save SHALL NOT require elevation.

#### Scenario: Token for another connector
- **WHEN** the elevation token was issued for a different connector
- **THEN** the save SHALL be rejected.

### Requirement: Actions in configuration files and backups
Config-file connectors with actions SHALL reconcile without elevation; creating or changing their actions SHALL be audited. Backup import SHALL validate every custom connector's recipe before writing anything and reject the whole bundle if any is invalid. Valid imported recipes SHALL retain actions, and the import audit SHALL name the connectors imported with actions.

#### Scenario: Declared connector with actions
- **WHEN** the configuration file declares a custom connector whose recipe has a `restart` action
- **THEN** it SHALL be reconciled, SHALL support restart, and an audit entry SHALL record that its actions were set.

#### Scenario: Unchanged declaration
- **WHEN** reconciliation runs again with the same configuration file
- **THEN** no further audit entry about actions SHALL be written.

#### Scenario: Backup with an invalid recipe
- **WHEN** a backup bundle contains a custom connector whose recipe fails validation
- **THEN** the import SHALL be rejected and no connector from the bundle SHALL be created.

#### Scenario: Backup with actions
- **WHEN** a backup bundle contains a custom connector with a valid recipe that declares actions
- **THEN** the connector SHALL be imported with its actions and the import's audit entry SHALL name it.

### Requirement: Action preview
Every recipe action, named or lifecycle, SHALL offer a dry-run without sending a request. It SHALL include the built-in lifecycle preview, a user-defined marker, resolved method, URL, static headers and body. Connector credentials and secret headers SHALL NOT be shown; query credentials SHALL be redacted. An unsupported operation's dry-run SHALL be rejected for every connector type.

#### Scenario: Preview shows the request
- **WHEN** an operator requests the dry-run of `restart` on container `web1` of a custom connector
- **THEN** the preview SHALL show the resolved method, URL and body, mark the action as user-defined, and no request SHALL reach the service.

#### Scenario: Token in the query string
- **WHEN** the connector authenticates with a query parameter
- **THEN** the URL in the preview SHALL NOT contain the token's value.

#### Scenario: Unsupported verb on a built-in connector
- **WHEN** a dry-run of `stop` is requested for a connector whose type does not support stop
- **THEN** the dry-run SHALL be rejected as an unsupported operation.

### Requirement: Running a lifecycle action from a recipe
A recipe-declared `restart`, `start` or `stop` SHALL run through the same operation as on a built-in connector: the same role requirement, the same elevation action, the same audit action, the same failure alert, and the same availability in bulk restart and in runbook lifecycle steps.

#### Scenario: Restart with elevation
- **WHEN** an operator with a valid elevation token for restart requests a restart of a custom connector whose recipe declares it
- **THEN** the declared request SHALL be sent once and an audit entry for the restart SHALL be recorded.

#### Scenario: Restart without elevation
- **WHEN** the same request carries no elevation token
- **THEN** it SHALL be rejected and no request SHALL be sent to the service.

#### Scenario: Lifecycle step in a runbook
- **WHEN** a runbook lifecycle step targets a custom connector whose recipe declares that verb
- **THEN** the step SHALL be accepted when the runbook is saved and SHALL send the declared request when the run reaches it.

### Requirement: Running a named action
The system SHALL let a user with an operator grant on the connector run a named action the connector's recipe declares, on the service or on one entity. Running it SHALL require a valid elevation token for the named-action operation bound to that connector and that action name. A token bound to another connector or another action name SHALL NOT be accepted. An undeclared name SHALL be rejected as an unsupported operation. No sync SHALL be triggered by an action.

#### Scenario: Rescan with a matching token
- **WHEN** an operator runs `rescan` with an elevation token bound to that connector and `rescan`
- **THEN** the declared request SHALL be sent once.

#### Scenario: Token for another action
- **WHEN** an operator runs `purge` with a token that was issued for `rescan` on the same connector
- **THEN** the request SHALL be rejected and nothing SHALL be sent to the service.

#### Scenario: Viewer
- **WHEN** a user with only a viewer grant on the connector runs a named action
- **THEN** the request SHALL be rejected.

#### Scenario: Undeclared action
- **WHEN** an operator runs `rescan` on a connector whose recipe does not declare it
- **THEN** the request SHALL be rejected as an unsupported operation.

### Requirement: Action result
Any 2xx response SHALL succeed regardless of body or JSON format. Other statuses, transport errors and oversized responses SHALL fail with the usual connector error classes and lifecycle failure alert. Success and non-2xx operator responses SHALL include status and at most 512 bytes of plain-text body; non-text bodies SHALL be omitted. Excerpts SHALL NOT be stored in audit, alerts, logs or run records.

#### Scenario: Success with a non-JSON body
- **WHEN** the service answers 200 with the text `OK`
- **THEN** the action SHALL succeed and the operator SHALL be shown status 200 and `OK`.

#### Scenario: Error message in a failing response
- **WHEN** the service answers 409 with a JSON error message
- **THEN** the action SHALL fail and the operator SHALL be shown status 409 and the start of that message.

#### Scenario: Long body
- **WHEN** the service answers with a 5 KiB body
- **THEN** at most 512 bytes of it SHALL be shown.

### Requirement: Action audit
Each action that was sent and answered with a 2xx status SHALL be recorded in the audit log with the acting user, the connector, the action name, the entity reference when there is one, the method, the request URL with credentials and query string removed, and the response status. A named action SHALL use one audit action distinct from the lifecycle audit actions; a recipe-declared lifecycle verb SHALL keep the existing lifecycle audit action with the method, URL and status added.

#### Scenario: Named action audited
- **WHEN** `rescan` succeeds on entity `lib1`
- **THEN** the audit log SHALL contain one entry naming the user, the connector, `rescan`, `lib1`, the method, the URL without credentials and the status.

#### Scenario: Nothing from the body
- **WHEN** an action's response body contains a secret
- **THEN** no audit entry, alert or log line SHALL contain any part of that body.

### Requirement: Recipe test preview leaves actions alone
Testing a recipe SHALL validate its actions and SHALL NOT send any action request.

#### Scenario: Preview of a recipe with actions
- **WHEN** an administrator tests a recipe that declares actions
- **THEN** only the recipe's endpoints SHALL be requested, and an invalid action SHALL be reported as a validation error.

### Requirement: Action controls in the web app
Custom connector controls SHALL offer only declared lifecycle verbs and each named action, on the service or entities of its declaring kind. Controls SHALL show dry-run request, label and description before step-up, then status and body excerpt. Saving changed actions SHALL ask for step-up and complete the save afterwards. New text SHALL be available in English and Brazilian Portuguese.

#### Scenario: Only declared controls
- **WHEN** a custom connector's recipe declares `restart` and `rescan` for the service
- **THEN** its page SHALL offer Restart and the rescan control, and SHALL NOT offer Start or Stop.

#### Scenario: Confirming an action
- **WHEN** an operator clicks a named action
- **THEN** the dry-run with the exact request SHALL be shown before step-up is requested.

#### Scenario: Saving changed actions
- **WHEN** an instance admin saves a recipe with a changed action in the web app
- **THEN** the app SHALL ask for step-up and, once given, SHALL save the connector.

### Requirement: Documented action format
The recipe format reference SHALL document actions, placeholders, metadata and the rules for enabling them, with at least one example recipe that declares an entity action and a service action. The example SHALL be verified by an automated test that loads the documented text.

#### Scenario: Example stays valid
- **WHEN** the documented action example is changed so that it no longer validates
- **THEN** an automated test SHALL fail.
