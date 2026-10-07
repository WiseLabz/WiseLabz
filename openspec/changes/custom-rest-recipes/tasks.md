# Tasks

## 1. Connector categories (PR 1)

- [x] 1.1 Add a paired sqlite/postgres migration widening the connector category CHECK constraint to the eight values, with a down migration; verify migration up/down tests on both dialects and that existing rows keep their category.
- [x] 1.2 Add the four categories to `backup.validCategories` and validate category against the shared list in the connectors create/update handlers; verify handler tests for an accepted new category and a rejected unknown one, and a backup import test with category `monitoring`.
- [x] 1.3 Extend the OpenAPI `ConnectorCategory` enum and regenerate the web client; verify the OpenAPI contract test and web typecheck.
- [x] 1.4 Add icons in `categoryIcon.ts`, the template editor `CATEGORIES` entries and en and pt-BR labels; verify vitest for the grouped connector list and template editor, and update `docs/BACKUP.md` where categories are listed.

## 2. Recipe format and validation (PR 2)

- [ ] 2.1 Add `github.com/tidwall/gjson` to `go.mod`; verify `go build ./...` and `go mod tidy` leave no diff.
- [ ] 2.2 Add the `textarea` schema field kind (multi-line, not encrypted, returned by the API) to the registry and config validation; verify registry tests and that `store.IsSecretFieldType` does not treat it as secret.
- [ ] 2.3 Implement recipe types, strict YAML parsing and semantic validation with located, aggregated errors in `internal/connector/custom/recipe.go`; verify table tests for every validation error in the spec, the 64 KiB and 20 endpoint limits, and several errors reported together.
- [ ] 2.4 Register the `recipe`, `auth_token`, `auth_username` and `auth_password` fields on the custom type and validate mode-required credentials as config field errors; verify custom connector config tests and `ValidateDeclared` tests for a valid and an invalid declared recipe.

## 3. Recipe fetch and mapping (PR 2)

- [ ] 3.1 Implement request building for GET and POST with static body, query, headers, the four auth modes and the same-origin check; verify `httptest` tests for each auth mode, a rejected absolute path and the legacy headers still applied.
- [ ] 3.2 Implement entity mapping with required external ID, skipped-item counting and duplicate detection; verify tests for mapped items, an item without identifier and a duplicate identifier.
- [ ] 3.3 Implement attribute mapping: path, constant, template, type conversion, value map and default; verify table tests for every attribute scenario in the spec including conversion failure.
- [ ] 3.4 Implement dependency mapping from constants and paths with de-duplication; verify tests for both sources.
- [ ] 3.5 Branch `Fetch` and `Validate` on the presence of a recipe, fail the whole sync on any endpoint error with the endpoint named and classified, and add the URL redaction helper; verify all existing `custom_test.go` tests pass unchanged, plus tests for one endpoint down, items path not a list, 401 as auth error and a query token absent from the error text.
- [ ] 3.6 Derive the category from the recipe in the connectors create/update handlers and in `reconcile` through a per-type registry hook; verify handler and reconcile tests for recipe-set category, a conflicting request category and a recipe-less connector staying `virtualization`.
- [ ] 3.7 Write the recipe format reference in `docs/connectors` and the declared-connector example in `docs/CONNECTORS_IN_CONFIG.md`; verify the documented example is loaded by a test.

## 4. Pagination (PR 3)

- [ ] 4.1 Implement the pagination loop with page-number and offset styles and the 100 page and 10,000 entity caps; verify tests for stop on empty page and both caps failing the sync.
- [ ] 4.2 Implement cursor and next-link styles (body path and `Link` header) with non-advancing detection and the same-origin check on links; verify tests for each end condition, a repeated cursor and a cross-origin next link that sends no request.
- [ ] 4.3 Document pagination in the recipe reference; verify each documented snippet parses in a test.

## 5. Preview endpoint (PR 4)

- [ ] 5.1 Add a non-aborting run mode returning per-endpoint counts, skipped counts, up to 20 sample entities, dependencies and errors; verify a test where one endpoint fails and the others still report results.
- [ ] 5.2 Add admin-only `POST /api/connectors/recipe-preview` with stored-credential fill-in for an existing connector, audit entry `connector.recipe_preview` and credential-free output; verify handler tests for unsaved preview, stored credentials, 403 for non-admin, validation errors, and that no snapshot, change or alert is written.
- [ ] 5.3 Add the endpoint to OpenAPI, regenerate the web client and add the action to `docs/AUDIT.md`; verify the OpenAPI contract test.

## 6. Web editor and examples (PR 4)

- [ ] 6.1 Render `textarea` fields as a monospace multi-line editor in the connector form and show located recipe validation errors; verify vitest for editing, error display and that the stored recipe is shown when editing a connector.
- [ ] 6.2 Add the Test recipe panel showing per-endpoint results, samples and errors, and show the recipe's category as read-only in the form; verify vitest for success, endpoint error and validation error states, with en and pt-BR strings.
- [ ] 6.3 Add `docs/connectors/recipes/sonarr.yaml` and `jellyfin.yaml` with recorded anonymised responses in `testdata`; verify tests that load the documented files and assert the expected entities, attributes and dependencies.

## 7. Integration

- [ ] 7.1 Run `openspec validate custom-rest-recipes --strict`, backend tests and lint with `GOFLAGS=-p=4 GOMAXPROCS=4`, then web tests, lint and typecheck at concurrency 4, sequentially; record the results.
- [ ] 7.2 Run the app, create a custom connector from the Sonarr example against a local mock, preview it, sync it, confirm entities, category grouping and a recipe-less custom connector still syncing; then run `graphify update .`.
