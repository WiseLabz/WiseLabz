# Custom REST recipe format

A custom connector accepts a `recipe` YAML string in its configuration. It maps
JSON responses to entities, attributes and service dependencies. A recipe is
shareable data: put credentials only in the connector's separate
`auth_token`, `auth_username` and `auth_password` fields, all encrypted at rest.
The recipe itself is a non-secret `textarea` field returned by the API and
included unchanged in backups. When the connector is declared in `config.yaml`,
the recipe text is used verbatim and `${VAR}` references inside it are not
expanded. Everything in a recipe, including static
`query`, `headers` and `body` values, is stored unencrypted and readable by
every user who can view the connector. See the [declared connector example](../CONNECTORS_IN_CONFIG.md#custom-rest-recipes).

## Document and validation

Version 1 requires `version: 1`, `category`, `auth` and between 1 and 20
`endpoints`. The UTF-8 document may occupy at most 64 KiB. Categories are
`virtualization`, `containers_paas`, `networking`, `dns`, `storage`,
`monitoring`, `media` and `other`. The server derives the connector category
from the recipe and rejects a conflicting category supplied by an API caller.
A custom connector without a recipe uses `virtualization`.

Unknown keys, invalid values and malformed paths are rejected before requests.
Errors are collected together and carry dotted locations such as
`endpoints[1].entity.external_id`. Validation runs when saving, declaring,
fetching or testing the connector. YAML contains no executable expressions.

YAML anchors and aliases are allowed only while the expanded document stays
within the 64 KiB limit; a recipe whose aliases expand beyond it is rejected.
Merge keys (`<<`) and duplicate mapping keys are rejected.

## Authentication

`auth.mode` is one of:

| Mode | Recipe fields | Required connector credentials |
|---|---|---|
| `none` | none | none |
| `header` | `name` is the header; optional `prefix` precedes the token verbatim | `auth_token` |
| `basic` | none | `auth_username`, `auth_password` |
| `query` | `name` is the query parameter | `auth_token` |

For example, header mode can use `name: Authorization` and `prefix: "Bearer "`.
The legacy encrypted `headers` JSON field still applies to every request.
Query authentication tokens are redacted in errors and sync messages.

## Endpoints

Each endpoint requires a unique `name`, a `path` relative to the connector's
URL, a `method` of `GET` or `POST`, an `items` path and an `entity` mapping.
`query` and `headers` are optional maps of static string values. `body` is a
static JSON value for POST, which sends JSON with `Content-Type: application/json`.
GET permits an omitted or null body only. POST covers read APIs such as GraphQL and RPC;
choose service endpoints whose requests only read data.

Paths may begin with `/` but cannot contain a URL scheme or begin with `//`.
Each request is resolved against the connector URL and checked for the same
scheme, hostname and port. A path starting with `/` replaces the path of the
connector URL, while a path without a leading `/` is resolved relative to it.
A service behind a reverse-proxy sub-path therefore needs a connector URL that
ends in `/` (for example `https://host/prefix/`) and relative endpoint paths
(`api/items`). Redirects are refused, loopback and link-local
blocking still applies, and responses use the shared body size limit.

## Pagination

An endpoint without `pagination` makes exactly one request. With pagination,
the configured authentication, legacy headers, endpoint headers and static
body are applied to every request. Page-number and offset pagination stop when
`items` is an empty list. Cursor pagination stops when its response path is
missing, null or an empty string. Next-link pagination stops when its body path
is missing or empty, or when the `Link` header has no link with a `next` relation.

Page numbers start at 1 when `start` is omitted; offsets start at 0. Set
`start` to 0 for an API with zero-based page numbers. `size` is required for
page and offset pagination; `size_param` sends that size to the API. Offset
pagination requires both `param` and `size_param` and advances the offset by
`size`. A page or offset request replaces a same-named static query parameter
with the current paging value.

Cursor pagination sends each response cursor as the next value of `param`. An
initial cursor may come from the connector URL, the endpoint path or the
endpoint's static `query`. A cursor equal to one already sent fails the sync.
The response's `cursor_path` is evaluated against the complete JSON response.
Cursor APIs may also declare `size_param` and a positive `size`; the size is
sent on the initial and subsequent cursor requests.

For `next_link`, choose exactly one source: a body `next_path`, or
`link_header: true`. A body path must resolve to a URL string. Header parsing
uses links whose relation contains the `next` token; it accepts multiple Link
values and relation values such as `rel="prev next"`. Relative and absolute
next URLs are resolved against the connector URL. Every next URL must retain
the connector's scheme, hostname and port; cross-origin links, URL user
information and fragments fail before a request is sent. Repeating a next
URL also fails. For every next link, the connector URL's query, the endpoint
path's query and the endpoint's static `query` are applied again, in that
order; a parameter present in the link replaces the same-named value from
those sources. Query-token authentication is applied once on every page and
cannot be replaced by a paging or link query parameter.

The runner reads at most 100 pages per endpoint. When the 100th page still
announces another page, a single extra request confirms the end of the data:
the sync fails if that response returns any items or announces a further page,
and succeeds if it is empty and ends pagination. It also fails if the complete
sync maps more than 10,000 entities or more than the existing 10 MiB
mapped-output budget. A cursor or next link longer than 8 KiB (8192 bytes)
also fails the sync. A cap failure returns no snapshot.

These complete recipes demonstrate each pagination form and are parsed by the
custom connector tests:

<!-- pagination-recipes-start -->

### Page number

```yaml
version: 1
category: media
auth: {mode: none}
endpoints:
  - name: items
    path: /api/items
    method: GET
    items: items
    pagination: {type: page, param: page, size_param: pageSize, size: 100, start: 1}
    entity: {kind: media_item, name: name, external_id: id}
```

### Offset

```yaml
version: 1
category: media
auth: {mode: none}
endpoints:
  - name: items
    path: /api/items
    method: GET
    items: items
    pagination: {type: offset, param: offset, size_param: limit, size: 100}
    entity: {kind: media_item, name: name, external_id: id}
```

### Cursor

```yaml
version: 1
category: media
auth: {mode: none}
endpoints:
  - name: items
    path: /api/items
    method: GET
    items: items
    pagination: {type: cursor, param: cursor, cursor_path: nextCursor}
    entity: {kind: media_item, name: name, external_id: id}
```

### Next link from the body

```yaml
version: 1
category: media
auth: {mode: none}
endpoints:
  - name: items
    path: /api/items
    method: GET
    items: items
    pagination: {type: next_link, next_path: paging.next}
    entity: {kind: media_item, name: name, external_id: id}
```

### Next link from the Link header

```yaml
version: 1
category: media
auth: {mode: none}
endpoints:
  - name: items
    path: /api/items
    method: GET
    items: items
    pagination: {type: next_link, link_header: true}
    entity: {kind: media_item, name: name, external_id: id}
```

<!-- pagination-recipes-end -->

An endpoint transport error, non-success HTTP status, invalid JSON, oversized
response, `items` path that does not select a list or mapped entities and
attributes larger than the response size limit aborts the complete sync.
The error names the endpoint, and for a paginated endpoint also the page, for
example `endpoint "items": page 3: ...`. No partial snapshot is stored; authentication
and availability errors retain their connector error classifications. A
connection test requests only the first endpoint once with the configured auth.

## Paths and entities

Paths use [GJSON syntax](https://github.com/tidwall/gjson#path-syntax): `items`
selects a field, `@this` selects the root, and `items.#.name` projects names
from an array. Only the modifiers `@this`, `@reverse`, `@flatten`, `@join`,
`@keys` and `@values` are accepted; a key containing `@` must be escaped as
`\@`. Modifier arguments (`@name:...`) are not accepted. A path may contain at
most one multipath group (`[a,b]` or `{a,b}`) selecting at most 8 values. An
`@`, brace, bracket or comma that is data must be escaped with a backslash, or
written as a JSON unicode escape (`\u0040` for `@`) inside a quoted query
value. A path expression is limited to 1024 bytes. `items` selects the response
list. Entity paths are evaluated
against each item; dependency paths are evaluated against the response root.

`entity.kind` is a fixed string. `entity.name` and `entity.external_id` are
required paths. `ip`, `hostname`, `mac` and `aliases` are optional paths; an
aliases path may select several strings. Use a stable ID so a name change
modifies an entity instead of removing and adding it. Missing or empty IDs
skip that item; skipped counts are recorded per endpoint in snapshot metadata
and summary sections. Duplicate IDs for the same kind across the complete
sync fail it. Raw responses are not stored as sections in recipe mode.

## Attributes

`entity.attributes` maps an attribute name to a definition. Exactly one source
is required: `path`, `const`, or `template`. A missing path, or a missing path
inside a template, omits the attribute.

Templates substitute `{path}` placeholders, for example `{host}:{port}`.
Use `{{` and `}}` for literal braces. They compose strings only.

An optional `map` replaces values by their string form before conversion;
`default` supplies a value for an unmatched map key. An optional `type`
converts strictly to a string, number, boolean (`bool`), or list of strings (`list`).
Conversion failure aborts the sync and names the endpoint and attribute.

Examples of attribute definitions:

```yaml
monitored: {path: monitored, type: bool}
status: {path: status, map: {"1": enabled, "0": disabled}, default: unknown}
address: {template: "{host}:{port}"}
source: {const: library}
```

Avoid volatile counters and timestamps as attributes: changing them on every
sync creates changes without describing configuration changes.

## Dependencies

`dependencies` can appear at the recipe root or on an endpoint. Each definition
has `kind` (`host`, `network`, `storage`, `upstream_service`) and exactly one
name source: `const` or `path`. Root path definitions are evaluated against each endpoint response; endpoint
paths are evaluated against that endpoint's complete response. Multi-value paths
produce one dependency per distinct name. Duplicate kind/name pairs are
removed and dependencies are stored with the snapshot.
