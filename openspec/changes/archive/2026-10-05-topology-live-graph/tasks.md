# Tasks

- [x] Add SQLite and PostgreSQL migration 000059 for topology edge detail and cover migration behavior.
- [x] Derive typed directional DNS, proxy, and runtime edges from the existing connector snapshots.
- [x] Move shortest path traversal into a shared package without changing MCP behavior.
- [x] Add authorized REST path and graph endpoints; resolve graph endpoints to active identities only.
- [x] Regenerate an existing Lab Topology doc only when the edge set changes.
- [x] Update OpenAPI and generated client, then verify backend tests and lint.
- [x] Add `nodeId` to topology path steps (graph node ID, resolved through visible active members) with authorization tests, OpenAPI and generated client.
- [x] Replace the Mermaid-only topology page with a live pan and zoom graph: dagre layout, typed edge styling, legend, truncated notice, node list, and theme-aware chrome.
- [x] Add connector and kind filters, show-unlinked toggle and URL-backed state, with a no-matches state.
- [x] Add the trace form (path and from-only walk) with hop list, highlighting and error states.
- [x] Keep the Mermaid export for instance admins, with export error feedback.
- [x] Add page tests, including one against the real React Flow, and verify web, backend and build checks.
- [x] Rebase onto main once PR #635 merges and open the ready-for-review PR that closes #498.
