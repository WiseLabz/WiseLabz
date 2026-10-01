# Design

## Context

Manual check lives inline in `api/connectors/handlers.go` Health (validate, `ClassifyHealth`, update status, `RecordHealthCheck`). Scheduler is robfig cron wrapped in `internal/scheduler` with `SkipIfStillRunning` and job health tracking; leader election already gates it. `GetConnectorUptime` is time-weighted and ignores maintenance. `health_check_days` (default 90) prunes history. Web has generated `getConnectorsConnectorIdUptime` but no consumers.

## Goals / Non-Goals

**Goals:** continuous, maintenance-aware data; cheap fleet and history reads.
**Non-Goals:** per-connector intervals, rollup tables, alerting on flaps, notifications on status transition.

## Decisions

- **Shared helper** `RunHealthCheck` in new package `internal/health` (not `internal/connector`: store tests import connector, so connector importing store would create an import cycle) taking store + record; Health handler and job both call it. Keeps one classification path.
- **Job**: modeled on `sync.Engine.RunDueSyncs` (semaphore, waitgroup, per-check `context.WithTimeout`); lists enabled connectors without active maintenance (new store query); registered in `cmd/server/main.go`; cron in config with default `*/60`-equivalent and entry in `validateCronExpressions`.
- **Maintenance-aware math**: in `GetConnectorUptime`, subtract maintenance intervals (clipped to window) from the timeline before computing availability and outages. Alternative of dropping rows near maintenance rejected as lossy.
- **Gaps are not extrapolated**: a check's status holds for at most `max(5m, 3 x median gap between the window's consecutive checks)` after it (the median approximates the effective check interval, so it adapts to the configured cron). Time beyond that is unknown and counts toward neither availability nor outage duration, so a disabled connector or a stopped server does not read as up/down for the whole gap, and a stale last status does not extend to the window end.
- **"No data" contract**: `checkCount` is the number of checks that contributed measured (non-maintenance) time. A window whose checks all fall inside maintenance, or at the window edge, reports `checkCount: 0` (availability is then meaningless); no extra field is needed and the UI already renders `checkCount == 0` as "No data".
- **Cancellation vs timeout**: `RunHealthCheck` persists with `context.WithoutCancel` so a per-check deadline (hung connector) is recorded as offline, but a cancelled parent (`context.Canceled`: shutdown, manual-check client disconnect) writes nothing and returns the error.
- **History**: fixed bucket counts per window (worst status, avg latency), bucketed in Go over one indexed range query since checked_at is an RFC3339 string and sqlite/postgres date math differs.
- **Fleet**: one query over visible connector ids (grant filter), reusing window logic; new `WidgetPlacementType` value `uptime`.
- **History response** also carries `windowStart`, `windowEnd` and `bucketSeconds` so the sparkline places buckets by time (gaps visible) and breaks the latency line at buckets without latency instead of plotting 0.
- **Web**: generated hooks; "no data" when `checkCount==0`; i18n keys in `en.ts`.

## Risks / Trade-offs

- 60s × 90 days ≈ 130k rows/connector → indexed `(connector_id, checked_at)`, bucketed reads; revisit rollups if slow.
- Scheduled status writes may race with sync status → last-writer-wins, same as manual today. The status write is skipped when status and message are unchanged (no per-interval `updated_at` bump), but a changed result still overwrites a richer status set by sync (e.g. a sync-derived degraded message) until the next sync; accepted, a compare-and-set on the connector row would be the fix if it matters.
- The status-unchanged comparison uses the record loaded at the start of the sweep, so a concurrent sync write can make it skip or repeat one write; harmless.
- Load on external systems → global interval is configurable; per-check timeout and bounded concurrency.
