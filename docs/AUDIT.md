# Audit Trail

A record of who did what, and when, for the operator actions sensitive
enough to need attribution — connector changes and syncs, elevation,
auth settings changes, and document restores.

## Endpoint

`GET /api/system/audit` — instance-admin only (403 otherwise, `auth.RequireInstanceAdmin`; #240 replaced the flat operator/viewer role with per-connector grants), paginated
newest-first like other list endpoints (`page`, `pageSize`; see
`docs/openapi.yaml`'s `Page`/`PageSize` parameters). Optional filters:

- `action` — exact match, e.g. `connector.create`.
- `targetType` — exact match, e.g. `connector`, `doc`.

Response shape mirrors `ChangePage`/`AlertPage`: `{ items, total, page, pageSize }`.

### Keyset (cursor) pagination

Offset pagination re-counts and re-skips rows on every page, which gets
expensive as `audit_log` grows. Sending `cursor` opts into keyset pagination
instead: `?cursor=` for the first page, then the previous response's
`nextCursor` for each page after it. The server answers
`WHERE (created_at, id) < (cursor) ORDER BY created_at DESC, id DESC LIMIT
pageSize`, which reads straight out of an index (migration 000033) and cannot
skip or repeat a row when several records share a `created_at`.

The cursor is opaque — never build one client-side. `nextCursor` is absent on
the last page and on requests that did not send `cursor`, so offset clients see
the unchanged four-key envelope. While a cursor is in use `page` is ignored;
`total` still reports the full filtered count.

`GET /api/changes` and `GET /api/connectors/{id}/syncs` take the same `cursor`
parameter. Since the sync-run response body is a bare array, its cursor comes
back in the `X-Next-Cursor` response header rather than an envelope field.

## What's recorded

Each entry (`AuditRecord` in `docs/openapi.yaml`) has: `actorUserId`,
`actorRole`, `action`, `targetType`, `targetId`, `detail` (a small JSON
object, action-specific), and `createdAt`.

| Action | Trigger | targetType / targetId |
|---|---|---|
| `connector.create` | `POST /api/connectors` | connector / new ID |
| `connector.update` | `PUT /api/connectors/{id}` | connector / id |
| `connector.delete` | `DELETE /api/connectors/{id}` | connector / id |
| `connector.toggle_enabled` | `PUT /api/connectors/{id}/enabled` | connector / id |
| `connector.recipe_preview` | `POST /api/connectors/recipe-preview` (including validation or endpoint errors) | connector / optional existing ID; detail contains only the redacted target URL |
| `connector.sync` | `POST /api/connectors/{id}/sync` | connector / id |
| `snapshot.diff.export` | `GET /api/connectors/{id}/snapshots/diff?from=…&to=…&format=json\|csv\|md\|html` | connector / id |
| `connector.sync_all` | `POST /api/sync` | connector / (none) |
| `connector.restart` | `POST /api/connectors/{id}/restart` (dryRun omitted/false), or `POST /api/runbooks/{id}/steps/{stepId}/execute` for a `restart` step | connector / id |
| `connector.start` | `POST /api/connectors/{id}/start` (dryRun omitted/false), or `POST /api/runbooks/{id}/steps/{stepId}/execute` for a `start` step | connector / id |
| `connector.stop` | `POST /api/connectors/{id}/stop` (dryRun omitted/false), or `POST /api/runbooks/{id}/steps/{stepId}/execute` for a `stop` step | connector / id |
| `connector.action` | `POST /api/connectors/{id}/actions/{name}`, or a named-action step in a run | connector / id; action, entity reference, method, URL without query or credentials, and status only |
| `connector.recipe_actions_changed` | Saving or reconciling changed recipe actions | connector / id; added, changed and removed qualified action names |
| `backup.import` | Successful backup restore | backup_import / default; imported counts and names of imported connectors with actions |
| `runbook.run.step_resent` | Resuming an unknown named-action step with `resend` | runbook_run / run id; step id and acting user |
| `runbook.run.step_marked_done` | Resuming an unknown named-action step with `mark_done` | runbook_run / run id; step id and acting user |
| `connector.configPush` | `POST /api/connectors/{id}/config-push` (successful, verified push only), or a `config_push` step in a runbook run started via `POST /api/runbooks/{id}/run` or resumed via `POST /api/runbook-runs/{runId}/resume` | connector / id |
| `connector.maintenanceWindow.open` | `POST /api/connectors/{id}/maintenance-window` | connector / id |
| `connector.maintenanceWindow.close` | `DELETE /api/connectors/{id}/maintenance-window` (only when a window was actually active) | connector / id |
| `connector.bulk_sync` | `POST /api/connectors/bulk-sync` | connector / id — one record per resolved item |
| `connector.bulk_reauth` | `POST /api/connectors/bulk-reauth` | connector / id — one record per resolved item |
| `connector.bulk_restart` | `POST /api/connectors/bulk-restart` | connector / id — one record per resolved item |
| `connector.release` | `POST /api/connectors/{id}/release` (an orphaned connector returned to UI management) | connector / id |
| `connector.config_create` | Startup: a connector declared in `config.yaml` was created. `actorRole` is `system`, with no actor user | connector / new ID |
| `connector.config_adopt` | Startup: a UI-created or orphaned connector was taken under config management. `detail.fields` names what changed; a changed secret appears only as `config.secret` | connector / id |
| `connector.config_update` | Startup: a config-managed connector's entry changed. Same `detail` as adopt | connector / id |
| `connector.config_orphan` | Startup: a config-managed connector's entry was removed; it was disabled | connector / id |
| `connector.config_grants` | Startup: grants declared for the connector changed. `detail` holds the added and removed counts | connector / id |
| `auth.elevate` | `POST /api/auth/elevate` | action / the elevated action name |
| `auth.elevation_requested` | Any step-up-gated endpoint receiving `X-Elevation-Token` | action / the required action name |
| `auth.elevation_denied` | Failed elevation-token validation on a step-up-gated endpoint | action / the required action name |
| `auth.config.update` | `PUT /api/auth/config` | auth_config / (none) |
| `auth.provider.enabled` | `PUT /api/auth/providers/{id}/enabled` | oidc_provider / provider id |
| `journal.create` | `POST /api/journal` | journal / new ID |
| `journal.update` | `PUT /api/journal/{id}` | journal / id |
| `journal.delete` | `DELETE /api/journal/{id}` | journal / id |
| `doc.restore` | `POST /api/docs/{id}/versions/{rev}/restore` | doc / doc id |
| `doc.edit_proposed` | MCP `propose_doc_edit` (full-scope key; `detail` has `proposalId`, `baseVersion`) | doc / doc id |
| `doc.edit_approved` | `POST /api/docs/edit-proposals/{id}/approve` | doc / doc id |
| `doc.edit_rejected` | `POST /api/docs/edit-proposals/{id}/reject` | doc / doc id |
| `change.ack` | `POST /api/changes/{id}/ack` | change / id |
| `change.dismiss` | `POST /api/changes/{id}/dismiss` | change / id |
| `change.bulk_ack` | `POST /api/changes/bulk-resolve` (`status: acknowledged`) | change / id — one record per resolved item |
| `change.bulk_dismiss` | `POST /api/changes/bulk-resolve` (`status: dismissed`) | change / id — one record per resolved item |
| `alert.resolve` | `POST /api/alerts/{id}/resolve` | alert / id |
| `alert.dismiss` | `POST /api/alerts/{id}/dismiss` | alert / id |
| `alert.snooze` | `POST /api/alerts/{id}/snooze` | alert / id |
| `finding.resolve` | `POST /api/findings/{id}/resolve` | finding / id |
| `runbook.create` | `POST /api/runbooks` | runbook / new ID |
| `runbook.update` | `PUT /api/runbooks/{id}` | runbook / id |
| `runbook.run.start` | `POST /api/runbooks/{id}/run` (non-preview) | runbook_run / run ID; detail includes runId and runbookId |
| `runbook.run.confirm` | `POST /api/runbook-runs/{runId}/steps/{stepId}/confirm` | runbook_run / run ID; detail includes runId, runbookId and stepId |
| `runbook.run.resume` | `POST /api/runbook-runs/{runId}/resume` | runbook_run / run ID; detail includes runId and runbookId |
| `runbook.run.cancel` | `POST /api/runbook-runs/{runId}/cancel` | runbook_run / run ID; detail includes runId and runbookId |
| `runbook.delete` | `DELETE /api/runbooks/{id}` | runbook / id |
| `entity.override.create` | `POST /api/entity-overrides` | entity_override / new ID |
| `entity.override.delete` | `DELETE /api/entity-overrides/{id}` | entity_override / id |
| `compliance_rule.create` | `POST /api/compliance/rules` | compliance_rule / new ID |
| `compliance_rule.update` | `PUT /api/compliance/rules/{id}` | compliance_rule / id |
| `compliance_rule.delete` | `DELETE /api/compliance/rules/{id}` | compliance_rule / id |
| `compliance_rule.enable` | `PUT /api/compliance/rules/{id}` (enabled true) | compliance_rule / id |
| `compliance_rule.disable` | `PUT /api/compliance/rules/{id}` (enabled false) | compliance_rule / id |
| `discovery.scan.start` | `POST /api/discovery/scan` (a scan was accepted) | discovery_scan / scan ID; detail `range` (the masked CIDR) |
| `discovery.scan.complete` | A network scan ends on its own, as `completed` or `failed` (written by the scan, attributed to the admin who started it) | discovery_scan / scan ID; detail `range`, `state`, `probed`, `answered`, `candidates`, `durationMs`, `partial` |
| `discovery.scan.cancel` | `DELETE /api/discovery/scan` stops a running scan, or the server shuts down while one runs (written by the scan, attributed to the admin who started it; same detail as complete, `state` `cancelled`, plus `cancelledBy` or `cancelReason`) | discovery_scan / scan ID; detail also `cancelledBy` (user ID of the admin who cancelled, which may differ from the actor) or `cancelReason` `shutdown` (no `cancelledBy`) |
| `discovery.scan.reject` | `POST /api/discovery/scan` refused, for an invalid range or while a scan runs, or over the hourly limit | discovery_scan / (none); detail `range` (as submitted, at most 64 characters) and `reason` |

`detail` never carries secret values. `connector.update` and
`auth.config.update` record which *fields* changed (a name list), not
their values — connector config can hold credentials, and this keeps the
audit log safe to expose to any instance admin without redaction logic.
`connector.configPush` follows the same discipline: `detail` records the
pushed `fieldKey` (name) and `entityRef`, never the pushed `value`. For a direct
push (`POST /api/connectors/{id}/config-push`), `detail` contains `fieldKey` and
`entityRef`. When triggered by a `config_push` step in a runbook run, `detail`
additionally identifies the run: `runId`, `stepId`, `stepIndex` (the 0-based
step position), and `runbookId` (when the run originated from an existing runbook).
The audit actor is the run's acting user: the user who last resumed the run,
otherwise the user who started it (`runbookrun.ActingUser`).
Like direct pushes, only successful verified writes produce an audit record; a push
step that finds the field already at target succeeds without writing and records
no audit entry.

`connector.restart`/`connector.start`/`connector.stop` triggered by
`POST /api/runbooks/{id}/steps/{stepId}/execute` carry `runbookId` and
`stepId` in `detail` alongside `entityRef`, so an execution can be traced
back to the runbook step that triggered it — linking a step still grants no
mutation permission by itself (see `docs/adr/0001-lab-mutating-operation-boundaries.md`);
the caller still needs an operator grant on the step's connector and a
valid elevation token for `connector.<verb>`. `runbook.create`/
`runbook.update` record the runbook's `title` and its `steps` (each step's
`id`, `kind`, `connectorId`, `verb`, `entityRef`, `timeoutSeconds`); `runbook.update` additionally
records `changedFields`, the list of top-level keys present in the request
body. `runbook.delete` records the deleted runbook's `title`.

`discovery.scan.*` detail carries the scanned range, the scan outcome and counts
only: `probed` is how many addresses were probed, `answered` how many accepted a
connection on a listed port, and `candidates` the number of products found per
connector type. It never contains the address of a scanned or found host. A range
of `/32` is the one address the admin chose to scan, and it appears as that
range. A cancel entry names who cancelled in `cancelledBy` (the actor is always
the admin who started the scan), or carries `cancelReason` `shutdown` when the
server stopped the scan while shutting down. `discovery.scan.reject` `range` is
the admin's own input exactly as submitted, cut to 64 characters, and `reason` is one of `invalid_cidr`, `not_ipv4`, `not_private`,
`too_wide`, `scan_in_progress` or `rate_limited`.

`entity.override.create`/`entity.override.delete` record the override's
`action` (`detach` or `merge`), `note` and `members` (each `connectorId`,
`kind`, `ref`; one for a detach, two for a merge). Identity IDs are not
recorded: they change on merge and split.

## What's not recorded

- **Failed attempts, except elevation denials, recipe previews and refused network scans.** An audit entry is written
  only after the action itself succeeds. Recipe previews are also audited when
  validation or an endpoint fails. Refused network scan starts are audited as
  `discovery.scan.reject`. A failed create/update/delete never
  reaches the audit log — this is a deliberate scope cut: this endpoint
  answers "what happened," not "what was attempted." Elevation-token denials
  are the exception because they are security-relevant attempts.
- **Reads.** Listing or viewing a resource is not an audited action, including the network discovery results. Downloading
  a snapshot diff with `format` is the exception; viewing the same diff as JSON
  without `format` is not audited.
- A write to the audit log failing is logged (`slog.Error`) but never
  fails the request — the audited action has already gone through by
  that point, so refusing the response would be misleading.

## Retention

Audit rows are pruned by the retention job (`DeleteOldAuditRecords` in
`backend/internal/store/retention.go`, called from
`backend/internal/retention/retention.go`) like other operational tables, using
the `retention.audit_days` setting (env `WISELABZ_RETENTION_AUDIT_DAYS`,
default `180`). Rows with `created_at` older than that many days are deleted;
there is no "keep latest" guard. Setting `retention.audit_days` to `0` disables
audit pruning entirely, so rows are kept indefinitely. The elevation
request/denial rows are ordinary `audit_log` rows and follow the same policy.

## Filtering and export

`GET /api/system/audit` accepts these filters (all optional, combinable):

- `action` — exact match.
- `targetType` — exact match.
- `createdAfter` — inclusive lower bound on `createdAt` (RFC 3339 date-time).
- `createdBefore` — inclusive upper bound on `createdAt`.

`GET /api/system/audit/export?format=json|csv` (instance-admin only) returns
every record matching the same four filters, newest first. It is **not
paginated** and has no row limit, so narrow it with filters on large logs.
`format` defaults to `json`; any other value is a 400 (`invalid_format`).
Both formats are served as an attachment named
`wiselabz-audit-<UTC timestamp>.json|csv`. JSON is an array of `AuditRecord`
objects. CSV has a header row with the columns `id`, `actorUserId`,
`actorRole`, `action`, `targetType`, `targetId`, `detail` (the raw JSON string),
`createdAt`.

Starting and resuming a whole run require the `runbook.run` elevation action,
targeted at the runbook ID (ADR 0006). Elevation is validated once; lifecycle
steps retain their `connector.<verb>` audit action with runId, runbookId and
stepId in detail, and config-push steps retain `connector.configPush` with runId,
runbookId, stepId, stepIndex (0-based), fieldKey and entityRef. Preview,
confirmation and cancellation need no elevation.

The start, resume and confirm entries are also written when the request ends
in 503 during shutdown, because the transition was already recorded (the run
exists as failed/interrupted, the manual step stays confirmed).

Recipe actions follow ADR 0008: previews resolve from the stored snapshot and show
the fixed request without credentials. Responses expose at most 512 bytes of
plain text once to the operator; excerpts never enter audit details, alerts, logs
or run records. The Journal includes named actions, backup imports and unknown
step decisions as lab activity. Recipe-action enablement is a security boundary
and remains in the admin audit log, outside the Journal lab-action list.

The Journal shows an allowlist of lab audit actions. Its audit rows carry a
connector scope captured when the action is recorded, with existing rows
backfilled where their target identifies a connector. Members see a row only
when they hold a viewer or operator grant on every connector in its scope, and
connector-restricted API keys must allow every connector in a nonempty scope;
restricted keys cannot read unscoped rows. Unscoped rows remain visible only to
instance admins. Admins can also see rows scoped to connectors that have since
been deleted; live scoped rows still require the matching grant. Security actions
are not in the Journal allowlist. The Journal exposes
the actor but omits audit detail; `GET /api/system/audit` and its export remain
instance-admin only.
