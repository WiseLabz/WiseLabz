# 0008 — Recipe-defined actions

Status: accepted (#653)

## Context

ADR 0001 and ADR 0002 set the permission, elevation, audit and dry-run boundary
for lab-mutating connector operations. Some custom integrations need a fixed
vendor request that cannot be represented by `restart`, `start` or `stop`.
Recipes already describe fetch requests as plain configuration, so this ADR
defines how a recipe may also describe bounded service-level or entity-level
actions without creating a general-purpose request interface.

## Decision

### 1. A recipe declares fixed actions as data

A recipe may declare named actions at the service root or under one entity
kind. Each action fixes its HTTP method, relative path, query, headers, and
optional static JSON body. Entity placeholders resolve from the latest stored
snapshot. Path placeholders use the existing strict `connector.PathSegment`
rule; query values and JSON bodies use their normal encodings. The request is
checked against the connector origin, and the existing guarded HTTP client
continues to reject unsafe destinations and redirects.

There are no caller-supplied parameters. The action preview resolves the
concrete method, redacted URL, headers, body and metadata from the stored
recipe and snapshot without making a request. Execution sends that same
resolved request once, so the approved request cannot diverge from the sent
request. Once a status line has arrived, the status alone decides: any 2xx is
success and any other status is a failure, whatever happens to the body. Only a
bounded prefix of the body is read, for the text excerpt returned to the caller. The excerpt is not
written to audit, alerts, logs or run history.

### 2. Keep the ADR 0001/0002 authorization boundary

Direct lifecycle routes retain their existing operator grant and elevation
actions: `connector.restart`, `connector.start` and `connector.stop`.
Running a named action requires an operator grant on the connector and
elevation for `connector.action`, targeted to `<connector id>:<action name>`.
The named-action route uses the same preview-before-confirmation pattern as
the lifecycle routes.

Changing recipe actions remains an instance-admin-only connector configuration
change. When the canonical action set changes and the submitted set is
non-empty, the save also requires elevation for `connector.recipeActions`,
targeted to the connector ID (empty target on create). Clearing the last
action is allowed without that additional step-up, while the existing
instance-admin requirement still applies. The same elevation action is
accepted by the password/session, WebAuthn and OIDC elevation mechanisms.

When instance step-up is disabled, saving and running actions pass the
elevation check as other elevated operations do. Saving still requires an
instance admin, and running still requires an operator grant on the connector.

### 3. Audit changes without recording request data

When a saved recipe action set changes, the system records
`connector.recipe_actions_changed` with qualified names in its `added`,
`changed` and `removed` lists. Service actions use `service.<name>` and entity
actions use `entity.<kind>.<name>`, keeping scopes distinct. Reconciliation
writes the same event with the system actor when a declared connector's
actions are created or changed; unchanged declarations add no such row.

Successful direct named actions use the existing `connector.action` audit
boundary, and lifecycle verbs keep their existing audit actions. A successful
API backup import records `backup.import` with import counts and the names of
newly imported connectors that declare actions. Audit detail never includes
the recipe body or the concrete action request body.

### 4. Treat declared configuration and backups as trusted

Recipes in the instance configuration file are applied without elevation and
their action changes are audited during reconciliation. Backup import is
instance-admin-only; it validates every custom recipe before writing any
records, then restores valid actions without an additional action-specific
step-up. Recipe credentials are redacted from exports, so import validation
checks the recipe, legacy headers and top-level URL but does not require the
redacted credential fields. The restored connector remains unusable for
authenticated requests until an administrator supplies those credentials.

## Non-goals

- Caller-supplied action parameters.
- Inspecting a response body to decide whether an action succeeded.
- Automatic sync, wait or post-action verification; runbooks can express
  follow-up steps separately.
- Configuration push through a recipe action.
- Exposing recipe actions over MCP.
- Holding actions from a trusted config file or backup inert until a separate
  approval step.
