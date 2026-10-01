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

- **Lab-wide docs and restricted keys**: lab-wide docs (no connector) cannot honor a connector allow-list, so a connector-restricted API key is never instance admin. `auth.InstanceAdminFromContext` enforces this itself (AuthMiddleware already cleared it), so MCP search, propose and runbook doc links, and the REST list/review paths, all treat such a key as non-admin for lab-wide docs.
- **Proposal bounds**: a new proposal by the same author for the same doc replaces (deletes) their older pending one, and an author may hold at most 25 pending proposals (`store.ErrProposalLimit`, surfaced as a tool error). The count-then-insert runs in one transaction but is not serialized across writers, so the cap is soft. `baseVersion` must be within 1..current; a stale base is accepted and fails at approval. Content is capped at 256 KiB. Review listing filters by reviewable connectors and pages in SQL and omits `content`; `GET /docs/edit-proposals/{id}` returns it. The audit write after a proposal is best-effort (logged, not a tool error), matching other audited writes.
- **Self-approval**: nothing stops an operator who proposes through MCP (own full-scope key) from approving it in the UI. The invariant is that MCP itself never applies edits and that every applied edit is a human REST action with a version and audit entry; separation of duties is not enforced because a single operator could already edit the doc directly.
- **Search ranking and engine differences**: SQLite uses FTS5 (porter, bm25 with title weighted 5x) and Postgres a generated `tsvector` (english config, title weight A, body weight B, `ts_rank`). Both receive the same sanitized tokens (letters/digits only, ANDed, last token a prefix; Postgres via `to_tsquery`, not `websearch_to_tsquery`, so `OR`, `-` and quotes are inert on both). English stopwords are dropped from the query in Go so a stopword-only query returns no hits, not an error, on both engines (SQLite keeps stopwords in its index; Postgres drops them). Scores are not comparable across engines or across tables, so docs and runbooks are interleaved by rank and scores are normalized to 0-1 per result type. Residual differences: stemmers differ slightly (porter vs snowball english), tie order may differ, and accent folding differs (SQLite FTS5 folds diacritics so `munchen` matches `München`; Postgres `english` does not, and `unaccent` is deliberately not required). Queries with the exact accented spelling match on both.
- **FTS delete cost (SQLite)**: the FTS5 `id` column is UNINDEXED, so trigger deletes scan the FTS table. Keying FTS rowids to `docs.rowid` would avoid that, but TEXT-keyed tables may be renumbered by `VACUUM`, desynchronizing the index; with a lab-sized corpus the scan is cheap and correct.
- **Topology freshness**: edges are rebuilt after each successful sync. Only a missing snapshot clears a connector's edges; any other load error keeps the last good edges. On startup the leader runs a background backfill (idempotent, non-blocking, logged) for connectors that have a snapshot but no edges, so `topology_path` works right after upgrade. `topology_edges` is indexed `(src_connector_id, kind)` and `(dst_connector_id, kind)` to serve both the endpoint listing and the `same_as` cleanup in either direction.
- **Draft runbook content**: the draft quotes only the alert, the connector display name and the change summary/diff (a snapshot diff); it never reads connector configuration, so credentials in connector config are not copied. Diffs are cut on rune boundaries and fenced with a backtick run longer than any inside the diff.

## Risks / Trade-offs

- FTS index drift between engines → parity tests and write-path tests.
- Edge staleness between syncs → rebuild per connector atomically in a transaction.
- Heuristic entity matching creates false edges → record edge `source`, document in tool output.
- Weakened "MCP read-only" invariant → single write tool, covered by e2e key-scope tests.
