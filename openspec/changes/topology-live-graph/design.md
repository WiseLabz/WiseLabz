# Design

## Context

`topology_edges` already stores connector-owned endpoint references and backs MCP `topology_path`. Entity identities and active connector members are introduced by the preceding #498 PR. DNS, reverse proxy, container, and virtualization connectors expose relationship information in snapshots, but the existing builder only records generic dependencies, containment, and heuristic same-as links.

## Decisions

- Extend the existing edge table and builder. Directional edge kinds are `resolves_to` (DNS record to the matched IP or hostname owner), `proxies_to` (proxy entity to configured upstream), and `runs_on` (container or VM to host, VM, or node). `detail` stores useful edge labels such as an upstream port.
- `proxies_to` requires a positive, exact match (after host normalisation: case, trailing dot, URL/port stripped) between a proxy entity's declared upstream (NPM `forward_host`/`forwarding_host`, Caddy route `upstream`, Traefik router `service`) and a declared `upstream_service` dependency. Entities declaring no upstream emit nothing, no edge points back at its own source, and a Traefik `service` is a target of routers, never a source.
- `resolves_to` has a single owner: the connector of the DNS entity. A rebuild of either side re-derives the edge (a target connector's rebuild deletes DNS-owned `resolves_to` edges pointing at it and re-emits them with the DNS connector as owner), so removing the DNS record, or the target entity disappearing or changing IP, removes the edge on the next sync of either connector. DNS-to-DNS matches never produce `resolves_to`.
- Cross-connector lookups during a rebuild read every other connector's latest snapshot once and try connectors in connector-ID order, so a name that exists in several connectors resolves deterministically (same-connector matches always win).
- Move breadth-first traversal into a package shared by MCP and REST. MCP continues traversing its grant-filtered graph as an undirected shortest path.
- `GET /api/topology/path` returns an undirected shortest path when `to` is supplied and a directed reachable walk when it is omitted. Only visible connectors and connectors allowed by the API key enter traversal.
- `GET /api/topology/graph` emits identity nodes for entity endpoints, connector service nodes and unresolved endpoints as plain nodes, and resolves memberships only where `gone_at IS NULL`. Hidden connectors and their member metadata never enter the response.
- Responses are bounded rather than paginated: the graph returns at most 2000 nodes and 5000 edges and the directed walk at most 1000 steps; each response carries a `truncated` boolean that is true when the cap cut it short. Edges are taken in a stable order (kind, names, ID), so a capped result is deterministic. Duplicate edges (same endpoints, kind, source and detail, e.g. written by two connectors' rebuilds) collapse into one.
- Members of merged identities (`entities.merged_into` set) are ignored, as are gone members.
- Graph filters accept connector and entity kind; unlinked entities are excluded unless explicitly requested. Authorization is applied before graph construction and identity resolution.
- The generated Lab Topology document records a fingerprint of the stored edge set (an HTML comment). After every topology rebuild, the document is regenerated only when it already exists and its recorded fingerprint differs from the stored edges; the fingerprint ignores row IDs, timestamps, owners and the direction of symmetric `same_as` edges. An unchanged edge set creates no document version, and a failed regeneration is logged without failing the rebuild: the document keeps its old fingerprint, so the next rebuild retries.

## Risks

- Snapshot relationships can be incomplete or ambiguous; unresolved targets remain connector-owned plain nodes rather than creating identities.
- Identity IDs are global identifiers, not access grants. Every edge and active member must be scoped to visible connectors before response construction.
- A concurrent topology rebuild may change the edge set between comparison and persistence; replacement stays transactional and regeneration is best-effort and self-healing through the fingerprint.
- `contains` edges for a matched pair are written by whichever connector rebuilt last, so they can briefly exist under two owners; the graph collapses them and the fingerprint ignores the owner.
