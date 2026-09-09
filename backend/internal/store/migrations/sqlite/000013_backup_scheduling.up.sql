-- 000013_backup_scheduling.up.sql — backup scheduling and run history (issue #111)

CREATE TABLE backup_schedule (
    id             TEXT PRIMARY KEY CHECK(id = 'default'),
    cron_expr      TEXT NOT NULL,
    max_backups    INTEGER NOT NULL,
    max_age_hours  INTEGER NOT NULL,
    enabled        INTEGER NOT NULL CHECK(enabled IN (0,1)),
    updated_at     TEXT NOT NULL
);

CREATE TABLE backup_runs (
    id             TEXT PRIMARY KEY,
    triggered_by   TEXT NOT NULL CHECK(triggered_by IN ('schedule', 'manual')),
    file_path      TEXT NOT NULL,
    size_bytes     INTEGER NOT NULL,
    created_at     TEXT NOT NULL
);

CREATE INDEX idx_backup_runs_created ON backup_runs(created_at DESC);
