# Tasks

## 1. Recipe format

- [ ] 1.1 Parse and validate `actions` under `endpoints[i].entity` and at the recipe root (names, methods, relative paths, static query, headers and body, metadata limits, 10 per place, one declaring endpoint per kind, strict unknown-key rejection), reporting every problem with its dotted location; verify with table tests in `connector/custom` covering each rule in the spec's "Action declaration", "Action request" and "Action metadata" requirements and a test that a recipe without actions parses as before.
- [ ] 1.2 Validate placeholders: `{external_id}` and `{attr.<name>}` only, `{{`/`}}` escapes, mapped attributes only, none in service actions; verify with tests for each error and for a valid path, query value and body string value.
- [ ] 1.3 Add a deterministic canonical form of a recipe's actions and a diff of action names (added, changed, removed); verify with tests that comments, whitespace and key order do not change it and that any value change does.
- [ ] 1.4 Document actions in `docs/connectors/RECIPE_FORMAT.md` with an example declaring an entity action and a service action, and load that documented text in a test as the existing documented-recipe tests do; verify the test fails when the example is made invalid.

## 2. Per-connector capabilities

- [ ] 2.1 Add the optional per-instance capability interface to `connector`, consult it in `LifecycleOp`, `Capabilities` and `SupportsLifecycleVerb` (which now receives the connector's config), and implement it in `custom`; verify with tests that two custom configs report different support and that every built-in type reports what it did before.
- [ ] 2.2 Expose `capabilities` and `actions` on connector responses, in `docs/openapi.yaml` and the regenerated web client; verify with handler tests and the OpenAPI contract test.

## 3. Action execution in the custom package

- [ ] 3.1 Resolve an action (verb or name, entity reference, snapshot entity) into a concrete request description, using `connector.PathSegment` for path values; verify with tests for the spec's "Target placeholders" scenarios, including values that must be rejected before sending.
- [ ] 3.2 Send a resolved action through the existing recipe request builder and client, returning status, a text excerpt of at most 512 bytes, and whether the request had been written when a failure occurred; verify with `httptest` servers for each method, non-JSON bodies, non-2xx, a redirect, a cross-origin result, an oversized body, a binary body and a dropped connection.
- [ ] 3.3 Implement `Restarter`, `Starter` and `Stopper` on the custom connector on top of 3.1 and 3.2; verify with tests that each verb sends exactly the declared request once.

## 4. Lifecycle operations and the dry-run

- [ ] 4.1 Reject unsupported verbs in the lifecycle dry-run for every connector type; verify with a test on a built-in type and one on a custom connector without the verb.
- [ ] 4.2 Extend the lifecycle dry-run for recipe actions with the request block (method, URL, static headers, body), the user-defined marker, label, description and declared downtime, without contacting the service and without credentials; verify with handler tests including query-token redaction and a server that fails the test if called.
- [ ] 4.3 Run recipe-declared `restart`, `start` and `stop` through the existing mutate path with method, redacted URL and status in the audit detail and the status and excerpt in the response; verify with handler tests for elevation, role, audit content, the failure alert, and that no excerpt reaches the audit row, alert or log.
- [ ] 4.4 Verify bulk restart includes a custom connector only when its recipe declares a service `restart`, with a test.

## 5. Named actions

- [ ] 5.1 Add `connector.action` to the three elevation allowlists and `POST /api/connectors/{id}/actions/{name}` with `dryRun`, the operator role guard and an elevation token bound to `<connector id>:<action name>`; verify with handler tests for every scenario of "Running a named action", including a token for another action and for another connector, and a test that the three allowlists agree on the new strings.
- [ ] 5.2 Record the `connector.action` audit entry and raise the failure alert as lifecycle does; verify with tests of the audit detail and of the alert on a non-2xx answer.
- [ ] 5.3 Document the endpoint in `docs/openapi.yaml` and regenerate the web client; verify with the OpenAPI contract test.

## 6. Enabling actions

- [ ] 6.1 Require elevation `connector.recipeActions` (bound to the connector on update, unbound on create) when a save changes the actions to a non-empty set, add the string to the three allowlists, and write the `connector.recipe_actions_changed` audit row; verify with handler tests for every scenario of "Enabling actions on save".
- [ ] 6.2 Write the same audit row from reconcile when a declared connector's actions are created or changed, and none when unchanged; verify with reconcile tests, and add a short note to `docs/CONNECTORS_IN_CONFIG.md`.
- [ ] 6.3 Validate custom recipes in `ValidateBundle` through the registry, reject the bundle before any write when one is invalid, and name connectors imported with actions in the import audit entry; verify with backup tests for a valid bundle with actions, an invalid one, and a bundle with other connector types that must import as before.
- [ ] 6.4 Verify the recipe test preview validates actions and sends none, with a test whose server fails on any action path.

## 7. Web app

- [ ] 7.1 Show lifecycle controls from the connector's capabilities for custom connectors and add controls for named actions on the service and on entities of the declaring kind; verify with component tests that a recipe declaring only `restart` and `rescan` yields exactly those two controls and that a built-in connector's controls are unchanged.
- [ ] 7.2 Show the request block, label and description in the dry-run dialog, request the bound elevation token for named actions, and show status and excerpt in the result; verify with component tests of the dialog sequence.
- [ ] 7.3 Handle `elevation_required` on connector create and update by asking for step-up for `connector.recipeActions` and repeating the save; verify with a component test.
- [ ] 7.4 Add the new strings to `en.ts` and `pt-BR.ts` and their keys to the parity lists in `languages.test.ts`; verify that test passes.

## 8. Decision record

- [ ] 8.1 Write `docs/adr/0008-recipe-defined-actions.md` covering the points in design decision 14 and list it in `docs/ARCHITECTURE.md`; verify both files link correctly and the ADR states the non-goals.

## 9. Runbook step kind (last; ask the coordinator before cutting it)

- [ ] 9.1 Add migration `000066` for both drivers (widen both kind checks, add `action` to both step tables and `action_fingerprint` to run steps, sqlite rebuild with indexes, down migration that deletes the new-kind steps and the runs that froze one) and put a `000066` rollback step in front of the chained migration tests; verify with up and down tests on sqlite and postgres.
- [ ] 9.2 Store, return, freeze and back up the new fields; verify with store tests, a freeze test and a backup round-trip test.
- [ ] 9.3 Validate `connector_action` steps and lifecycle steps on custom connectors at authoring; verify with handler tests for every scenario of "Connector-action step authoring" and the changed "Step constraints".
- [ ] 9.4 Execute `connector_action` steps through the shared named-action implementation with run and step identifiers in the audit detail, fail without sending when the fingerprint changed, and set `unknown` when the request was written and no status arrived; verify with executor tests for every scenario of "Connector-action step execution".
- [ ] 9.5 Require a decision on resume for an `unknown` `connector_action` step (`resend` or `mark_done`), with the two audit actions and the 409 otherwise; verify with handler tests for every new scenario of "Resuming a failed run", including that an unknown lifecycle step still repeats without a decision.
- [ ] 9.6 Show the request in the run preview, list the kind in `mcp/runbooks.go`, update `docs/openapi.yaml` and the generated client, and add the kind to the step editor and the resume dialog in the web app with en and pt-BR strings; verify with handler, MCP and component tests and the OpenAPI contract test.

## 10. Integration

- [ ] 10.1 Add an API-level test that creates a custom connector with a recipe declaring entity and service actions (with elevation), syncs it against a fake service, runs a dry-run, a lifecycle verb and a named action, and checks the requests received, the audit entries and the capabilities; verify it passes in CI.
- [ ] 10.2 Manual check, left to the maintainer and never ticked by a worker: against a real REST service, declare a restart and a named action, confirm the save asks for step-up, the dry-run shows the request, the action takes effect, and the audit log has no response body.
