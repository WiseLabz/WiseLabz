# Design

## Context

See proposal.md for motivation. The decisions marked "(user)" were made with the maintainer on 2026-10-08 and are not open. Current state (paths under `web/src/features/connectors/` unless stated; line numbers are from `main` at `b27dfb1`):

- `ConnectorForm.tsx` (about lines 370-393) renders the recipe, a schema field of type `textarea`, as a plain `<textarea>` (12 rows, monospace). CodeMirror is used only in `web/src/features/docs/DocEditorPage.tsx`; the packages present are `@codemirror/commands`, `@codemirror/lang-markdown`, `@codemirror/state`, `@codemirror/view`, `@uiw/react-codemirror` and `codemirror`.
- There is no YAML library in `web/package.json`. `recipeForm.ts` `readRecipeCategory` scans the text by hand for the root `category`, because the category shown for a custom connector is derived from the recipe.
- Validation errors come back from the server with dotted locations such as `config.recipe.endpoints[1].entity.external_id`. `recipeForm.ts` has `locatedErrorsFrom`, `errorsForField` and `focusFirstLocatedError`; `LocatedErrors.tsx` lists them as `<code>field</code>: message`. Today they all attach to the single recipe field.
- `TestRecipePanel.tsx` renders only for instance admins and calls `usePreviewConnectorRecipe` with `{url, verifyTls, config, connectorId?}`; the result has per-endpoint `items`, `count`, `skipped`, `samples`, `dependencies` and `error`.
- `ConnectorEditPage.tsx` always resends the recipe text on save and has a `beforeunload` guard for a dirty recipe (about lines 146-162). Connectors declared in `config.yaml` show an error state instead of the form. Changing `recipe` requires an instance admin on the server.
- Strings live in `web/src/i18n/en.ts` and `web/src/i18n/locales/pt-BR.ts` (lazy, `DeepPartial`); `languages.test.ts` asserts listed keys exist in both.
- The recipe format is defined by `backend/internal/connector/custom/recipe.go` and documented in `docs/connectors/RECIPE_FORMAT.md`; examples are `docs/connectors/recipes/sonarr.yaml` and `jellyfin.yaml`, plus the pagination examples between markers in the reference.
- The web app uses bun, vitest and React Testing Library.

## Goals / Non-Goals

**Goals:**
- The YAML text stays the single source of truth; the form is a view that edits it.
- Nothing a person wrote by hand is lost by using the form.
- No second copy of the validation rules.

**Non-Goals:**
- Client-side validation of recipe semantics (user).
- A validate-only endpoint or any backend change (user).
- Editing recipes of connectors declared in `config.yaml`.
- Recipe templates, a gallery, or import from a URL.
- Autocomplete of JSON paths from a sample response.

## Decisions

**1. Tabs over one string (user).** A `RecipeEditor` component owns nothing but the tab state; the recipe text stays in the connector form state where it is today, so save, dirty tracking and the `beforeunload` guard are untouched. Tabs are `Form | YAML`. The default tab is Form when the text is empty or parses, YAML otherwise; the last choice is remembered per browser in `localStorage`, read and written inside try/catch. Alternatives rejected (user): side by side, and a form with a YAML drawer.

**2. `yaml` package, Document API (user).** The form parses the text with `parseDocument` (keeping the CST) and applies each edit as a targeted mutation of the document (`setIn`, `deleteIn`, adding a pair or a sequence item), then serialises with `toString()`. Untouched nodes keep their source tokens, which is what preserves comments, order and quoting. The document is re-parsed from the text whenever the text changes from outside the form (typing in the YAML tab, loading the connector). Plain `parse` and `stringify` are not used for edits. Alternative rejected (user): convert on the server, which loses comments and needs a round trip per switch.
 - `toString()` options are chosen so that an unedited document serialises byte-identically; a test over the shipped examples and a set of hand-written fixtures (comments, flow maps, quoted keys, CRLF, a BOM, trailing spaces) pins this. If some input cannot round-trip unchanged, the form does not write on open: text is only replaced when the user makes an edit.
 - New nodes are created in block style with two-space indentation, matching the reference.

