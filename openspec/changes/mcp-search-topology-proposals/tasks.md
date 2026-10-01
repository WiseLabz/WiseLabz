# Tasks

## 1. Full-text search

- [x] 1.1 Add sqlite+postgres migrations for docs/runbooks FTS and store write-path sync; verify `migrations_parity_test` and new store tests pass
- [x] 1.2 Add store `SearchContent` with viewer filtering; verify unit tests for ranking, permissions and update/delete freshness
- [x] 1.3 Register MCP `search` tool; verify `mcp_test` works with AI disabled

## 2. Topology edges and path

- [x] 2.1 Add `topology_edges` migrations and store methods; verify parity tests
- [x] 2.2 Rebuild edges after connector sync using `matchEntities` and `ServiceDependency`; verify sync test asserts edges and replacement on re-sync
- [x] 2.3 Implement grant-filtered BFS and MCP `topology_path`; verify tests for found path, hidden intermediate and connector-limited key

## 3. Runbook MCP tools

- [x] 3.1 Add `list_runbooks`/`get_runbook` with step redaction; verify tests for redacted and visible steps

## 4. Doc edit proposals

- [x] 4.1 Add `doc_edit_proposals` migrations and store (create/list/approve/reject with base-version check); verify store tests including conflict
- [x] 4.2 Add MCP `propose_doc_edit` with read-only-key and role checks plus audit log; verify e2e `api/mcp_test` for full, read-only and unauthorized keys
- [x] 4.3 Add REST list/approve/reject endpoints and openapi entries; verify handler tests and openapi contract test
- [x] 4.4 Add minimal web review list (regenerate client with `bun run gen:api`); verify vitest for approve, reject and conflict states
- [x] 4.5 Update read-only comments in `mcp/server.go`, `api/routes.go`, `auth/apikey_scope.go` and docs; verify no stale "permanently read-only" text via grep

## 5. Alert runbook draft

- [x] 5.1 Implement `POST /alerts/{id}/draft-runbook` with openapi entry; verify handler tests for 200, 404, viewer 403 and AI-disabled

## 6. Integration

- [x] 6.1 Run `go test ./...`, web tests, `openspec validate mcp-search-topology-proposals`, update `test-shards.json` if needed, and run `graphify update .`
