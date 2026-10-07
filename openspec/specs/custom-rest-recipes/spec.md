# custom-rest-recipes Specification

## Purpose
Lets an administrator document any JSON REST service by describing, in a shareable recipe, which endpoints to read and how their responses map to entities, without writing connector code.

## Requirements

### Requirement: Recipe document
A custom connector SHALL accept an optional recipe written in YAML and stored as part of the connector's configuration, so that it is available to connectors created in the web app and to connectors declared in configuration, and is included in backups. A recipe SHALL declare a format version, a category, an authentication mode and between 1 and 20 endpoints. A recipe SHALL NOT contain credentials. A recipe larger than 64 KiB SHALL be rejected.

#### Scenario: Recipe declared in configuration
- **WHEN** a custom connector with a valid recipe is declared in the configuration file
- **THEN** it SHALL be reconciled and synced using that recipe.

#### Scenario: Recipe round-trips through backup
- **WHEN** a custom connector with a recipe is exported in a backup and imported into another instance
- **THEN** the imported connector SHALL have the identical recipe.

### Requirement: Recipe validation
The system SHALL validate a recipe when the connector is saved, when it is declared in configuration and when it is tested, and SHALL reject an invalid recipe without saving it. Validation SHALL report every problem found, each with the location in the recipe it refers to. Unknown keys, an unsupported version, an unknown category, an unknown authentication mode, an unsupported method, a malformed path expression, duplicate endpoint names and an endpoint without an entity `name` or `external_id` mapping SHALL each be validation errors.

#### Scenario: Unknown key
- **WHEN** a recipe contains a key that the format does not define
- **THEN** saving SHALL be rejected with an error naming that key and where it appears.

#### Scenario: Missing external ID mapping
- **WHEN** an endpoint maps entities without an `external_id`
- **THEN** saving SHALL be rejected with an error naming that endpoint.

#### Scenario: Several problems at once
- **WHEN** a recipe has an unsupported method on one endpoint and a malformed path on another
- **THEN** both errors SHALL be reported in one response.

#### Scenario: Invalid recipe in configuration
- **WHEN** a declared custom connector has an invalid recipe
- **THEN** that connector SHALL be reported as invalid with the recipe errors and other declared connectors SHALL be unaffected.

### Requirement: Authentication modes
A recipe SHALL select one authentication mode: none, a token sent in a named request header with an optional prefix, HTTP basic, or a token sent in a named query parameter. The token, username and password SHALL be supplied in separate connector fields that are encrypted at rest and never returned in clear by the API. The legacy headers field SHALL continue to be applied when present.

#### Scenario: Header token
- **WHEN** a recipe selects header authentication with header `X-Api-Key` and the connector has a token
- **THEN** every recipe request SHALL carry that header with the token.

#### Scenario: Query token
- **WHEN** a recipe selects query authentication with parameter `apikey`
- **THEN** every recipe request SHALL carry that parameter, and the token SHALL NOT appear in logs, errors, sync messages or preview output.

#### Scenario: Missing credential
- **WHEN** a recipe selects basic authentication and the connector has no username
- **THEN** saving the connector SHALL be rejected with a field error on the username.

### Requirement: Endpoint requests
Each endpoint SHALL define a path relative to the connector's URL and a method of GET or POST. A POST endpoint SHALL send the static JSON body given in the recipe. No other method SHALL be accepted. Each endpoint MAY define static query parameters and static headers.

#### Scenario: POST with static body
- **WHEN** an endpoint declares POST with a JSON body
- **THEN** the request SHALL be a POST carrying exactly that body with a JSON content type.

#### Scenario: Unsupported method
- **WHEN** an endpoint declares method DELETE
- **THEN** the recipe SHALL be rejected at validation.

### Requirement: Same-origin requests
Every request a recipe makes SHALL target the same scheme, host and port as the connector's URL. An endpoint path SHALL NOT be an absolute URL. A pagination link that points to a different origin SHALL fail the sync without the request being sent. Redirects SHALL NOT be followed. The existing blocking of loopback and link-local targets SHALL apply.

#### Scenario: Absolute endpoint URL
- **WHEN** an endpoint path is `https://other.example/api/items`
- **THEN** the recipe SHALL be rejected at validation.

