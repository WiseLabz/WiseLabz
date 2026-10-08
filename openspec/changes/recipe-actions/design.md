# Design

## Context

See proposal.md for motivation. The decisions marked "(user)" were made with the maintainer on 2026-10-08 and are not open. Current state that shapes the approach (paths relative to `backend/internal` unless stated; line numbers are from `main` at `b27dfb1` and may have moved):

- Lifecycle support is three optional interfaces in `connector/connector.go` (`Restarter`, `Starter`, `Stopper`, about lines 43-57). `connector.LifecycleOp` (about line 66) and `connector.Capabilities` (about line 136) decide support by type assertion. `registry.go` `ListSchemas` (about lines 395-404) builds the per-type capabilities by calling the factory with `{}`, and `SupportsLifecycleVerb` (about line 384) is likewise type-level with an empty config. Capabilities are served only by `GET /connectors/schema`; connector instance responses carry none, and the web app's Start/Stop/Restart buttons do not consult them.
- The lifecycle routes are in `api/routes_connectors.go` (about lines 68-78): `RequireConnectorRole(..., "operator", "id")` and `RequireManagedBy(ui, config)`. Elevation is checked inside the handler: `api/connectors/lifecycle.go` `lifecycleOpMutate` (about line 217) calls `prepareLifecycleOp` (verb support, 400 `unsupported_operation`), then `auth.ValidateElevationHeader(..., "connector."+verb)`, then `mutateLifecycleOp`, which validates `entityRef` with `connector.ValidateCompositeRef` and applies. Success writes the audit row (`recordLifecycleAudit`, detail `extraAudit` plus `entityRef`); failure raises an alert and a broadcast. `PreviewLifecycleOp` (about lines 133-195) builds the dry-run from the latest stored snapshot and checks neither verb support nor `entityRef`.
- An elevation token binds an exact action string, the session and an optional target (`auth/jwt.go` about lines 77-93, 333, 353). `/auth/elevate` accepts a `target`. The action string must be in an allowlist that exists three times: `api/auth/session.go` `validElevationAction` (about lines 293-303), `auth/webauthn.go` (about line 265) and `auth/oidc_elevate.go` (about line 63). When instance step-up is disabled the check passes (`auth/middleware.go` about lines 402-404).
- Connector saves: `POST /connectors/` is instance-admin only; `PUT /connectors/{id}` needs an operator grant and a UI-managed connector, and `authorizeConnectorRepoint` (`api/connectors/handlers.go` about lines 372-420) restricts changes to a type's `EndpointConfigKeys` to instance admins. For `custom` those keys are `url` and `recipe` (`connector/custom/custom.go` about line 25). No connector save takes an elevation token today.
- Recipe code is in `connector/custom`: `recipe.go` (types, strict YAML decode, validation with dotted locations), `recipe_fetch.go` (`buildRecipeRequest`/`buildRecipeRequestAt` with the same-origin check, auth application, `recipeHTTPClient` with no redirects on top of the guarded dialer, `sanitizeRecipeError`), `recipe_map.go` (`expandRecipeTemplate` for `{gjson path}` attribute templates). `fetchRecipePage` requires a JSON body. Endpoint paths are static today. The custom factory does not keep the recipe; it is parsed from `config["recipe"]` on each use.
- `connector/ref.go` has `ValidateRefSegment`, `ValidateCompositeRef` and `PathSegment` (validate, then `url.PathEscape`). Built-in connectors use them; `custom` does not yet.
- Declared connectors go through `config/connectors.go` and `connector/reconcile/reconcile.go`; the recipe is validated by `validateCustomConfig`. Backup import (`backup/backup.go` `importConnectors`, about lines 729-745) inserts connectors without any config validation; `ValidateBundle` checks only categories and runbooks. The import route is instance-admin only with no elevation.
- Runbook step kinds are a closed set: constants in `runbookrun/executor.go` (about lines 46-52), validation in `api/runbooks/handlers.go` (`validKind`, `validateStepTarget`), dispatch in `runbookrun/run.go` `perform` (about lines 248-300), CHECK constraints on `runbook_steps.kind` and `runbook_run_steps.kind` (migration `000065`, both drivers) and on `runbook_steps.verb` (`restart|start|stop`, Postgres `000045`, also present on sqlite). A run freezes its steps at start. A step left `running` at a backend restart becomes `unknown`; resume repeats the first step that is not `succeeded` (ADR 0006).
- No MCP tool performs a lifecycle operation; `mcp/runbooks.go` (about line 140) lists step kinds for read-only run history.

