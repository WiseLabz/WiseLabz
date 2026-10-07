-- 000064_connector_categories.down.sql
-- Restricts categories back to virtualization, containers_paas, networking, dns.
-- Rollback note: when any row uses storage, monitoring, media or other, the
-- down migration fails on the CHECK constraint, the transaction rolls back and
-- no row is changed or deleted. As with any failed golang-migrate step the
-- version is left marked dirty (63, while the schema is still the 000064 one).
-- To recover, move or delete those connectors and set the migration version
-- back (UPDATE schema_migrations SET version = 64, dirty = false), then retry.

DROP INDEX IF EXISTS idx_connectors_category;

CREATE TABLE connectors_new (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    category        TEXT NOT NULL CHECK(category IN ('virtualization','containers_paas','networking','dns')),
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
    credential_expires_at  TEXT,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL,
    secret_rotated_at TEXT,
    user_expires_at TEXT,
    rotation_max_age_days INTEGER,
    managed_by      TEXT NOT NULL DEFAULT 'ui' CHECK(managed_by IN ('ui','config','config-orphaned')),
    config_hash     TEXT NOT NULL DEFAULT ''
);

INSERT INTO connectors_new (id, name, category, type, url, owner, verify_tls, config_data, enabled, status, status_message,
    last_sync_at, schedule_seconds, next_run_at, last_sync_duration_ms, last_sync_error, retry_count, credential_expires_at,
    created_at, updated_at, secret_rotated_at, user_expires_at, rotation_max_age_days, managed_by, config_hash)
SELECT id, name, category, type, url, owner, verify_tls, config_data, enabled, status, status_message,
    last_sync_at, schedule_seconds, next_run_at, last_sync_duration_ms, last_sync_error, retry_count, credential_expires_at,
    created_at, updated_at, secret_rotated_at, user_expires_at, rotation_max_age_days, managed_by, config_hash
FROM connectors;

DROP TABLE connectors;

ALTER TABLE connectors_new RENAME TO connectors;

CREATE INDEX idx_connectors_category ON connectors(category);
