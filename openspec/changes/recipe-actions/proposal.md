# Proposal

## Why

Recipes for the custom REST connector (#513) are read-only: they fetch and map. A service documented through a recipe therefore cannot be restarted, started or stopped from WiseLabz, cannot take part in a runbook, and has no way to expose a service-specific operation such as "rescan" or "flush cache". Issue #653 asks for recipes to declare those requests.

A recipe-declared action turns a user-written HTTP call into a lab-mutating operation, so it needs its own decision against ADR 0001 and ADR 0002. The decisions below were made with the maintainer on 2026-10-08.

## What Changes

- A recipe MAY declare **actions**: per entity kind (under an endpoint's `entity`) and for the service as a whole (at the recipe root). The names `restart`, `start` and `stop` are the existing lifecycle verbs; any other name is a **named action**.
- An action is a fixed request: method POST, PUT, PATCH or DELETE, a relative same-origin path, optional static query, headers and JSON body. The only substitutions are the target entity's external ID and mapped attributes. Operators supply no values when they trigger one.
- An action MAY carry a label, a description and a downtime estimate, used by the button and the dry-run.
- Lifecycle support becomes **per connector instance**: a custom connector supports a verb only when its recipe declares it. Connector responses expose the instance's capabilities and its declared actions.
- Recipe-declared `restart`/`start`/`stop` run through the existing lifecycle endpoints, role gate, elevation actions, audit actions and runbook lifecycle steps.
- A new operation runs a named action: operator role, a dry-run, and an elevation bound to the connector and the action name.
- The dry-run for any recipe-declared action shows the exact request (method, URL, static headers, body) and marks it as user-defined. The dry-run of every connector now rejects a verb the connector does not support (it previously answered with a preview and failed only on the real call).
- A 2xx response is success. The operator is shown the status and at most 512 bytes of the response once; the audit entry stores the action, entity, method, redacted URL and status, never the body.
- **Turning actions on**: saving a recipe whose actions changed requires an instance admin (already required for any recipe change) and, new, an elevation token. Connectors declared in `config.yaml` are trusted and audited. A backup import validates custom recipes, keeps their actions and names the affected connectors in its audit entry.
- A new runbook step kind, `connector_action`, runs a named action inside a run. After an interruption, such a step is not sent again automatically: the operator chooses to send it again or to mark it done.
- ADR 0008 records the boundary; the recipe format reference documents actions with a tested example.

No existing recipe, connector or runbook changes behaviour. Recipes stay at format version 1; `actions` is optional.

## Capabilities

### New Capabilities
- `recipe-actions`: recipe-declared lifecycle and named actions on custom connectors: format and validation, how actions are enabled, per-instance capabilities, the dry-run, execution, results, audit, and treatment in configuration files and backups.

### Modified Capabilities
- `runbook-runs`: a seventh step kind (`connector_action`), its authoring and execution rules, and the resume rule for a step whose request may already have been sent.

## Impact

- Backend: `internal/connector` (per-instance capability check), `internal/connector/custom` (recipe parsing, action requests), `internal/api/connectors` (lifecycle preview and mutate, new action endpoint, save path), `internal/api/auth` and `internal/auth` (elevation action allowlist in three places), `internal/connector/reconcile`, `internal/backup`, `internal/runbookrun`, `internal/api/runbooks`, `internal/store` (migration `000066`, both drivers), `internal/mcp` (step kind listing).
- API: new `POST /api/connectors/{id}/actions/{name}`; connector responses gain capabilities and actions; lifecycle previews gain an optional request block; the run resume request gains a decision for an unknown action step; `docs/openapi.yaml` and the generated web client.
- Web: service detail page (buttons, dry-run and result dialogs), connector form (elevation on save), runbook step editor, en and pt-BR strings.
- Docs: `docs/adr/0008-recipe-defined-actions.md`, `docs/ARCHITECTURE.md`, `docs/connectors/RECIPE_FORMAT.md`, `docs/CONNECTORS_IN_CONFIG.md`.
- Security: this is the first connector save path that takes an elevation token, and the first user-defined mutating request. Same-origin, no-redirect and loopback/link-local protections of recipe requests apply unchanged.
- No new dependencies.
