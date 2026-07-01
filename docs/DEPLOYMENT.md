# WiseLabz — Deployment Guide

Supported deployment modes: Docker (primary), docker-compose, and bare Go
binary + systemd (secondary). Kubernetes is not supported — the SQLite
single-writer pool and in-process WebSocket hub assume single-instance
affinity, which contradicts horizontal scaling, and the target scale
(<50 concurrent users, homelab) gives no justification for it.

## PostgreSQL support

`db.driver: postgres` runs schema migrations (the SQL migration files in
`backend/internal/store/migrations/postgres/` are portable DDL) and then
runs the application's runtime queries normally. The `backend/internal/store/`
query layer is written using SQLite-style `?` placeholders; when
`db.driver` is `postgres`, `Store` wraps the connection
(`backend/internal/store/pgdb.go`) to rewrite `?` placeholders to
PostgreSQL's `$1, $2, ...` form transparently, so no query code needs to
differ between drivers.

Both the `docker-compose.yml` (Postgres) and `docker-compose.sqlite.yml`
reference stacks are fully functional end-to-end.

## WebSocket behind a reverse proxy

`/api/ws` shares the same port and origin as the rest of the HTTP API — no
separate service or port. Reverse proxy behavior varies:

- **Traefik / Caddy**: handle WebSocket upgrades automatically, no extra
  config needed.
- **nginx**: needs explicit `proxy_set_header Upgrade $http_upgrade;`,
  `proxy_set_header Connection "upgrade";`, `proxy_http_version 1.1;`, and a
  bumped `proxy_read_timeout` (nginx's default 60s will silently drop idle
  WebSocket connections).
- **Cloudflare (orange-cloud/proxied)**: has a ~100s idle timeout on
  WebSocket connections. Either send an application-level heartbeat/ping
  more frequently than that, or set the DNS record to "DNS only" (grey
  cloud) to bypass Cloudflare's proxy for this traffic.

## Backups

**SQLite**: either stop the container and copy the `.db` file directly, or
use `sqlite3 /data/wiselabz.db ".backup /data/backup.db"` while the app is
running (SQLite's online backup API, safe under concurrent access). Example
cron entry on the host (bare-metal/systemd mode):
```
0 3 * * * sqlite3 /opt/wiselabz/data/wiselabz.db ".backup /opt/wiselabz/backups/wiselabz-$(date +\%Y\%m\%d).db"
```

**PostgreSQL**: standard `pg_dump`, e.g. as a cron job against the
`postgres` compose service:
```
0 3 * * * docker compose exec -T postgres pg_dump -U wiselabz wiselabz | gzip > backups/wiselabz-$(date +\%Y\%m\%d).sql.gz
```

## systemd (bare binary)

A reference unit file is at `deploy/wiselabz.service`. Key points:
- `WorkingDirectory=` is pinned to avoid relative-path surprises (the
  default DSN is now an absolute path, `/data/wiselabz.db`, but pin this
  anyway for config file resolution).
- `EnvironmentFile=` should point at a `root:wiselabz`-owned, mode `0640`
  file holding `WISELABZ_AUTH_SECRET` / `WISELABZ_ADMIN_PASSWORD` / etc.
- Default `StandardOutput=journal` is sufficient — no extra logging config
  needed; use `journalctl -u wiselabz`.
- `TimeoutStopSec=15s` gives the app's graceful shutdown
  (`Server.ShutdownTimeoutSeconds`, default 10s) room to finish before
  systemd sends `SIGKILL`.
