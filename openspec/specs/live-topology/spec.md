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

### Requirement: Symmetric same_as edges are complete and order independent

`same_as` links two entities from different connectors that match by external ID, hostname or alias, or IP address. Because the relation is symmetric, each edge SHALL be stored in one canonical direction (ordered by connector, kind and reference) whichever endpoint's connector derived it, and every matching pair SHALL be stored, not only the first match per entity. After any order of syncs of the connectors involved the stored `same_as` edge set SHALL be the same. Which entities merge into one identity SHALL NOT depend on this requirement; identity clustering keeps its own strong-match rules.

Strong matches (external ID or hostname) SHALL always be complete. IP-only matches SHALL be complete while at most 50 entities, counted across all connectors, share one IP address. When more entities share one IP, only the first 50 of them in stable order (by connector ID, kind and reference) take part in IP-only `same_as` edges: a pair is stored only when both entities are inside that subset, so the subset is the same whichever connector is rebuilt and the edge count per IP is bounded at 50 x 50 per rebuild. The rebuild SHALL log one warning that names the cap, and strong matches between entities outside the subset SHALL still be stored. This bounds the rows and the time spent under the global rebuild lock at the price of dropping IP-based `same_as` edges for the entities beyond the cap.

Edges SHALL be written to the database in multi-row batches.

#### Scenario: Sync order does not change same_as edges
- **WHEN** three connectors report entities sharing one IP and are synced in any order
- **THEN** the stored `same_as` edges are exactly one per cross-connector pair, in one canonical direction each

#### Scenario: IP shared at or below the cap
- **WHEN** exactly 50 entities across two connectors share one IP
- **THEN** every cross-connector pair has a `same_as` edge

#### Scenario: IP shared above the cap
- **WHEN** more than 50 entities share one IP
- **THEN** only pairs inside the first 50 entities in stable order are stored, the same pairs after a sync of either connector in any order, and one warning is logged per rebuild
- **AND** a strong match between entities outside that subset is still stored

### Requirement: Directed traversal treats same_as as symmetric

A directed walk (`GET /api/topology/path` with only `from`) SHALL cross a `same_as` edge from either end regardless of its stored direction. All other kinds SHALL be followed only in their stored direction. A step reached over a `same_as` edge against its stored direction SHALL carry `edgeReversed`.

#### Scenario: Walk starts at the far end of a same_as edge
- **WHEN** a directed walk starts at the entity a `same_as` edge points to
- **THEN** the walk reaches the other end of that edge

### Requirement: Authorized topology path API

`GET /api/topology/path` SHALL accept a required `from` name or domain and optional `to`. When `to` is present it SHALL return a shortest path; otherwise it SHALL follow outgoing directed edges from matching start nodes. Only edges whose endpoint connectors are visible to the caller and allowed by the API key SHALL be loaded or traversed.

#### Scenario: Hidden connector separates a path
- **WHEN** a path crosses an edge owned by a connector the caller cannot view
- **THEN** the path response SHALL report no route through that connector

### Requirement: Path steps identify their graph node

Each step of a `GET /api/topology/path` response SHALL carry a `nodeId` equal to the ID the same node has in `GET /api/topology/graph`, so a client can highlight a path on the graph without matching by name. A step that is an active member of an entity identity on a connector the caller may view SHALL report that identity's ID; any other step, such as a connector service node, SHALL report its plain graph node key. The identity lookup SHALL consider only connectors the caller may view.

Each step after a start node SHALL also carry `fromNodeId`, the graph node ID of the step it was reached from, resolved exactly like `nodeId` (active members on connectors the caller may view only, else the plain graph node key), and `edgeReversed` when its edge was traversed against its direction (only possible for the undirected path to a target). With `edgeKind`, `edgeSource` and `detail` these identify the edge each step followed. MCP `topology_path` output SHALL NOT include `fromNodeId` or `edgeReversed`.

#### Scenario: Path step maps to a graph identity
- **WHEN** a path step is an entity that resolves to an identity visible in the graph
- **THEN** the step's `nodeId` equals that identity's graph node ID

#### Scenario: From node never names a hidden entity
- **WHEN** a step was reached from a node whose only identity member is on a connector the caller cannot view
- **THEN** its `fromNodeId` is the plain graph node key, never that entity's ID

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

MCP `topology_path` SHALL retain its existing undirected shortest-path behavior after traversal is shared. A sync SHALL regenerate an existing generated Lab Topology document only when the drawn edge kinds (`same_as`, `resolves_to`, `proxies_to`, `runs_on`) differ from the ones the document was generated from, and SHALL NOT create a new document version for an unchanged drawing. Edges the diagram does not draw (`contains`, `dependency`) SHALL NOT make the document stale. The fingerprint of the edge set a document was rendered from SHALL be stored beside the document (`docs.topology_fingerprint`), never inside its content, so no save, restore, export or AI proposal of the content can lose it and no served content carries it. When a rebuild finds the drawing unchanged but the stored fingerprint different, it SHALL update only the fingerprint, without a new version. Content that still carries the legacy `<!-- wl:topology-edges:... -->` comment SHALL have it removed in place, without a new version, by the next regeneration check; until then the doc and version endpoints SHALL omit it. A regeneration failure SHALL NOT fail the rebuild and SHALL be retried by a later rebuild. Human-origin documents titled "Lab Topology" SHALL never be regenerated.

#### Scenario: Sync rebuild leaves edges unchanged
- **WHEN** a connector sync rebuilds the same topology edge set
- **THEN** the existing Lab Topology document SHALL not be regenerated

#### Scenario: Regeneration fails
- **WHEN** regenerating the Lab Topology document fails after the edges were stored
- **THEN** the rebuild SHALL still succeed and a later rebuild SHALL regenerate the document

