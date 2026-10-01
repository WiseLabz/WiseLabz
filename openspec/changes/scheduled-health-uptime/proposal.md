# Proposal

## Why

Health checks are recorded only when someone clicks "Check", so `GET /connectors/{id}/uptime` returns sparse data and no web code calls it (GitHub issue #481, finishing #281). Availability and MTTR are therefore invisible and unreliable.

## What Changes

- Add a scheduler job that health-checks every enabled connector on a global cron (`health.cron_expr`, default every 60s), skipping connectors in an active maintenance window.
- Extract the manual Check logic into a shared helper used by both paths; both update connector status and record a history row.
- Make uptime availability and MTTR exclude maintenance spans.
- Add a bucketed per-connector history endpoint and a fleet-wide uptime aggregate endpoint.
- Web: availability + MTTR strip and status/latency sparkline on the service page; fleet uptime dashboard widget (default disabled).

## Capabilities

### New Capabilities
- `scheduled-health-checks`: periodic, maintenance-aware health checking that records history and updates connector status.
- `uptime-reporting`: uptime windows, history and fleet aggregate APIs, and their UI surfaces.

### Modified Capabilities

## Impact

- Backend: `internal/connector` (shared helper), `internal/api/connectors` (Health, uptime), `internal/store/health_checks.go`, `internal/config`, `cmd/server/main.go`, `docs/openapi.yaml`.
- Web: `ServiceDetailPage.tsx`, dashboard widgets/store, i18n, regenerated API client.
- Load: ~1 row/connector/minute; existing 90-day retention retained (no rollups).
