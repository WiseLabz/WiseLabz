# Design

## Context

See proposal.md for motivation. Current state that shapes the approach:

- `internal/connector/custom/custom.go` registers type `custom` with fields `url`, `method`, `headers` (a "secret" JSON blob). `Fetch` makes one request, stores the body as a section and reads a top-level `entities` array via `tryParseEntities`.
- All connector HTTP goes through `connector.NewHTTPClient`, whose dialer blocks loopback and link-local targets and refuses redirects; bodies are capped by `connector.ReadBody` (`MaxResponseBytes`); `connector.CheckStatus`, `MapTransportError`, `NewAuthError` and `NewServiceUnavailableError` classify failures.
- Category is a column on the connector row. The API create handler takes it from the request; `reconcile.go` passes `schema.Category` for declared connectors. The database has a CHECK constraint on the four current values (migration `000022`), and `backup.validCategories` mirrors it.
- `SchemaField.Type` is one of text, password, number, select, toggle, secret. "secret" is the only multi-line kind and is encrypted at rest; `ValidateDeclared` rejects unknown config keys for declared connectors.
- `ServiceDependency` is snapshot-level: `{Kind, Name}` with kinds host, network, storage, upstream_service.
- The custom type's attribute catalog is the `*` wildcard.
- `go.yaml.in/yaml/v3` is already a dependency; no JSON path library is.

## Goals / Non-Goals

**Goals:**
- A recipe is plain data: it can be validated completely before any request is made.
- Fetch stays read-intent and confined to the connector's origin.
- Zero behaviour change for custom connectors without a recipe.

**Non-Goals:**
- XML, HTML or Prometheus text responses. Recipes map JSON only.
- Per-item follow-up requests (fetching a detail endpoint for each list item).
- A general expression language. Shaping is limited to path, constant, template, type and value map.
- Per-instance attribute catalogs for compliance authoring; the wildcard catalog stays.

## Decisions

**1. Recipe as one YAML string in connector config, parsed with strict decoding.**
`yaml.v3` with `KnownFields(true)` into Go structs, then a semantic validation pass that collects all errors with a dotted location (`endpoints[1].entity.external_id`). Alternative: a recipes table shared across connectors; rejected in planning because it adds a resource, permissions and backup surface for little gain.

Format (version 1):

```yaml
version: 1
category: media
auth:
  mode: header            # none | header | basic | query
  name: X-Api-Key         # header or query parameter name
  prefix: ""              # header mode only, e.g. "Bearer "
endpoints:
  - name: series
    path: /api/v3/series
    method: GET           # GET | POST
    query: {includeSeasonImages: "false"}
    headers: {}
    body: null            # POST only, static JSON
    items: "@this"        # gjson path to the list
    pagination:
      type: page          # page | offset | cursor | next_link
      param: page
      size_param: pageSize
      size: 100
      start: 1
      cursor_path: ""     # cursor: path in the response
      next_path: ""       # next_link: path in the body, or
      link_header: false  # next_link: use the Link header
    entity:
      kind: series
      name: title
      external_id: id
      hostname: ""
      ip: ""
      mac: ""
      aliases: ""
      attributes:
        monitored: {path: monitored, type: bool}
        status: {path: status, map: {continuing: active}, default: other}
        address: {template: "{host}:{port}"}
        source: {const: sonarr}
    dependencies:
      - {kind: upstream_service, path: "#.downloadClient"}
dependencies:
  - {kind: storage, const: tank}
```

**2. gjson for paths.**
Small, dependency-free, validates syntax cheaply, and its array syntax (`#.name`) gives multi-value results for aliases and dependencies. Paths in `entity` are evaluated against one item; `items`, pagination paths and endpoint-level dependency paths against the response root. Alternative: RFC 9535 JSONPath; rejected for a heavier dependency with no homelab benefit.

**3. Templates and value maps are deliberately tiny.**
A template is literal text with `{gjson path}` placeholders, `{{` and `}}` escaping braces; a missing placeholder path makes the attribute absent. A value map is keyed by the string form of the resolved value and applies before type conversion. Type conversion is strict; failure is a sync error so a recipe bug is visible rather than silently producing wrong compliance input.

**4. Credentials as new schema fields on the custom type.**
`auth_token` (password), `auth_username` (text), `auth_password` (password), plus `recipe`. The recipe names which mode uses them, so the same recipe works on any instance. Cross-field validation (mode requires which field) happens in the custom connector's config validation, surfaced as `ConfigValidationError` so the form and `ValidateDeclared` both report it. The legacy `headers` and `method` fields stay; `headers` is still applied to recipe requests, `method` only without a recipe.