## Goals / Non-Goals

**Goals:**
- A recipe stays plain data that is validated completely before anything is sent, including its actions.
- What the operator approves in the dry-run is exactly what is sent.
- A recipe pasted from elsewhere cannot make a connector able to mutate the lab without an instance admin stepping up.
- Built-in connectors keep their behaviour, except that their dry-run now rejects unsupported verbs.

**Non-Goals:**
- Operator-supplied parameters (user). The key `params` is not part of the format.
- Checking the response body for success (user): 2xx is success.
- A sync, a wait or any verification after an action. A runbook can add `sync_and_wait` or `wait_for_entity` steps.
- Config push through recipes.
- Exposing actions over MCP.
- Holding declared or imported actions inert until approved (user: the file and the backup are trusted).

## Decisions

**1. Format (user).** `actions` is an optional map from action name to definition, under `endpoints[i].entity` and at the recipe root:

```yaml
version: 1
category: other
auth: {mode: header, name: X-Api-Key}
endpoints:
  - name: containers
    path: /api/containers
    items: "@this"
    entity:
      kind: container
      name: name
      external_id: id
      attributes:
        node: {path: node}
      actions:
        restart:
          method: POST
          path: /api/nodes/{attr.node}/containers/{external_id}/restart
        stop:
          method: POST
          path: /api/containers/{external_id}/stop
          body: {force: false}
        rescan:
          method: POST
          path: /api/containers/{external_id}/rescan
          label: Rescan
          description: Re-reads the container's volumes.
          downtime_seconds: 0
actions:
  restart:
    method: POST
    path: /api/system/restart
```

Definition keys: `method` (POST, PUT, PATCH, DELETE), `path`, `query`, `headers`, `body`, `label`, `description`, `downtime_seconds`. Strict decoding rejects anything else, so `params` and similar are validation errors. Limits are in the spec. The version stays 1 because the key is optional and additive; an older server rejects such a recipe as having an unknown key, which is the safe direction.

**2. Entity actions are keyed by kind, and a kind may declare them on one endpoint only.** An action is triggered with an entity reference, which is the entity's external ID, as for built-in connectors. The handler finds that entity in the latest stored snapshot, takes its kind, and uses that kind's actions. Allowing two endpoints of the same kind to declare actions would make that lookup ambiguous. Alternative: address actions by endpoint name in the API; rejected because the entity reference convention is shared with every other connector and with runbook steps.

**3. Placeholders resolve from the stored snapshot, not from a fresh fetch.** `{external_id}` and `{attr.<name>}` are filled from the snapshot entity. This is what makes the dry-run exact without calling the service, and it keeps an action to one outbound request. Path values go through `connector.PathSegment`; query values are URL-encoded; a body string value that contains placeholders stays a JSON string after substitution. `{{` and `}}` escape braces, as in attribute templates. Any other `{...}` is a validation error, so the gjson template syntax of attribute mapping cannot be confused with it. Alternative: fetch the entity live before acting; rejected, since recipes have no per-item endpoints and it would double the calls.

**4. One execution path in the custom package.** New file `connector/custom/recipe_action.go`: resolve (recipe, verb or name, entity reference, snapshot entity) into a concrete request description, and send it. The description (method, absolute URL, static headers, body, label, description, downtime) is what the dry-run returns and what the sender consumes, so preview and execution cannot diverge. Sending reuses `buildRecipeRequestAt` (same-origin re-check, auth, connector `headers`), `recipeHTTPClient`, `connector.MapTransportError`, `connector.ReadBody` and `connector.CheckStatus`, with its own response handling because `fetchRecipePage` demands JSON. `custom.Connector` implements `Restarter`, `Starter` and `Stopper` by calling this path.

