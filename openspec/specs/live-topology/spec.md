# live-topology Specification

## Purpose

Typed, directional topology relationships between lab entities, exposed through grant-scoped REST endpoints and a live graph page, with traversal shared with MCP.

## Requirements

### Requirement: Typed directional topology edges

The topology builder SHALL persist `resolves_to`, `proxies_to`, and `runs_on` edges alongside `dependency`, `same_as`, and `contains`. Direction SHALL run from the observed DNS record, proxy, or runtime entity toward its resolved owner, upstream, or execution host. Edge detail SHALL preserve labels such as upstream ports.

#### Scenario: DNS record resolves to a visible owner
- **WHEN** a DNS record matches an entity by IP or hostname
- **THEN** the graph stores a directed `resolves_to` edge from the DNS record to that entity

#### Scenario: Removing the DNS record removes the edge
- **WHEN** the DNS connector's snapshot no longer contains the record, or the matched entity's connector re-syncs without that entity or IP
- **THEN** the `resolves_to` edge is removed and no duplicate owned by the other connector remains

#### Scenario: Proxy without a declared upstream
- **WHEN** a proxy entity declares no upstream, or its upstream does not exactly match a dependency
- **THEN** no `proxies_to` edge is stored for it

#### Scenario: Proxy reports an upstream port
- **WHEN** a proxy snapshot reports an upstream and port
- **THEN** the graph stores a directed `proxies_to` edge with the port in edge detail

### Requirement: Authorized topology path API

`GET /api/topology/path` SHALL accept a required `from` name or domain and optional `to`. When `to` is present it SHALL return a shortest path; otherwise it SHALL follow outgoing directed edges from matching start nodes. Only edges whose endpoint connectors are visible to the caller and allowed by the API key SHALL be loaded or traversed.

#### Scenario: Hidden connector separates a path
- **WHEN** a path crosses an edge owned by a connector the caller cannot view
- **THEN** the path response SHALL report no route through that connector

### Requirement: Path steps identify their graph node

Each step of a `GET /api/topology/path` response SHALL carry a `nodeId` equal to the ID the same node has in `GET /api/topology/graph`, so a client can highlight a path on the graph without matching by name. A step that is an active member of an entity identity on a connector the caller may view SHALL report that identity's ID; any other step, such as a connector service node, SHALL report its plain graph node key. The identity lookup SHALL consider only connectors the caller may view.

#### Scenario: Path step maps to a graph identity
- **WHEN** a path step is an entity that resolves to an identity visible in the graph
- **THEN** the step's `nodeId` equals that identity's graph node ID

#### Scenario: Connector service step
- **WHEN** a path step is a connector's own service node
- **THEN** its `nodeId` is the plain graph node key, not an identity ID

### Requirement: Identity-based graph API

`GET /api/topology/graph` SHALL return nodes and edges with entity endpoints resolved to identities through active `entity_members`; connector service nodes and unresolved placeholders SHALL remain plain nodes. Connector and kind filters SHALL be supported, and unlinked entities SHALL be excluded by default. Hidden connector members SHALL not appear by name, count, kind, or traversal.

#### Scenario: Identity spans visible and hidden members
- **WHEN** a visible edge resolves to an identity that also has hidden connector members
- **THEN** the graph SHALL expose only information supported by visible members

#### Scenario: Merged identity members are ignored
- **WHEN** an identity has been merged into another (`merged_into` set)
- **THEN** its members SHALL NOT resolve edge endpoints to it

#### Scenario: Large graphs are bounded
- **WHEN** the visible graph exceeds 2000 nodes or 5000 edges
- **THEN** the response SHALL be cut at the cap and report `truncated: true`

#### Scenario: Unlinked entities are omitted by default
- **WHEN** an entity has no visible edge and `includeUnlinked` is false
- **THEN** the graph SHALL omit it

### Requirement: Stable MCP traversal and topology document refresh

MCP `topology_path` SHALL retain its existing undirected shortest-path behavior after traversal is shared. A sync SHALL regenerate an existing Lab Topology document only when the stored edge set differs from the one the document was generated from, and SHALL NOT create a new document version for an unchanged edge set. A regeneration failure SHALL NOT fail the rebuild and SHALL be retried by a later rebuild.

#### Scenario: Sync rebuild leaves edges unchanged
- **WHEN** a connector sync rebuilds the same topology edge set
- **THEN** the existing Lab Topology document SHALL not be regenerated

#### Scenario: Regeneration fails
- **WHEN** regenerating the Lab Topology document fails after the edges were stored
- **THEN** the rebuild SHALL still succeed and a later rebuild SHALL regenerate the document