**5. New `textarea` schema field kind for the recipe.**
Multi-line like "secret" but not encrypted and returned by the API, since recipes are meant to be read and copied. The web form renders it with a monospace editor. Alternative: reuse "secret"; rejected because the recipe would be write-only in the UI.

**6. Same-origin enforced when building each request.**
Endpoint paths are validated as relative (no scheme, no `//`). Every request URL, including next links, is resolved against the base URL and compared on scheme, host and port before sending. This sits on top of the existing guarded dialer and no-redirect client.

**7. Pagination as a small strategy interface.**
Each style implements "given the previous response, produce the next request or stop". Shared loop enforces the caps (100 pages per endpoint, 10,000 entities per sync), detects a non-advancing cursor or repeated next link, and respects the sync's context deadline. The Link header is parsed for `rel="next"` only.

**8. Fetch pipeline and package layout.**
`custom.Fetch` branches on the presence of `recipe`: absent runs the existing code untouched; present runs `recipe.Run`. New files in `internal/connector/custom`: `recipe.go` (types, parse, validate), `recipe_fetch.go` (requests, auth, pagination), `recipe_map.go` (entities, attributes, dependencies), each with tests using `httptest` servers through the connector test client helpers. The run returns entities, dependencies and per-endpoint statistics; skipped-item counts go into snapshot `Metadata` and a per-endpoint summary section. Raw response bodies are not stored as sections in recipe mode, which keeps snapshots free of unmapped, possibly volatile data.

**9. Any endpoint failure aborts.**
The first failing endpoint returns an error wrapped with the endpoint name through the existing error classifiers. A partial snapshot would make missing entities look deleted.

**10. Category derived from the recipe on the server.**
For type `custom` with a recipe, the connectors create/update handlers and `reconcile` set the row's category from the parsed recipe and reject a conflicting request value. A small helper in the custom package exposes "category for this config"; the registry gains an optional per-type hook so `reconcile` does not import the custom package directly.

**11. Categories widened by migration.**
Paired migration replaces the CHECK constraint with the eight values (table rebuild on sqlite, as `000022` did). `backup.validCategories`, the OpenAPI `ConnectorCategory` enum, `web/src/components/categoryIcon.ts`, the template editor `CATEGORIES` list and the i18n catalogs gain the four values. Sync transformers are keyed by category and need no change: new categories simply have none. Existing connector types are not recategorised.

**12. Preview endpoint.**
`POST /api/connectors/recipe-preview`, admin only, body `{connectorId?, url, verifyTls, config}`. With `connectorId`, omitted secret fields are filled from the stored config. It runs the same `recipe.Run` in a mode that records per-endpoint errors instead of aborting, returns at most 20 sample entities per endpoint, and writes an audit entry `connector.recipe_preview`. Tokens in query strings are redacted from any URL echoed in errors.

**13. Examples as fixtures.**
`docs/connectors/recipes/sonarr.yaml` and `jellyfin.yaml` with recorded, anonymised responses under the package's `testdata`. Tests load the documented files directly so docs and behaviour cannot drift. Uptime Kuma is not an example because its metrics endpoint is Prometheus text, which recipes do not parse.

## Risks / Trade-offs

- [A shared recipe is untrusted input] → it is data only, confined to the connector's origin, GET/POST only, size-capped, and cannot carry credentials or choose where credentials are sent.
- [POST with a static body can still mutate a badly designed API] → documented; the method set is the narrowest that covers GraphQL and RPC read APIs.
- [Query-parameter tokens leak into logs] → a single redaction helper is used for every error, log line and preview that includes a request URL.
- [Volatile values mapped as attributes make every sync look changed] → the recipe reference warns against counters and timestamps, as `SnapshotEntity.Attributes` already requires.
- [Large paginated sources slow syncs] → page and entity caps plus the existing sync timeout.
- [sqlite CHECK change needs a table rebuild] → follow the proven `000022` migration and its down path.

## Migration Plan

One additive paired migration for the category constraint, with a down migration that fails safely if rows use a new category. No data migration for existing custom connectors. Delivered as four PRs: (1) categories, (2) recipe parse/validate/map without pagination, (3) pagination, (4) preview endpoint, web editor, docs and examples. Each is independently mergeable; PR 2 already makes recipes usable through `config.yaml`.
