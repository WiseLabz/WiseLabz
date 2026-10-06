# Design

## Context

See proposal.md for motivation. Current state:

- `doc.identityClusters` (`backend/internal/doc/identities.go`) runs a union-find over strong-match features of every member in the latest readable snapshots and returns clusters.
- `Store.ReconcileEntityIdentitiesWith` (`backend/internal/store/entity_identities.go`) takes those clusters inside one serialized transaction (process mutex plus a PostgreSQL advisory lock) and decides identity IDs: the oldest existing ID wins a merge, losers get `merged_into`/`merged_at`, a cluster that splits keeps its previous ID in exactly one part, gone members keep their last identity.
- `entity_members` is keyed by `(entity_id, connector_id, kind, ref)` with a unique active key on `(connector_id, kind, ref)`. Identity IDs change on merge and split; the member key does not.
- `GET /api/entities/{id}` is grant-filtered and answers every "not visible" case with the same 404.

## Goals / Non-Goals

**Goals:**

- Overrides that survive reconciliation, restarts, members going away and coming back.
- A precedence rule that is deterministic and cannot reach an unsatisfiable state.
- No change to the identity-selection code or to connector-local refs.

**Non-Goals:**

- Pairwise "never the same" constraints between two specific members.
- Per-connector (non-admin) override permissions.
- Editing an override in place; an override is removed and recreated.
- Changing which automatic signals count as strong.

## Decisions

### Override record: a separate table keyed by member key

`entity_identity_overrides(id, action, connector_id, kind, ref, other_connector_id, other_kind, other_ref, note, created_by, created_at)`; `action` is `merge` or `detach`; the `other_*` columns are NULL for a detach. Both connector columns reference `connectors(id) ON DELETE CASCADE`. A merge pair is stored in sorted member-key order and a unique index covers the whole tuple, so A/B and B/A are the same row.

*Alternatives:* columns on `entities` (identity IDs are not stable across merges and splits, and an override would be lost when its row redirects); a foreign key to `entity_members` (the row's `entity_id` is part of its primary key and changes on merge, and a purge of member history would cascade unpredictably). The member key is the only stable handle.

### Split is "detach member", not "cannot link"

A detach removes one member's strong-match features before clustering. A pairwise cannot-link is ambiguous under transitivity (A matches C, C matches B: which side keeps C?) and can contradict a merge override; detach has neither problem. Moving a member is expressed as detach plus merge.

### Precedence is applied in `identityClusters`

`identityClusters(members, overrides)`:

1. Members with a detach override contribute no features.
2. Existing union-find over the remaining features, unchanged.
3. For every merge override whose two members are both in `members`, join them.

The reconcile callback in `BackfillEntityIdentities` loads overrides from the transaction-bound store it already receives, so overrides and snapshots are read under the same lock.

*Alternative:* post-processing clusters in the store layer. Rejected because the store receives only final clusters and has no feature data to re-cluster the remainder of a split group.

### Identity selection is not modified

`chooseIdentityIDs`, `flattenIdentityRedirects` and `moveMergedIdentityMembers` already handle clusters that grow or shrink, so a manual merge yields an ordinary redirect and a detach yields a new identity for one side. Tests pin this behaviour for override-driven changes instead of assuming it.

### Lifecycle

- **Validation at creation** (store layer, inside a transaction): each member key must have an `entity_members` row (active or gone); merge needs two different keys of the same kind; a duplicate maps to a conflict error. A detach is allowed on a single-member identity: it is a valid "never auto-merge this" instruction.
- **State** is derived, not stored: `active` when every referenced member row has `gone_at IS NULL`, otherwise `dormant`.
- **Retention**: `DeleteExpiredEntityIdentities` already runs under the reconcile lock; in the same transaction it deletes an override when this run removes the member history it references (the member key has `entity_members` rows and all of them expire). An override whose member has never been observed, for example after a restore before the connector's first sync, is kept and stays dormant until the member appears, its connector is deleted, or an admin removes it.
- **Backup**: the override table is added to the export and import table list. `entities` and `entity_members` stay out, because they are rebuilt from snapshots; overrides refer to member keys, which a rebuild reproduces. Import order places the table after `connectors`.

### API

Routes in `mountWorkflowRoutes`, grouped under `auth.RequireInstanceAdmin`. The handler needs the doc engine to reconcile; it calls `BackfillEntityIdentities` after the store mutation and then resolves the current identity ID of each affected member for the response. Reconciliation failure after a committed mutation returns 500 with the override in place; the next sync reconciles it. Audit uses `Store.RecordAuditFromContext`, non-fatal on failure as elsewhere.

The returned identity ID is not a visibility grant: `GET /api/entities/{id}` keeps its connector grant rule and uniform 404, so an administrator without a viewer grant on the member's connector gets the ID but a 404 for it, and the web flow stays on the page with a confirmation in that case.

Admin-only keeps the first version simple: an override changes what every user sees, including on connectors a per-connector operator cannot view.

### Web

Actions live on `EntityDetailPage`; the merge target is chosen with the existing entity picker, filtered to the same kind. The overrides list is an admin page linked from the entity detail page. After a mutation the page navigates to the identity ID returned by the API, since the ID in the URL may have become a redirect.

## Risks / Trade-offs

- [A detach hides a real match forever] → the overrides list shows every override with its state and offers removal; removal restores automatic clustering at once.
- [Merge overrides chain into a large identity] → merges are same-kind only and each is listed and audited.
- [Synchronous reconciliation makes mutations slow on large installations] → it is the same full reconcile every connector sync already runs; mutations are rare admin actions.
- [Overrides restored from a backup reference members not yet observed] → they are dormant until the first sync, then apply.
- [Reconcile failure after commit] → the override is stored and applies at the next reconcile; the API reports the failure.

## Migration Plan

Additive migration `000061` (up creates the table and indexes, down drops them). No data backfill. Rollback: run the down migration; identities return to automatic clustering at the next reconcile.