**3. A schema table drives the rows.** One TypeScript description of the format (which keys exist at each level, their input kind and options) drives both rendering and "is this key known". It mirrors `recipe.go`; a comment names that file as the source. It is not a validator: it decides which rows to draw, not what is valid. Keys present in the document and absent from the table are "unknown" for decision 5.

**4. Scalars keep their type.** Constants, defaults and value map values can be strings, numbers, booleans or null. Those inputs carry a small type selector; an existing value shows its current type, and an edit writes a scalar of the selected type. Paths, names and templates are always strings and are quoted only when YAML requires it. The endpoint `body` is edited as a small text block that must parse as JSON or YAML and is inserted as a node.

**5. What the form will not open, and what it keeps (user).** The Form tab is unavailable, with line and reason from the parser, when: the text has a YAML error; it is not exactly one document whose root is a mapping; or it uses anchors, aliases or merge keys (an edit through an alias would change several places at once). Otherwise it opens. Unknown keys are never touched; the nearest row shows a "more in YAML" marker, and removing a row that holds unknown content asks first. Before the action rows exist, `actions` blocks are exactly this case, which is how the two changes can be developed in parallel.

**6. Errors by location (user: server only).** A small resolver turns a server location (`endpoints[1].entity.attributes.status.map`) into (a) a row id and field for the form and (b) a node in the document, hence a line range for the YAML tab. The form expands the row, marks the field with `aria-invalid` and `aria-describedby`, and shows the message; unresolvable locations stay in the existing `LocatedErrors` list. `focusFirstLocatedError` is extended to open the right tab-independent target: in the Form tab the field, in the YAML tab the line. Errors are attached to the text they were returned for; after the text changes they are shown as stale until the next Test or Save.

**7. Test panel.** `TestRecipePanel` moves below the tabs unchanged in behaviour. Its per-endpoint result is passed up so the form can badge an endpoint row as failed or show its item count.

**8. YAML tab on CodeMirror (user).** `@uiw/react-codemirror` with `@codemirror/lang-yaml`, line numbers, and diagnostics for located errors and for the parser's own syntax error. The editor is loaded lazily so pages for other connector types do not pay for it; while it loads, and if it fails to load, the plain textarea is shown and stays fully functional. The field keeps an accessible label.

**9. `readRecipeCategory`.** Replace the hand scanner with a read through the same parsed document, keeping its current behaviour for unparseable text (no category). Its existing tests must still pass.

**10. Read-only.** The editor takes the same `disabled`/read-only signal the textarea receives today; in that state the form renders values without controls and the YAML tab is read-only.

**11. Action rows last (user).** They are task group 7 and depend on the `recipe-actions` change being merged to `main`: the field list of an action (method, path, query, headers, body, label, description, downtime) and the two places it can be declared come from that change's recipe reference. The worker starts group 7 only after the coordinator says the other PR has merged, then rebases onto `main`. Saving a recipe with changed actions then answers `elevation_required`; that retry is implemented by the other change in the connector pages, and this change must not break it.

**12. Strings.** New keys under the connectors section of `en.ts` and `pt-BR.ts`, and in the parity lists of `languages.test.ts`.

## Risks / Trade-offs

- [A form edit reformats the file] → edits go through the Document API only; byte-identity tests on open-without-edit and single-line-diff tests on edit.
- [The form's idea of the format drifts from the server's] → the form never validates, unknown keys are preserved, and the server's located errors are shown wherever they point. Drift degrades to "edit this in YAML", not to data loss.
- [Bundle size] → `yaml` is loaded with the builder and CodeMirror YAML mode lazily, only for the custom connector's recipe field.
- [Large recipes are slow to re-parse on every keystroke in the YAML tab] → recipes are capped at 64 KiB by the server; re-parse is debounced and happens on tab switch at the latest.
- [Anchors and aliases lock the form] → rare in recipes; the YAML tab stays fully usable and the message says why.
- [Located error cannot be mapped to a line] → it stays in the error list, as today.

## Migration Plan

None. No stored data changes. Reverting the PR restores the plain textarea.