### Requirement: Live topology graph page

The `/topology` page SHALL render the graph from `GET /api/topology/graph` as a pan and zoom diagram with an automatic left-to-right layout. Nodes SHALL be entity identities and connector service nodes; unresolved placeholders SHALL NOT be drawn. Edges SHALL be labelled with their kind, plus the detail when present, and styled per kind, with a legend. Several edges between the same ordered pair of nodes SHALL be drawn as one edge whose label lists each kind. The page SHALL provide a text list of the nodes as an alternative to the canvas, and the diagram chrome SHALL follow the application theme. The page SHALL show loading, error with retry, and empty states, and SHALL show a notice when the response reports `truncated`.

#### Scenario: Default view is connected only
- **WHEN** the page opens with no query parameters
- **THEN** the request omits `includeUnlinked` and only identities and connector nodes with at least one visible edge are drawn

#### Scenario: Truncated graph
- **WHEN** the response reports `truncated: true`
- **THEN** the page shows a notice that the graph is limited to the first 2,000 nodes and 5,000 edges

#### Scenario: Parallel edges between the same pair
- **WHEN** two edges of different kinds connect the same source and target
- **THEN** one edge is drawn and its label lists both kinds

#### Scenario: Node navigation
- **WHEN** a user activates an identity node
- **THEN** the app navigates to that entity's page
- **WHEN** a user activates a connector node
- **THEN** the app navigates to that connector's document, or to its service page when it has none

### Requirement: Topology filters and URL state

The page SHALL offer a connector filter, a kind filter chosen from the kinds present in the unfiltered graph, and a show-unlinked toggle, each mapped to the `connector`, `kind` and `includeUnlinked` query parameters of the graph endpoint. Filter and trace state SHALL live in the page URL so a view can be shared and restored. Changing a filter SHALL replace the current history entry and SHALL NOT issue a request per keystroke. When the filters leave no nodes the page SHALL say that no topology matches them and offer a control that clears the filters, instead of the "no topology yet" state.

#### Scenario: Filters are written to the URL
- **WHEN** a user picks a connector and a kind and enables show-unlinked
- **THEN** the URL carries `connector`, `kind` and `includeUnlinked=true`, and the graph request uses the same values

#### Scenario: View restored from a URL
- **WHEN** the page opens with `connector`, `kind`, `includeUnlinked`, `from` and `to` in the URL
- **THEN** the controls show those values and the graph and trace are requested with them

#### Scenario: Kind options survive a kind filter
- **WHEN** a kind filter narrows the response
- **THEN** the kind control still lists every kind from the unfiltered graph

#### Scenario: Filters match nothing
- **WHEN** the filters yield no nodes
- **THEN** the page shows a no-matches state with a clear-filters action, and clearing them restores the graph

### Requirement: Trace highlighting

The page SHALL provide a trace form with a required `from` endpoint and an optional `to` endpoint, submitted with the Enter key or the trace button, that calls `GET /api/topology/path`. With both endpoints the page SHALL highlight the shortest path between them; with only `from` it SHALL highlight the directed walk that follows outgoing edges from it. The returned hops SHALL be listed in order. Endpoints SHALL be validated (`from` required, at most 256 characters each) before a request is sent, and a no-path result, a validation error from the server, a request failure, or a truncated walk SHALL each be reported without hiding the graph. Highlighting SHALL NOT recompute the graph layout.

#### Scenario: Path between two endpoints
- **WHEN** a user submits both endpoints and a path is found
- **THEN** the edges between consecutive hops are highlighted, no other edge is, and the hops are listed

#### Scenario: Walk from one endpoint
- **WHEN** a user submits only `from`
- **THEN** the request omits `to` and the returned directed walk is highlighted and listed

#### Scenario: Missing start
- **WHEN** a user submits the form with no `from`
- **THEN** a validation message is shown and no path request is made

#### Scenario: No path
- **WHEN** the response reports `found: false`
- **THEN** the page says no path was found and the graph stays visible

### Requirement: Mermaid export for instance admins

The page SHALL keep the Mermaid Lab Topology document as an export. An export control that calls `POST /docs/topology` and opens the returned document SHALL be shown to instance admins only, and a failed export SHALL show an error message.

#### Scenario: Non-admin
- **WHEN** a user who is not an instance admin opens the page
- **THEN** the export control is not rendered

#### Scenario: Admin export
- **WHEN** an instance admin activates the export control
- **THEN** the app opens the generated document

#### Scenario: Export fails
- **WHEN** the export request fails
- **THEN** the page shows an error and stays on the topology page