#### Scenario: Next link to another host
- **WHEN** a response's next link points to a different host than the connector's URL
- **THEN** the sync SHALL fail with an error naming the endpoint, and no request SHALL be sent to that host.

### Requirement: Pagination
An endpoint MAY declare one pagination style: page number, offset, cursor taken from the response, or next link taken from the response body or from the `Link` response header. The system SHALL request pages until the style's end condition is met: an empty page of items, an absent or empty cursor, or an absent next link. The system SHALL stop with a sync failure if an endpoint exceeds 100 pages or the recipe yields more than 10,000 entities in one sync. An endpoint without pagination SHALL make exactly one request.

#### Scenario: Page-number pagination
- **WHEN** an endpoint uses page-number pagination and the third page returns no items
- **THEN** entities from pages one and two SHALL be mapped and no fourth request SHALL be made.

#### Scenario: Cursor pagination
- **WHEN** an endpoint uses cursor pagination and a response carries no cursor
- **THEN** that response SHALL be the last page.

#### Scenario: Link header pagination
- **WHEN** an endpoint uses next-link pagination from the `Link` header and a response has a `rel="next"` link on the same origin
- **THEN** the system SHALL request that link next.

#### Scenario: Page cap exceeded
- **WHEN** an endpoint still returns items on its 101st page
- **THEN** the sync SHALL fail with an error stating the page limit and no snapshot SHALL be stored.

#### Scenario: Cursor does not advance
- **WHEN** a response returns the same cursor that was just sent
- **THEN** the sync SHALL fail instead of requesting the same page again.

### Requirement: Entity mapping
Each endpoint SHALL declare a path selecting the list of items in the response, a fixed entity kind, and path expressions for the entity name and external ID. It MAY map IP address, hostname, MAC address and aliases. Each item SHALL produce one entity. An item whose external ID resolves to an empty value SHALL be skipped, and the number of skipped items per endpoint SHALL be reported with the sync result. Two items of the same kind with the same external ID in one sync SHALL fail the sync.

#### Scenario: Items mapped
- **WHEN** an endpoint returns three items with distinct identifiers
- **THEN** the snapshot SHALL contain three entities of the declared kind with the mapped names and external IDs.

#### Scenario: Item without identifier
- **WHEN** one of three items has no value at the external ID path
- **THEN** two entities SHALL be produced and the sync result SHALL report one skipped item for that endpoint.

#### Scenario: Renamed item keeps identity
- **WHEN** an item's name changes between two syncs while its external ID stays the same
- **THEN** the change SHALL be recorded as a modification of one entity, not a removal and an addition.

#### Scenario: Duplicate identifier
- **WHEN** two items of the same kind resolve to the same external ID
- **THEN** the sync SHALL fail with an error naming the endpoint and the identifier.

### Requirement: Attribute mapping
An endpoint MAY map any number of named attributes. An attribute's value SHALL come from exactly one of: a path expression, a constant, or a string template that composes several paths with literal text. An attribute MAY declare a type of string, number, boolean or list of strings, and the value SHALL be converted to it. An attribute MAY declare a value map that replaces a resolved value with a configured one, with an optional default for unmapped values. An attribute whose path resolves to nothing SHALL be omitted from the entity. A value that cannot be converted to the declared type SHALL fail the sync with an error naming the endpoint and attribute.

#### Scenario: Typed attribute
- **WHEN** an attribute with type boolean resolves to the JSON value `true`
- **THEN** the entity attribute SHALL be the boolean true.

#### Scenario: Value map
- **WHEN** an attribute maps `1` to `enabled` and `0` to `disabled` and an item has the value `1`
- **THEN** the entity attribute SHALL be `enabled`.

#### Scenario: Unmapped value with default
- **WHEN** a value map has a default of `unknown` and an item has a value that is not in the map
- **THEN** the entity attribute SHALL be `unknown`.

#### Scenario: String template
- **WHEN** an attribute uses the template `{host}:{port}` and an item has host `nas` and port `8096`
- **THEN** the entity attribute SHALL be `nas:8096`.

#### Scenario: Missing path
- **WHEN** an attribute's path does not exist in an item
- **THEN** that entity SHALL have no such attribute and the sync SHALL succeed.

