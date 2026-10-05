# Design

## Context

`topology_edges` already stores connector-owned endpoint references and backs MCP `topology_path`. Entity identities and active connector members are introduced by the preceding #498 PR. DNS, reverse proxy, container, and virtualization connectors expose relationship information in snapshots, but the existing builder only records generic dependencies, containment, and heuristic same-as links.

## Decisions

- Extend the existing edge table and builder. Directional edge kinds are `resolves_to` (DNS record to the matched IP or hostname owner), `proxies_to` (proxy entity to configured upstream), and `runs_on` (container or VM to host, VM, or node). `detail` stores useful edge labels such as an upstream port.
- Move breadth-first traversal into a package shared by MCP and REST. MCP continues traversing its grant-filtered graph as an undirected shortest path.
- `GET /api/topology/path` returns an undirected shortest path when `to` is supplied and a directed reachable walk when it is omitted. Only visible connectors and connectors allowed by the API key enter traversal.
- `GET /api/topology/graph` emits identity nodes for entity endpoints, connector service nodes and unresolved endpoints as plain nodes, and resolves memberships only where `gone_at IS NULL`. Hidden connectors and their member metadata never enter the response.
- Graph filters accept connector and entity kind; unlinked entities are excluded unless explicitly requested. Authorization is applied before graph construction and identity resolution.
- After a topology rebuild changes persisted edge fields, sync refreshes the generated Lab Topology document only when that generated document already exists. An unchanged edge set does not create a doc version.

## Risks

- Snapshot relationships can be incomplete or ambiguous; unresolved targets remain connector-owned plain nodes rather than creating identities.
- Identity IDs are global identifiers, not access grants. Every edge and active member must be scoped to visible connectors before response construction.
- A concurrent topology rebuild may change the edge set between comparison and persistence; replacement stays transactional and regeneration is best-effort follow-up work.
