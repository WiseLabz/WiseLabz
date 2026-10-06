# Proposal

## Why

The Custom HTTP connector is a stub: it makes one unauthenticated-by-design GET, only understands a response that is already shaped as WiseLabz entities, and is filed under "virtualization" (#513). Any homelab service without a native connector, such as the *arr stack or Jellyfin, cannot be documented without writing Go.

## What Changes

- The custom connector accepts a **recipe**: a YAML document in its config that declares the connector's category, authentication mode, one or more endpoints, and how each response maps to entities, attributes and dependencies.
- Mapping uses path expressions over the JSON response, with typed attributes, value maps and string templates. Every mapped entity has a required stable external ID.
- Recipes support GET and POST with a static body, and four pagination styles (page number, offset, cursor, next link) with hard caps.
- Requests stay on the connector's own origin. Credentials live in separate encrypted fields, so a recipe can be shared without secrets.
- A failing endpoint fails the whole sync; no partial snapshot is stored.
- A custom connector without a recipe behaves exactly as today.
- Admins can **test a recipe** and see the mapped result without storing anything. The connector form gets a YAML editor with validation errors.
- Four new connector categories: `storage`, `monitoring`, `media`, `other`. Existing connectors keep their category.
- Two documented example recipes (Sonarr, Jellyfin) double as test fixtures.

Out of scope, tracked separately: structured form builder (#651), recipe-defined lifecycle actions (#653), in-app recipe library and further ready-made recipes (#516).

## Capabilities

### New Capabilities
- `custom-rest-recipes`: the recipe format, its validation, request and pagination rules, response mapping, failure behaviour, compatibility with recipe-less custom connectors, and the test preview.
- `connector-categories`: the set of valid connector categories and how a connector's category is determined.

### Modified Capabilities

None. No existing spec covers connectors.

## Impact

- **Dependencies**: adds `github.com/tidwall/gjson`. YAML parsing uses the existing `go.yaml.in/yaml/v3`.
- **Database**: one paired migration widening the connector category CHECK constraint.
- **Backend**: `internal/connector/custom` (recipe parse, validate, fetch, map, paginate), `internal/connector/registry.go` (multi-line non-secret field kind), `internal/connector/reconcile` and `internal/api/connectors` (category from recipe, preview endpoint), `internal/backup` (valid categories).
- **API**: `ConnectorCategory` enum gains four values; new recipe preview endpoint; custom type schema gains fields. Generated web client regenerated. No breaking change.
- **Web**: connector form (YAML field, test preview panel), category icons and labels, template editor category list, en and pt-BR strings.
- **Docs**: recipe reference and examples under `docs/connectors`, `docs/CONNECTORS_IN_CONFIG.md`, `docs/BACKUP.md`, `docs/AUDIT.md`.
