# Prometheus metrics

WiseLabz can expose a Prometheus-format `/metrics` endpoint at the server root. It is **off by
default** and, when enabled, requires a bearer token.

| Setting | Env var | Default | |
|---------|---------|---------|---|
| `metrics.enabled` | `WISELABZ_METRICS_ENABLED` | `false` | Mounts `GET /metrics`. |
| `metrics.token` | `WISELABZ_METRICS_TOKEN` | — | Required when enabled; scrapers send `Authorization: Bearer <token>`. |

Prometheus scrape config:

```yaml
scrape_configs:
  - job_name: wiselabz
    metrics_path: /metrics
    authorization:
      credentials: <token>
    static_configs:
      - targets: ['wiselabz:8080']
```

## Metrics

All metrics are gauges, computed from the database at scrape time.

| Metric | Labels | Meaning |
|--------|--------|---------|
| `wiselabz_attention_items` | `kind` = `changes_new`, `alerts_pending`, `findings_open` | Open items needing attention. |
| `wiselabz_connectors` | `status` | Connectors by health status. |
| `wiselabz_sync_runs` | `status` | Retained sync runs by outcome (bounded by sync-run retention, so not a monotonic counter). |
| `wiselabz_sync_run_duration_seconds` | `status` | Summed duration of retained sync runs. |
| `wiselabz_job_up` | `job` | 1 if the scheduled job's last run succeeded, 0 if failing. |
| `wiselabz_job_last_run_timestamp_seconds` | `job` | Unix time of the job's last run. |
| `wiselabz_job_last_success_timestamp_seconds` | `job` | Unix time of the job's last success. |
