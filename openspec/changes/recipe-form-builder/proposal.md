# Proposal

## Why

A custom connector recipe (#513) is written as YAML in a plain text area. That works for people who already know the format, but the format has grown to endpoints, four pagination styles, entity mappings, typed attributes with value maps and templates, and dependencies, and a mistake is only reported after pressing Test or Save. Issue #651 asks for a structured builder on top of the same YAML, so both ways of editing stay interchangeable.

## What Changes

- The recipe field of a custom connector gets two tabs over the same text: **Form** and **YAML**. Either can be used at any time and each reflects the other's edits. The test panel sits below both and works from either.
- The Form tab has rows for everything the format defines: category, authentication, endpoints (request, items path, pagination), entity field mappings, attributes (source, type, value map, default), and dependencies. Rows for actions are added once the `recipe-actions` change has merged.
- Form edits rewrite only the part of the text they touch. Comments, key order and formatting written by hand are kept.
- If the text cannot be parsed, the Form tab is unavailable and says where the problem is. If the text parses but contains keys the form has no row for, the form opens, leaves those keys untouched and marks them as editable in YAML only.
- Validation stays on the server. Errors returned by Test and Save are shown on the matching row and field in the Form tab and on the matching line in the YAML tab.
- The YAML tab becomes a code editor with YAML highlighting instead of a plain text area.
- New interface text ships in English and Brazilian Portuguese.

No backend, API or recipe format change. A connector saved through the form is the same YAML string as before.

## Capabilities

### New Capabilities
- `recipe-form-builder`: the structured editor for custom connector recipes in the web app: the two views, what the form covers, how it preserves hand-written YAML, how it behaves on text it cannot represent, and how server validation errors are shown.

### Modified Capabilities

None. The requirements of `custom-rest-recipes` are unchanged: the recipe is still one YAML document validated by the server.

## Impact

- Web: `web/src/features/connectors/` (`ConnectorForm.tsx`, `recipeForm.ts`, `TestRecipePanel.tsx`, `ConnectorEditPage.tsx`, new builder components), `web/src/i18n/en.ts`, `web/src/i18n/locales/pt-BR.ts`, `languages.test.ts`.
- New web dependencies: `yaml` (parsing and comment-preserving edits) and `@codemirror/lang-yaml` (highlighting); `@codemirror/lint` if it is not already a direct dependency. CodeMirror itself is already used by the docs editor.
- Docs: a short section on the form in `docs/connectors/RECIPE_FORMAT.md`.
- Depends on `recipe-actions` only for the action rows, which are the last task group.
