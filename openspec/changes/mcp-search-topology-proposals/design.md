# Design

## Context

MCP (`internal/mcp`, mark3labs/mcp-go) is mounted at `/api/mcp` behind `TreatAsSafeMethod`, so every POST passes the read-only-key gate; this holds only while all tools are reads. Docs search today is semantic (`chat.Retrieve`) plus title `LIKE`; there is no FTS. Topology code does not exist; `connector.ServiceDependency` and `doc/linking.go` `matchEntities` supply raw linkage. `Change` is an infra-drift record unsuited to doc proposals. Stores run on sqlite and postgres with parity tests.

## Goals / Non-Goals

**Goals:** read tools usable without AI; edits never applied by MCP; grant-correct results.
**Non-Goals:** LLM drafting, persisted drafts, semantic search changes, auto-merge of stale proposals.

## Decisions

- **FTS**: FTS5 virtual table (sqlite) and generated `tsvector` + GIN (postgres) over docs and runbooks, kept in sync in the store write paths. Alternative LIKE rejected for relevance.
- **Topology**: `topology_edges(connector_id, src_entity, dst_entity, kind, source)` rebuilt per connector after each sync, reusing `matchEntities` and `ServiceDependency`. BFS runs over edges whose both endpoints belong to connectors in the caller's allowed set (`FilterConnectorIDsByGrant` + API-key filter), so hidden nodes are never traversed. Alternative on-demand compute rejected per decision for speed; cost is migrations and a sync hook.
- **Proposals**: new `doc_edit_proposals` table (doc_id, base_version, content, author_id, status, reviewer, timestamps). Approve calls `Store.UpdateDocWithVersion` with base version. Alternative of reusing `ChangeRecord` rejected (service_id NOT NULL, no apply path, lab-wide docs).
- **Scope enforcement**: `propose_doc_edit` handler checks `APIKeyRestrictionFromContext(ctx).ReadOnly` and doc operator role itself; audit-logged. Comments claiming "permanently read-only" are updated.
- **Draft runbook**: pure function over alert, change and `GetRunbookByTarget`; sync; handler gets Store only.

## Risks / Trade-offs

- FTS index drift between engines → parity tests and write-path tests.
- Edge staleness between syncs → rebuild per connector atomically in a transaction.
- Heuristic entity matching creates false edges → record edge `source`, document in tool output.
- Weakened "MCP read-only" invariant → single write tool, covered by e2e key-scope tests.
