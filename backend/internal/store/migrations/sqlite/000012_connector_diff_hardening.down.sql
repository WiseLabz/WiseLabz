-- 000012_connector_diff_hardening.down.sql

DROP INDEX IF EXISTS idx_changes_pattern;

-- SQLite doesn't support DROP COLUMN before 3.35; recreate the tables instead.
ALTER TABLE connectors RENAME TO connectors_old;
CREATE TABLE connectors (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    category        TEXT NOT NULL CHECK(category IN ('virtualization','containers_paas','networking')),
    type            TEXT NOT NULL,
    url             TEXT NOT NULL,
    owner           TEXT,
    verify_tls      INTEGER NOT NULL DEFAULT 1 CHECK(verify_tls IN (0,1)),
    config_data     TEXT NOT NULL DEFAULT '{}',
    enabled         INTEGER NOT NULL DEFAULT 1 CHECK(enabled IN (0,1)),
    status          TEXT NOT NULL DEFAULT 'unknown' CHECK(status IN ('online','degraded','offline','unknown')),
    status_message  TEXT NOT NULL DEFAULT '',
    last_sync_at    TEXT,
    schedule_seconds       INTEGER,
    next_run_at            TEXT,
    last_sync_duration_ms  INTEGER,
    last_sync_error        TEXT NOT NULL DEFAULT '',
    retry_count            INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);
INSERT INTO connectors (id, name, category, type, url, owner, verify_tls, config_data, enabled, status, status_message,
    last_sync_at, schedule_seconds, next_run_at, last_sync_duration_ms, last_sync_error, retry_count, created_at, updated_at)
SELECT id, name, category, type, url, owner, verify_tls, config_data, enabled, status, status_message,
    last_sync_at, schedule_seconds, next_run_at, last_sync_duration_ms, last_sync_error, retry_count, created_at, updated_at
FROM connectors_old;
DROP TABLE connectors_old;
CREATE INDEX idx_connectors_category ON connectors(category);

ALTER TABLE changes RENAME TO changes_old;
CREATE TABLE changes (
    id              TEXT PRIMARY KEY,
    service_id      TEXT NOT NULL REFERENCES connectors(id) ON DELETE CASCADE,
    change_type     TEXT NOT NULL,
    severity        TEXT NOT NULL CHECK(severity IN ('info','warning','critical')),
    summary         TEXT NOT NULL,
    diff            TEXT NOT NULL DEFAULT '{}',
    status          TEXT NOT NULL DEFAULT 'new' CHECK(status IN ('new','acknowledged','dismissed')),
    detected_at     TEXT NOT NULL,
    affected_doc_ids TEXT NOT NULL DEFAULT '[]'
);
INSERT INTO changes (id, service_id, change_type, severity, summary, diff, status, detected_at, affected_doc_ids)
SELECT id, service_id, change_type, severity, summary, diff, status, detected_at, affected_doc_ids
FROM changes_old;
DROP TABLE changes_old;
CREATE INDEX idx_changes_service ON changes(service_id);
CREATE INDEX idx_changes_detected ON changes(detected_at DESC);
