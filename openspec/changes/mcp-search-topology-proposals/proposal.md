# Proposal

## Why

The MCP server exposes only five read-only tools: no keyword search, no topology reasoning, no runbook access, and no way for an AI client to suggest documentation fixes. Operators using MCP clients (GitHub issue #518) cannot ask "how is X connected to Y", find docs without embeddings configured, or turn an alert into a runbook starting point.

## What Changes

- Add full-text search (SQLite FTS5 / Postgres tsvector) over docs and runbooks, exposed as an MCP `search` tool that works without AI configured.
- Persist an entity-level topology edge table at sync time and expose an MCP `topology_path` tool (shortest path between two entities), filtered by the caller's connector grants.
- Add MCP `list_runbooks` / `get_runbook` tools; steps referencing connectors the caller cannot access are redacted.
- Add an MCP `propose_doc_edit` tool that stores a `doc_edit_proposal`; MCP never applies edits. Doc operators approve or reject via REST and a minimal review UI; approval applies the edit through the normal versioned save with a base-version conflict check.
- Add `POST /alerts/{id}/draft-runbook`: deterministic template draft (no LLM), returned only, not persisted.
- Update the "MCP is permanently read-only" statements: MCP gains exactly one write tool, restricted to non-read-only keys.

## Capabilities

### New Capabilities
- `mcp-knowledge-tools`: MCP search, topology path and runbook read tools, with grant and API-key scoping.
- `doc-edit-proposals`: proposing, reviewing, approving and rejecting draft-only documentation edits.
- `alert-runbook-drafting`: deterministic runbook draft generated from an alert.

### Modified Capabilities

## Impact

- Backend: `internal/mcp`, `internal/store` (FTS tables, `topology_edges`, `doc_edit_proposals`; sqlite+postgres migrations 000046+), `internal/sync` (edge build hook), `internal/api/alerts`, `internal/api/docs`, `routes_*`, `docs/openapi.yaml`.
- Auth: `TreatAsSafeMethod` for `/mcp` no longer implies all tools are reads; the propose handler enforces key scope itself.
- Web: small proposal review list for doc operators; regenerated API client.
- Tests/CI: MCP harness, e2e key-scope tests, store parity tests, `scripts/ci/test-shards.json` if packages are added.