**5. Per-instance capability check.** `connector` gains an optional interface that a connector implements when its support depends on configuration (verb support for a given config, and the list of declared actions). `LifecycleOp`, `Capabilities` and `SupportsLifecycleVerb` consult it when present and fall back to the type assertion otherwise, so built-in types are untouched. `SupportsLifecycleVerb` must receive the connector record's config; its caller at runbook authoring (`api/runbooks/handlers.go` about line 269) has the connector. Type-level schema capabilities for `custom` report no lifecycle support, which is correct for a custom connector without a recipe. Connector instance responses gain `capabilities` and `actions`. Alternative: make every custom connector a `Restarter` and fail at call time; rejected because the UI, runbook authoring and bulk restart would all advertise operations that do not exist.

**6. Lifecycle verbs keep the existing operation (user).** `POST /connectors/{id}/restart|start|stop`, the elevation actions `connector.restart|start|stop` with an empty target, the audit actions and the alert on failure are unchanged. `PreviewLifecycleOp` gains the verb-support check for every connector (the existing gap) and, for a recipe action, the request block, the user-defined marker, label, description and the declared downtime. `mutateLifecycleOp` passes the method, redacted URL and status into the audit detail and returns the status and excerpt.

**7. Named actions get one endpoint and one elevation action, bound to connector and name (user).** `POST /api/connectors/{id}/actions/{name}` with body `{entityRef}` and `?dryRun=true`, on the same route group and guards as the lifecycle routes. Elevation action `connector.action`, target `<connector id>:<action name>`, validated in the handler after the support check, as lifecycle does. The audit action is `connector.action`. The string is added to all three allowlists; a test asserts the three stay in step for it. The web requests the token with that target. Alternative considered and rejected (user): unbound token as for lifecycle, which would let one step-up cover any action on any connector for its lifetime.

**8. Result handling (user).** After the request, read the body up to the normal cap and discard it, except for an excerpt: the first 512 bytes if they are valid UTF-8 text after dropping control characters, otherwise nothing. The excerpt is returned in the HTTP response of a successful call and inside the error of a non-2xx call. It is never passed to the audit writer, the alert, the logger or the run store; the function that produces it returns it separately from the error value used for those.

**9. Elevation when actions change (user).** The save path computes a canonical form of the actions in the stored and the submitted recipe (the parsed structures, serialised deterministically) and compares them. If they differ and the submitted set is not empty, the handler requires `auth.ValidateElevationHeaderFor` with a new action `connector.recipeActions` and target the connector id (empty target on create), and writes an audit row `connector.recipe_actions_changed` with the added, changed and removed action names. This runs after the existing instance-admin check for `recipe`. The action string joins the three allowlists. The web app reacts to the `elevation_required` answer of a save by opening the existing step-up dialog for that action and target and repeating the save; it does not parse YAML to predict it.

**10. Declared connectors and backups (user).** Reconcile computes the same canonical form and writes the same audit row, with the system actor it already uses, when a declared connector's actions are created or changed. Backup: `ValidateBundle` validates the config of every `custom` connector that has a recipe, through the registry (`connector.ValidateConfig`) so `backup` does not import the custom package, and fails before any write. The import audit entry gains the names of connectors imported with actions. Other connector types are not validated on import, to avoid breaking restores of older bundles.

**11. Recipe test preview.** `POST /connectors/recipe-preview` validates the whole recipe, so invalid actions are reported there, and executes only fetch endpoints.

