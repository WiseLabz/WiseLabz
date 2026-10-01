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
- **History**: fixed bucket counts per window (worst status, avg latency), bucketed in Go over one indexed range query since checked_at is an RFC3339 string and sqlite/postgres date math differs.
- **Fleet**: one query over visible connector ids (grant filter), reusing window logic; new `WidgetPlacementType` value `uptime`.
- **Web**: generated hooks; "no data" when `checkCount==0`; i18n keys in `en.ts`.

## Risks / Trade-offs

- 60s × 90 days ≈ 130k rows/connector → indexed `(connector_id, checked_at)`, bucketed reads; revisit rollups if slow.
- Scheduled status writes may race with sync status → last-writer-wins, same as manual today.
- Load on external systems → global interval is configurable; per-check timeout and bounded concurrency.
