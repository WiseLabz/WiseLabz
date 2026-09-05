# Bulk Review Actions

Lets an operator acknowledge or dismiss several low-risk detected changes
in one request, instead of resolving them one at a time, without losing
per-change provenance (issue #27).

## Endpoint

`POST /api/changes/bulk-resolve` — operator-only (403 for `viewer`), same
gate as the existing single-item `/api/changes/{id}/ack` and `/dismiss`.

Request body:

```json
{ "ids": ["change-1", "change-2"], "status": "acknowledged" }
```

- `ids` — an explicit, non-empty list of change IDs. There is no "all
  matching the current filter" option — the caller must name every ID it
  wants resolved. This is deliberate: a filter can silently include a
  change the operator never actually looked at.
- `status` — `acknowledged` or `dismissed` (the same two values
  `UpdateChangeStatus` already accepts for the single-item endpoints).

Response body: a per-item outcome, so the caller always knows what
happened to each ID, even when the batch is a mix of hits and misses:

```json
{
  "results": [
    { "id": "change-1", "status": "success" },
    { "id": "change-2", "status": "error", "reason": "not_low_risk" }
  ]
}
```

`reason` values: `not_found` (no such change), `not_low_risk` (severity
is `critical`), `internal_error`.

## What counts as low-risk

Severity `info` or `warning`. Severity `critical` is never eligible for
bulk resolution — those changes need individual review. This is a
judgment call, not something derived from a spec: a `critical` change is,
by definition, the kind of thing that shouldn't be waved through in a
batch.

**Enforced server-side, per item.** The endpoint never trusts the
client's selection — for every ID in the request it independently loads
the change and re-checks severity before acting. Sending a `critical` ID
(accidentally or otherwise) just gets that one item an error outcome; it
does not fail the request or affect the other IDs.

## Partial failure is not batch failure

One bad ID (not found, or not low-risk) never aborts the rest of the
batch. Each ID is evaluated and resolved independently, and the response
always lists one outcome per input ID whether it succeeded or not.

## Auditability

One audit record (`action` `change.bulk_ack` or `change.bulk_dismiss`,
`targetType` `change`, `targetId` the change's ID) is written per
*successfully*-resolved item — never one record for the whole batch.
This keeps the audit trail identical in shape to the existing single-item
`change.ack` / `change.dismiss` actions: every state change on a change
record is attributable to one action, one actor, one target, regardless
of whether it came through the single-item or bulk endpoint. See
`docs/AUDIT.md` for the full action table and audit trail conventions
(failed items are not audited — same "audit only success" rule as every
other audited action).

## Frontend

The Changes list (`web/src/features/changes/ChangesPage.tsx`) shows a
checkbox per row for operators; checkboxes on `critical` rows are
disabled. Selecting one or more rows reveals a bulk action bar with
Acknowledge/Dismiss buttons. On response, a toast summarizes the outcome
(e.g. "8 change(s) resolved" or "8 succeeded, 1 failed: not_low_risk").
