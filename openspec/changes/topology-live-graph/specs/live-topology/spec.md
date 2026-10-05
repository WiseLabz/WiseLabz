# Spec Delta

## ADDED Requirements

### Requirement: Typed directional topology edges

The topology builder SHALL persist `resolves_to`, `proxies_to`, and `runs_on` edges alongside `dependency`, `same_as`, and `contains`. Direction SHALL run from the observed DNS record, proxy, or runtime entity toward its resolved owner, upstream, or execution host. Edge detail SHALL preserve labels such as upstream ports.

#### Scenario: DNS record resolves to a visible owner
- **WHEN** a DNS record matches an entity by IP or hostname
- **THEN** the graph stores a directed `resolves_to` edge from the DNS record to that entity

#### Scenario: Proxy reports an upstream port
- **WHEN** a proxy snapshot reports an upstream and port
- **THEN** the graph stores a directed `proxies_to` edge with the port in edge detail

### Requirement: Authorized topology path API

`GET /api/topology/path` SHALL accept a required `from` name or domain and optional `to`. When `to` is present it SHALL return a shortest path; otherwise it SHALL follow outgoing directed edges from matching start nodes. Only edges whose endpoint connectors are visible to the caller and allowed by the API key SHALL be loaded or traversed.

#### Scenario: Hidden connector separates a path
- **WHEN** a path crosses an edge owned by a connector the caller cannot view
- **THEN** the path response SHALL report no route through that connector

### Requirement: Identity-based graph API

`GET /api/topology/graph` SHALL return nodes and edges with entity endpoints resolved to identities through active `entity_members`; connector service nodes and unresolved placeholders SHALL remain plain nodes. Connector and kind filters SHALL be supported, and unlinked entities SHALL be excluded by default. Hidden connector members SHALL not appear by name, count, kind, or traversal.

#### Scenario: Identity spans visible and hidden members
- **WHEN** a visible edge resolves to an identity that also has hidden connector members
- **THEN** the graph SHALL expose only information supported by visible members

#### Scenario: Unlinked entities are omitted by default
- **WHEN** an entity has no visible edge and `includeUnlinked` is false
- **THEN** the graph SHALL omit it

### Requirement: Stable MCP traversal and topology document refresh

MCP `topology_path` SHALL retain its existing undirected shortest-path behavior after traversal is shared. A sync SHALL regenerate an existing Lab Topology document only when its rebuilt edge set changes, and SHALL NOT create a new document version for an unchanged edge set.

#### Scenario: Sync rebuild leaves edges unchanged
- **WHEN** a connector sync rebuilds the same topology edge set
- **THEN** the existing Lab Topology document SHALL not be regenerated
