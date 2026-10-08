# Tasks

## 1. Document model

- [x] 1.1 Add the `yaml` dependency and a recipe document module that parses text, reports a syntax error with its line, detects the cases the form will not open (not one mapping document, anchors, aliases, merge keys) and serialises; verify with unit tests for each case and a test that every shipped example (`docs/connectors/recipes/*.yaml`, the examples in `docs/connectors/RECIPE_FORMAT.md`) and a set of hand-written fixtures (comments, flow maps, quoted keys, CRLF, BOM) serialise byte-identically when unedited.
- [x] 1.2 Add targeted edit operations (set a scalar with a chosen type, add and remove a key, add, remove and move a sequence item, insert a parsed block) that change only the touched node; verify with tests that assert a single-line or single-block diff and that comments and unrelated formatting survive.
- [x] 1.3 Add the format table that lists the known keys at each level with their input kinds, and a function that reports unknown keys per node; verify with tests against the reference examples (no unknown keys) and a fixture with unknown keys at root, endpoint, entity and attribute level.
- [x] 1.4 Replace the hand scanner in `readRecipeCategory` with a read through the parsed document; verify its existing tests still pass unchanged.

## 2. Tabs and YAML editor

- [x] 2.1 Add the `RecipeEditor` with `Form | YAML` tabs over the existing recipe form value, default tab selection and the remembered choice, with keyboard support and selected state exposed; verify with component tests for switching, the default on empty, parseable and unparseable text, and that the saved value is the YAML text.
- [x] 2.2 Replace the textarea in the YAML tab with a lazily loaded CodeMirror editor using `@codemirror/lang-yaml`, with line numbers, the plain textarea as loading and failure fallback, and the read-only state; verify with component tests for the fallback, read-only mode and the labelled control, and that the unsaved-changes guard still triggers after an edit in either tab.

## 3. Form rows

- [x] 3.1 Render and edit category and authentication (mode, name, prefix, with fields following the mode); verify with component tests that each control writes the expected YAML and reads it back.
- [x] 3.2 Render and edit endpoints: add, remove, reorder, name, path, method, query parameters, headers, body block and items path; verify with component tests, including that removing an endpoint removes only that item.
- [x] 3.3 Render and edit pagination with the fields of the selected style and no fields of other styles; verify with a test per style.
- [x] 3.4 Render and edit the entity mapping (kind, name, external ID, hostname, IP, MAC, aliases) and attributes (source, type, value map entries, default, with typed scalars); verify with component tests for every scenario of the spec's "Form coverage" requirement except action rows.
- [x] 3.5 Render and edit endpoint dependencies and recipe dependencies; verify with component tests.
- [x] 3.6 Offer to start a recipe from an empty field without writing text until accepted, and verify with an API-shaped test that a recipe built only through form rows matches a fixture the backend's parser accepts (reuse a documented example as the expected text).

## 4. Text the form cannot represent

- [x] 4.1 Make the Form tab unavailable with line and reason for syntax errors, non-mapping or multiple documents, anchors, aliases and merge keys, leaving the YAML tab usable; verify with component tests for each case.
- [x] 4.2 Mark rows that hold unknown keys, keep those keys through every edit, and confirm before removing a row that holds them; verify with component tests for every scenario of "Text the form cannot represent".

## 5. Validation and testing in place

- [x] 5.1 Resolve server error locations to a form row and field and to a line range in the text, show messages on the field with `aria-invalid` and a description, keep unresolvable ones in the existing error list, and move focus to the first error after a failed save; verify with unit tests of the resolver over nested locations and component tests for every scenario of "Server validation shown in place".
- [x] 5.2 Show located errors and the parser's syntax error as diagnostics on the matching lines in the YAML tab; verify with a component test.
- [x] 5.3 Place the test panel below both tabs, test the current text from either, and badge endpoint rows with their result; verify with component tests for both scenarios of "Testing from either view".

## 6. Rights, languages and docs

- [x] 6.1 Honour the read-only state in the form for users who may not edit the recipe; verify with a component test.
- [x] 6.2 Add every new string to `en.ts` and `pt-BR.ts` and the keys to the parity lists in `languages.test.ts`; verify that test passes.
- [x] 6.3 Add a short section on the form to `docs/connectors/RECIPE_FORMAT.md` covering the two tabs, what is preserved and when the form is unavailable; verify the documented-example tests of the backend still pass (the markers in that file must not move).

## 7. Action rows (only after the coordinator confirms `recipe-actions` has merged; rebase onto main first)

- [ ] 7.1 Add the action keys to the format table and rows for entity actions and service actions with every field an action defines, following `docs/connectors/RECIPE_FORMAT.md` as merged; verify with component tests for the "Action rows" scenario and that a recipe with actions opens with no unknown-key markers.
- [ ] 7.2 Verify the connector pages' step-up retry on save still works when the changed actions were edited through the form, with a component test, and add the en and pt-BR strings for the action rows to the parity lists.

## 8. Integration

- [x] 8.1 Run the web typecheck, lint and full vitest suite and the bundle build, and record in the PR the size change of the connectors chunk; verify all pass in CI.
- [ ] 8.2 Manual check, left to the maintainer and never ticked by a worker: open an existing hand-written recipe with comments, edit one field in the form, confirm only that line changed, break the YAML and confirm the form says where, and save a recipe with an invalid mapping to see the error on its row.