**12. Runbook step kind `connector_action` (user; last part of the PR).** Migration `000066`, both drivers: widen the two kind CHECK constraints, add `action` (`NOT NULL DEFAULT ''`) to `runbook_steps` and `runbook_run_steps`, and `action_fingerprint` (`NOT NULL DEFAULT ''`) to `runbook_run_steps`. SQLite uses the rename, recreate, copy pattern of `000065`, including the indexes; the down migration deletes steps of the new kind and the runs that froze one before narrowing the check. The migration tests that chain rollbacks assume `000065` is newest and need a `000066` step in front. If another migration lands first, take the next free number.
- Authoring (`validateStepTarget`): rules in the spec. Lifecycle steps on custom connectors use the per-instance check from decision 5.
- Freeze: copy `action`, and store a fingerprint of the action's canonical definition. At execution the step resolves the action again and fails without sending if the fingerprint differs.
- Execution (`run.go` `perform`): a new method on the interface the executor uses for lifecycle (`executor.go` about line 133), implemented by the connectors handler, so the direct and the run path share one implementation, audit detail and alert.
- `unknown`: the sender reports whether the request was written before the failure. Written and no status received makes the step `unknown`; everything else is `failed`. This is in addition to the existing rule that an interrupted `running` step becomes `unknown`.
- Resume: the request body gains an optional decision for the first non-succeeded step, `resend` or `mark_done`. It is required when that step is a `connector_action` in state `unknown` (409 `unknown_step_decision_required` otherwise) and ignored in every other case. Audit actions `runbook.run.step_resent` and `runbook.run.step_marked_done`.
- Also: OpenAPI and the generated enum `runbookStepKind`, `mcp/runbooks.go`, the run preview, the step editor in `web/src/features/settings/RunbooksPage.tsx`.

**13. Web.** `web/src/features/services/ServiceDetailPage.tsx` (lifecycle buttons about lines 259-276, `useLifecycleOp` about lines 380-410) reads the connector's `capabilities` and `actions`: for a custom connector it shows only declared lifecycle controls and adds a control per named action, on the service and wherever per-entity lifecycle controls exist for entities of the declaring kind. Dialogs reuse `MutatingOpDialogs` (`components/manager/LifecycleOp.tsx`), `useMutatingOp` and `ElevationConfirm`; the dry-run dialog gains a request block and the result gains status and excerpt. Built-in connectors keep the buttons they have. `ConnectorEditPage.tsx` and the create flow handle the elevation retry of decision 9. Strings go into `web/src/i18n/en.ts` and `web/src/i18n/locales/pt-BR.ts`, and the new keys into the parity lists in `languages.test.ts`.

**14. ADR 0008.** `docs/adr/0008-recipe-defined-actions.md` records, against ADR 0001 and 0002: role, the three elevation actions involved, audit rows, the dry-run contract, the trust statement for configuration files and backups, and the non-goals. `docs/ARCHITECTURE.md` lists it with the other ADRs.

**15. One PR, ordered so the runbook step is last.** Task groups 1 to 8 are complete without group 9. If group 9 cannot be finished in this PR, the worker stops and asks the coordinator before cutting it; it then becomes a follow-up issue and the `runbook-runs` delta of this change is moved to that work.

## Risks / Trade-offs

- [A shared recipe carries a destructive call] → instance admin plus elevation to save it, an audit row naming the changed actions, a dry-run that shows the exact request, and operator plus elevation to run it.
- [Stale snapshot: the entity was renamed or removed upstream] → the request targets the stored external ID; the service answers 404 and the action fails visibly. No silent retargeting, because the reference is the ID, not a name.
- [A value from the service becomes part of a URL] → path values pass `connector.PathSegment`; the built request is re-checked for same origin; redirects are refused; the dialer blocks loopback and link-local.
- [The excerpt shows something sensitive to the operator] → 512 bytes, text only, shown once to a user who already holds operator on that connector, never stored.
- [Instance step-up disabled] → saving and running pass the elevation check as every elevated action does; the instance-admin and operator requirements still apply. Documented in ADR 0008.
- [A restored backup or a config file enables actions without step-up] → accepted (user); both are audited.
- [Dry-run now rejects unsupported verbs for built-in connectors] → clients that called the preview for an unsupported verb get a 400 instead of a preview that could never be executed; the web app does not do this for built-in types.
- [SQLite table rebuild in the migration] → follow `000065` and its tests.
- [Allowlist in three places drifts] → a test covers the two new action strings in all three.

## Migration Plan

One additive paired migration (`000066`) in task group 9, with a down migration that deletes `connector_action` steps and the runs that froze one. Nothing else needs data migration: existing recipes have no actions. Rollback before group 9 ships is a plain revert.
