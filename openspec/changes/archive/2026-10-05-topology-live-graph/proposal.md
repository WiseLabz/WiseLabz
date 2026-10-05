# Proposal

## Why

Persisted topology currently has dependency, same-as, and containment links, and MCP is the only path interface. A live graph needs typed directional relationships, identity nodes, and a grant-filtered REST API that both the UI and MCP can use safely.

## What Changes

- Derive directional `resolves_to`, `proxies_to`, and `runs_on` edges from DNS, proxy, container, and virtualization snapshots; persist optional edge detail such as upstream ports.
- Share topology traversal between MCP and REST while preserving the existing MCP path behavior.
- Add grant- and API-key-scoped REST endpoints for graph retrieval and path tracing, resolving observed entity endpoints to active entity identities.
- Regenerate an existing Lab Topology document after sync only when the persisted edge set changes.

## Capabilities

### New Capabilities
- `live-topology`: typed topology relationships, shared traversal, and authorized graph and path APIs.

## Impact

- Backend topology builder, store migration 000059 for SQLite and PostgreSQL, sync doc refresh behavior, MCP traversal, REST API, OpenAPI contract, and generated frontend client.
- Depends on entity identity migration 000058 from #631. Identity endpoints resolve only through active members, and connector grants remain the security boundary.
