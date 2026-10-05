# Design

## Context

`doc/entityRef` already defines a connector-local key as external ID when available, otherwise name. `doc/matchReason` supplies external-ID, IP, and hostname links; only external-ID and hostname are strong enough to collapse identities. Persisted `topology_edges` continue to represent weak IP relationships. Sync already invokes a topology builder after a successful snapshot, and lifecycle leadership startup already runs backfills.

## Decisions

- `entities` stores UUID identity, kind, display name, first/last observation time, gone time, and optional merge target. `entity_members` stores the unique `(connector_id, kind, ref)` observations and their names. Findings store nullable kind/ref so connector-level findings remain valid.
- Identity reconciliation reads latest snapshots while holding a transaction-scoped reconcile lock, unions strong matches transitively, and writes only the membership and identity changes. Gone members remain linked to their last identity with a `gone_at` marker; a returning connector-local member reuses that identity. Unreadable snapshots preserve that connector's last known memberships. Oldest existing identity wins a merge (clusters claim IDs active-first, then by the oldest identity they hold, then by how many identities they hold, with the cluster key only as the last tiebreak, so the result never depends on connector UUID or map order); a merge stamps the loser's `merged_at`, which redirect flattening preserves and clearing `merged_into` clears; merged rows remain addressable through `merged_into` and redirects point directly to their final winner. A previous identity can be retained by only one cluster after a split; other components get new identities.
- IP matches never participate in identity unioning. Existing topology edge generation remains unchanged and continues to record the IP reason.
- A pair matching both IP and hostname still merges by the strong hostname signal. Topology keeps its existing IP-first explanation for the link; identity matching checks strong signals independently of that displayed reason.
- A successful connector sync triggers reconciliation after topology rebuilding. A leader backfill initializes/reconciles all readable snapshots. Retention uses `snapshot_days` for both gone entities and merged redirect rows: gone rows age from `gone_at`, merged rows from `merged_at`.
- Search entity hits resolve their member key to the identity UUID. Entity-specific compliance findings persist kind/ref and use that pair as part of the open-finding uniqueness key.

## Future work

The next PR in #502 will expose entity detail pages, redirects, grant-filtered members and related links, changes, findings, and runbooks. Manual identity merge/split is not included here; the schema leaves room for a future manual override by keeping identity metadata separate from connector-local membership references, but this change adds no override fields or behavior.

## Entity details

The entity detail endpoint follows flattened `merged_into` redirects, then authorizes only when the caller can view at least one member connector. It applies API-key connector restrictions through the same grant filter as connector-scoped endpoints and constructs every response section from the resulting visible connector set. Display name, kind, and gone state come from visible members so hidden memberships cannot affect the response. Attribute history is derived on demand from retained snapshots using `sync.BuildSnapshotDiff` and its `EntityChange` records; no history table is added. Topology edges are read by stored kind and endpoints so future edge types appear without endpoint changes. Manual merge/split remains deferred to the linked follow-up issue.

## Risks

- Strong heuristic hostname matches can merge distinct systems; preserving merge redirects and keeping IP links weak limits accidental identity collapse.
- A malformed latest snapshot must not cause membership deletion. Reconciliation should fail without replacing the last known-good membership graph.
- Concurrent sync reconciliation must respect the unique connector/kind/ref member key.

## Entity-finding notifications

A compliance rule notifies once per connector, not once per entity. Before upserting a run's findings the checker resolves findings of vanished entities, then reads the highest `notified_severity` among the rule's remaining open findings on that connector. A finding notifies only when its severity is above that level and its own `notified_severity`; every other upserted finding is marked notified without sending. Consequences:

- A new entity matching an already-notified rule, or an entity that flaps while other findings stay open, does not notify again at the same or a lower severity.
- A real severity escalation notifies exactly once.
- When all of a rule's open findings on a connector resolve (including a full replacement of the matching entities in one run), the next firing is a new incident and notifies once.
