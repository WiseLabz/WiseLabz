-- 000017_retention_settings.up.sql — DB-backed retention settings (issue #138)
-- and a companion index for audit_log target_type filtering (issue #140).

CREATE TABLE retention_settings (
    id                TEXT PRIMARY KEY CHECK(id = 'default'),
    snapshot_days     INTEGER NOT NULL,
    doc_version_days  INTEGER NOT NULL,
    alert_days        INTEGER NOT NULL,
    sync_run_days     INTEGER NOT NULL,
    audit_days        INTEGER NOT NULL,
    cron_expr         TEXT NOT NULL,
    updated_at        TEXT NOT NULL
);

CREATE INDEX idx_audit_log_target_type ON audit_log(target_type);
