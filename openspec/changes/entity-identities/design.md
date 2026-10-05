# Design

## Context

`doc/entityRef` already defines a connector-local key as external ID when available, otherwise name. `doc/matchReason` supplies external-ID, IP, and hostname links; only external-ID and hostname are strong enough to collapse identities. Persisted `topology_edges` continue to represent weak IP relationships. Sync already invokes a topology builder after a successful snapshot, and lifecycle leadership startup already runs backfills.

## Decisions

- `entities` stores UUID identity, kind, display name, first/last observation time, gone time, and optional merge target. `entity_members` stores the unique `(connector_id, kind, ref)` observations and their names. Findings store nullable kind/ref so connector-level findings remain valid.
- Identity reconciliation reads latest snapshots while holding a transaction-scoped reconcile lock, unions strong matches transitively, and writes only the membership and identity changes. Gone members remain linked to their last identity with a `gone_at` marker; a returning connector-local member reuses that identity. Unreadable snapshots preserve that connector's last known memberships. Oldest existing identity wins a merge; merged rows remain addressable through `merged_into` and redirects point directly to their final winner. A previous identity can be retained by only one cluster after a split; other components get new identities.
- IP matches never participate in identity unioning. Existing topology edge generation remains unchanged and continues to record the IP reason.
- A pair matching both IP and hostname still merges by the strong hostname signal. Topology keeps its existing IP-first explanation for the link; identity matching checks strong signals independently of that displayed reason.
- A successful connector sync triggers reconciliation after topology rebuilding. A leader backfill initializes/reconciles all readable snapshots. Retention uses `snapshot_days` for both gone entities and merged redirect rows.
- Search entity hits resolve their member key to the identity UUID. Entity-specific compliance findings persist kind/ref and use that pair as part of the open-finding uniqueness key.

## Future work

The next PR in #502 will expose entity detail pages, redirects, grant-filtered members and related links, changes, findings, and runbooks. Manual identity merge/split is not included here; the schema leaves room for a future manual override by keeping identity metadata separate from connector-local membership references, but this change adds no override fields or behavior.

## Risks

- Strong heuristic hostname matches can merge distinct systems; preserving merge redirects and keeping IP links weak limits accidental identity collapse.
- A malformed latest snapshot must not cause membership deletion. Reconciliation should fail without replacing the last known-good membership graph.
- Concurrent sync reconciliation must respect the unique connector/kind/ref member key.
