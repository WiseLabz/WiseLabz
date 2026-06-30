# WiseLabz — Deployment Guide

Supported deployment modes: Docker (primary), docker-compose, and bare Go
binary + systemd (secondary). Kubernetes is not supported — the SQLite
single-writer pool and in-process WebSocket hub assume single-instance
affinity, which contradicts horizontal scaling, and the target scale
(<50 concurrent users, homelab) gives no justification for it.

## Known limitation: PostgreSQL is not yet fully supported

`db.driver: postgres` will run schema migrations successfully (the SQL
migration files in `backend/internal/store/migrations/postgres/` are
portable DDL), but **the application's runtime queries will fail
immediately after**. The entire `backend/internal/store/` query layer
(`store.go`, `user.go`, `doc.go`, `change.go`, `connector.go` — roughly 800
call sites) is written against SQLite syntax:

- `?` positional placeholders, which PostgreSQL drivers (both `pgx` and
  `lib/pq`) do not accept — PostgreSQL requires `$1, $2, ...`. There is no
  driver-level translation between the two; this isn't configurable.
- `INSERT OR IGNORE INTO ...` (3 occurrences in `store.go`), which is
  SQLite-only syntax. The PostgreSQL equivalent is
  `INSERT INTO ... ON CONFLICT DO NOTHING`.

This was discovered by actually running the app against a real PostgreSQL
container: migrations completed, then the very first runtime query
(`initSingletons`, called during startup) failed with
`pq: syntax error at or near "OR"`.

**Until this is fixed, treat `db.driver: postgres` as non-functional.**
SQLite (the default) is the only driver that actually works end-to-end
today. The `docker-compose.yml` Postgres reference stack in this repo will
build and the containers will start, but the app container will crash-loop
on first boot for the same reason — use `docker-compose.sqlite.yml` instead
until this is resolved.

Fixing this properly means either rewriting every query's placeholders and
the 3 `INSERT OR IGNORE` statements to portable/driver-aware SQL, or adding
a query-rewriting layer — both are real engineering work scoped separately
from deployment packaging, not a quick follow-up.

What *is* already correct and ready for when the query layer is fixed:
the `postgres` migration files, the `pgx`-backed `OpenDB` connection
pooling (sized for concurrent writers), and the `RunMigrations` driver
wiring in `backend/internal/store/migrations.go`.

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

**PostgreSQL** (once the query-layer limitation above is fixed): standard
`pg_dump`, e.g. as a cron job against the `postgres` compose service:
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