#### Scenario: Conversion failure
- **WHEN** an attribute with type number resolves to the string `abc`
- **THEN** the sync SHALL fail with an error naming the endpoint and the attribute.

### Requirement: Dependency mapping
A recipe MAY declare service dependencies, each with a kind of host, network, storage or upstream service and a name that is either a constant or a path expression evaluated against an endpoint's response. A path that resolves to several values SHALL produce one dependency per distinct value. Dependencies SHALL be part of the stored snapshot.

#### Scenario: Constant dependency
- **WHEN** a recipe declares a storage dependency with the constant name `tank`
- **THEN** the snapshot SHALL contain a storage dependency named `tank`.

#### Scenario: Dependencies from a response
- **WHEN** a dependency of kind upstream service takes its name from a path that resolves to `qbittorrent`, `sabnzbd` and `qbittorrent`
- **THEN** the snapshot SHALL contain two upstream service dependencies, `qbittorrent` and `sabnzbd`.

### Requirement: All-or-nothing sync
If any endpoint of a recipe fails, by transport error, authentication error, non-success status, a response that is not valid JSON, an items path that does not resolve to a list, or a response over the size limit, the sync SHALL fail, no snapshot SHALL be stored, and the connector SHALL report an error that names the failing endpoint. Authentication and unavailability failures SHALL be classified as they are for other connectors.

#### Scenario: One endpoint down
- **WHEN** a recipe has two endpoints and the second returns status 500
- **THEN** the sync SHALL fail naming the second endpoint, and the previously stored snapshot and its entities SHALL remain unchanged.

#### Scenario: Items path not a list
- **WHEN** an endpoint's items path resolves to an object
- **THEN** the sync SHALL fail with an error naming the endpoint and the path.

#### Scenario: Rejected credentials
- **WHEN** an endpoint returns status 401
- **THEN** the sync SHALL fail as an authentication error.

### Requirement: Recipe-less compatibility
A custom connector without a recipe SHALL behave as before this change: one request to its URL with the configured method and headers, the raw response stored as a section, and entities taken from a top-level `entities` array when present. No existing custom connector SHALL require reconfiguration.

#### Scenario: Existing custom connector after upgrade
- **WHEN** a custom connector created before this change syncs after the upgrade
- **THEN** its snapshot SHALL contain the same sections and entities as before.

### Requirement: Connection test with a recipe
Testing the connection of a custom connector that has a recipe SHALL request the first endpoint once with the configured authentication and SHALL report success, an authentication error or an availability error accordingly.

#### Scenario: Wrong token
- **WHEN** the connection is tested with a token the service rejects
- **THEN** the test SHALL report an authentication error.

### Requirement: Recipe test preview
Administrators SHALL be able to test a recipe, saved or not, against the target service and receive, per endpoint, the number of items, the number skipped, a sample of mapped entities with their attributes, the mapped dependencies, and any error. A preview SHALL NOT store a snapshot, create changes or alerts, or alter the connector. A preview of an existing connector SHALL be able to use its stored credentials without the client resending them. Preview output SHALL NOT contain credentials. Non-administrators SHALL be refused. Each preview SHALL be audited.

#### Scenario: Preview before saving
- **WHEN** an administrator previews a new recipe with a URL and token
- **THEN** the response SHALL list mapped sample entities per endpoint and no connector, snapshot or change SHALL be created.

#### Scenario: Preview with an endpoint error
- **WHEN** one endpoint of the previewed recipe fails
- **THEN** the response SHALL show that endpoint's error and still show the results of the endpoints that succeeded.

#### Scenario: Preview using stored credentials
- **WHEN** an administrator previews an edited recipe for an existing connector without sending the token
- **THEN** the preview SHALL authenticate with the stored token.

#### Scenario: Non-admin
- **WHEN** a non-administrator requests a preview
- **THEN** the request SHALL be rejected with status 403.

### Requirement: Documented example recipes
The project SHALL ship a recipe format reference and at least two example recipes for real services. Each example SHALL be verified by an automated test that maps recorded responses of that service to the expected entities.

#### Scenario: Example stays valid
- **WHEN** the recipe format changes in a way that breaks a shipped example
- **THEN** the example's automated test SHALL fail.