#### Scenario: Only a typed edge changes
- **WHEN** a sync changes only a `proxies_to`, `resolves_to` or `runs_on` edge
- **THEN** the document gains exactly one new version

#### Scenario: Only an undrawn edge changes
- **WHEN** a sync changes only `contains` or `dependency` edges
- **THEN** the document gains no version, and the next rebuild does not re-render it

#### Scenario: A user saves the generated document
- **WHEN** a user saves the Lab Topology document from the editor and a sync follows
- **THEN** the sync does not re-render the document or add a version, because the fingerprint is not part of the saved content

#### Scenario: Fingerprint differs but drawing is the same
- **WHEN** the stored fingerprint differs from the current edges but the rendered content is equal
- **THEN** only the fingerprint is updated, with no new version

#### Scenario: Legacy marker in stored content
- **WHEN** a document written before the fingerprint moved still contains the marker comment
- **THEN** the next regeneration check removes it in place without a new version

### Requirement: Lab Topology document draws typed edges safely

The generated Lab Topology Mermaid document SHALL draw one arrow per distinct `resolves_to`, `proxies_to` and `runs_on` edge from the source to the target, labelled with the kind and, when present, the detail (for example `proxies_to :8080`), in addition to the `same_as` links. An endpoint with no entity of its own SHALL get a node of its own. Every node and edge label SHALL be emitted through one helper that produces a single-line quoted Mermaid string: quotes, angle brackets, square brackets, pipes, backticks, `%`, `#` and line breaks in connector-supplied names, kinds and details SHALL be neutralised with Mermaid entity codes or spaces, so such text cannot end the label, forge nodes or edges, add `click` or `href` directives, or close the code fence. The web Mermaid renderer SHALL keep its default strict security level.

#### Scenario: Hostile name does not alter the diagram
- **WHEN** a connector reports a name or detail such as `x"] --> evil["y` or one containing a newline and a `click` directive
- **THEN** the diagram has exactly the nodes and edges it would have for a benign name, and the text appears only inside its label

#### Scenario: Typed edge is labelled
- **WHEN** a proxy forwards to an upstream on port 8080
- **THEN** the diagram draws an arrow labelled `proxies_to :8080`

### Requirement: Live topology graph page

The `/topology` page SHALL render the graph from `GET /api/topology/graph` as a pan and zoom diagram with an automatic left-to-right layout. Nodes SHALL be entity identities and connector service nodes; unresolved placeholders SHALL NOT be drawn. Edges SHALL be labelled with their kind, plus the detail when present, and styled per kind, with a legend. Several edges between the same ordered pair of nodes SHALL be drawn as one edge whose label lists each kind. The label of a `contains` edge SHALL be drawn only while the edge is highlighted by a trace, because those edges fan out from every connector; its kind stays in the legend and in the edge's accessible name. Each edge SHALL have an accessible name built from a translated template (`docs.topology.edgeLabel`, in every shipped locale) that names the source node, the label (kinds and details) and the target node, never raw IDs. The page SHALL provide a text list of the nodes as an alternative to the canvas, and the diagram chrome SHALL follow the application theme. The page SHALL show loading, error with retry, and empty states, and SHALL show a notice when the response reports `truncated`.

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
- **WHEN** a user clicks or activates an identity node
- **THEN** the app navigates to that entity's page
- **WHEN** a user clicks or activates a connector node
- **THEN** the app navigates to that connector's document, or to its service page when it has none

#### Scenario: Nodes receive pointer events
- **WHEN** the graph is rendered
- **THEN** no node is `pointer-events: none`, so a real mouse click reaches the node's link

#### Scenario: One tab stop per node
- **WHEN** a user tabs through the graph
- **THEN** each node is a single tab stop (its link); the node wrapper is not focusable

#### Scenario: Edge accessible name
- **WHEN** an edge runs from "Host Alpha" to "VM Beta" with kind `runs_on` and detail `vmid 7`
- **THEN** its accessible name is built from the locale's template, in English "Host Alpha runs_on · vmid 7 VM Beta", and contains no ID

#### Scenario: Contains labels while tracing
- **WHEN** no trace is active
- **THEN** `contains` edges are drawn without a text label
- **WHEN** a trace highlights a `contains` edge
- **THEN** its label is shown

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

The page SHALL provide a trace form with a required `from` endpoint and an optional `to` endpoint, submitted with the Enter key or the trace button, that calls `GET /api/topology/path`. With both endpoints the page SHALL highlight the shortest path between them; with only `from` it SHALL highlight the directed walk that follows outgoing edges from it. In both cases the page SHALL highlight exactly the edges the steps followed, identified by the step's `fromNodeId`, `nodeId`, `edgeReversed`, `edgeKind`, `edgeSource` and `detail`, never an edge merely between two consecutive steps; where both directions or several kinds exist between a pair only the traversed one counts, and a hop through a node the page does not draw highlights nothing. The returned hops SHALL be listed in order. Endpoints SHALL be validated (`from` required, at most 256 characters each) before a request is sent, and a no-path result, a validation error from the server, a request failure, or a truncated walk SHALL each be reported without hiding the graph. Highlighting SHALL NOT recompute the graph layout or refit the viewport, but the viewport SHALL refit whenever the set of displayed nodes changes (filters, show-unlinked, or cached data swapping in). The kind filter's options SHALL come from the graph without the kind filter, including when a kind is already set in the URL.

#### Scenario: Path between two endpoints
- **WHEN** a user submits both endpoints and a path is found
- **THEN** exactly the edges the path followed are highlighted, no other edge is, and the hops are listed

#### Scenario: Walk from one endpoint
- **WHEN** a user submits only `from`
- **THEN** the request omits `to` and the edges the walk followed (for a branching walk, each node's edge from its parent) are highlighted and the walk is listed

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
