-- 000001_init.up.sql — WiseLabz initial schema (portable SQL)
-- Works on both SQLite and PostgreSQL.
-- UUIDs: application-generated TEXT
-- Booleans: INTEGER CHECK(0,1)
-- Timestamps: ISO-8601 TEXT
-- JSON: TEXT

CREATE TABLE users (
    id              TEXT PRIMARY KEY,
    username        TEXT NOT NULL UNIQUE,
    display_name    TEXT NOT NULL DEFAULT '',
    email           TEXT NOT NULL DEFAULT '',
    role            TEXT NOT NULL DEFAULT 'viewer' CHECK(role IN ('viewer','operator')),
    auth_source     TEXT NOT NULL DEFAULT 'local' CHECK(auth_source IN ('local','oidc')),
    password_hash   TEXT NOT NULL DEFAULT '',
    disabled        INTEGER NOT NULL DEFAULT 0 CHECK(disabled IN (0,1)),
    created_at      TEXT NOT NULL
);

CREATE TABLE sessions (
    id              TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash      TEXT NOT NULL,
    user_agent      TEXT NOT NULL DEFAULT '',
    ip              TEXT NOT NULL DEFAULT '',
    created_at      TEXT NOT NULL,
    last_seen_at    TEXT NOT NULL
);
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_token_hash ON sessions(token_hash);

CREATE TABLE oidc_provider_flags (
    provider_id     TEXT PRIMARY KEY,
    enabled         INTEGER NOT NULL DEFAULT 1 CHECK(enabled IN (0,1))
);

CREATE TABLE connectors (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    category        TEXT NOT NULL CHECK(category IN ('virtualization','containers_paas','networking')),
    type            TEXT NOT NULL,
    url             TEXT NOT NULL,
    verify_tls      INTEGER NOT NULL DEFAULT 1 CHECK(verify_tls IN (0,1)),
    config_data     TEXT NOT NULL DEFAULT '{}',
    enabled         INTEGER NOT NULL DEFAULT 1 CHECK(enabled IN (0,1)),
    status          TEXT NOT NULL DEFAULT 'unknown' CHECK(status IN ('online','degraded','offline','unknown')),
    status_message  TEXT NOT NULL DEFAULT '',
    last_sync_at    TEXT,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);
CREATE INDEX idx_connectors_category ON connectors(category);

CREATE TABLE service_snapshots (
    id              TEXT PRIMARY KEY,
    connector_id    TEXT NOT NULL REFERENCES connectors(id) ON DELETE CASCADE,
    data            TEXT NOT NULL,
    fetched_at      TEXT NOT NULL
);
CREATE INDEX idx_snapshots_connector ON service_snapshots(connector_id, fetched_at DESC);

CREATE TABLE docs (
    id              TEXT PRIMARY KEY,
    title           TEXT NOT NULL,
    kind            TEXT NOT NULL CHECK(kind IN ('lab','service')),
    service_id      TEXT REFERENCES connectors(id) ON DELETE SET NULL,
    content         TEXT NOT NULL DEFAULT '',
    current_version INTEGER NOT NULL DEFAULT 1,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);
CREATE INDEX idx_docs_service ON docs(service_id);

CREATE TABLE doc_versions (
    id              TEXT PRIMARY KEY,
    doc_id          TEXT NOT NULL REFERENCES docs(id) ON DELETE CASCADE,
    rev             INTEGER NOT NULL,
    content         TEXT NOT NULL,
    author          TEXT,
    trigger         TEXT NOT NULL CHECK(trigger IN ('ai','template','manual')),
    created_at      TEXT NOT NULL
);
CREATE UNIQUE INDEX idx_doc_versions_doc_rev ON doc_versions(doc_id, rev DESC);

CREATE TABLE templates (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    applies_to      TEXT,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);

CREATE TABLE template_sections (
    id              TEXT PRIMARY KEY,
    template_id     TEXT NOT NULL REFERENCES templates(id) ON DELETE CASCADE,
    title           TEXT NOT NULL,
    ord             INTEGER NOT NULL DEFAULT 0,
    body            TEXT NOT NULL
);
CREATE UNIQUE INDEX idx_template_sections_tmpl_ord ON template_sections(template_id, ord);

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
CREATE INDEX idx_changes_service ON changes(service_id);
CREATE INDEX idx_changes_detected ON changes(detected_at DESC);

CREATE TABLE alerts (
    id              TEXT PRIMARY KEY,
    change_id       TEXT REFERENCES changes(id) ON DELETE SET NULL,
    service_id      TEXT NOT NULL REFERENCES connectors(id) ON DELETE CASCADE,
    severity        TEXT NOT NULL CHECK(severity IN ('info','warning','critical')),
    title           TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','resolved','dismissed','snoozed')),
    snoozed_until   TEXT,
    created_at      TEXT NOT NULL
);
CREATE INDEX idx_alerts_service ON alerts(service_id);
CREATE INDEX idx_alerts_status ON alerts(status);
CREATE INDEX idx_alerts_created ON alerts(created_at DESC);

CREATE TABLE dashboard_layouts (
    user_id         TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    widgets         TEXT NOT NULL DEFAULT '[]'
);

CREATE TABLE auth_config (
    id                      INTEGER PRIMARY KEY CHECK(id = 1),
    local_enabled           INTEGER NOT NULL DEFAULT 1 CHECK(local_enabled IN (0,1)),
    access_token_ttl        INTEGER NOT NULL DEFAULT 900,
    refresh_token_ttl       INTEGER NOT NULL DEFAULT 604800,
    step_up_for_destructive INTEGER NOT NULL DEFAULT 1 CHECK(step_up_for_destructive IN (0,1))
);

CREATE TABLE ai_config (
    id                  INTEGER PRIMARY KEY CHECK(id = 1),
    enabled             INTEGER NOT NULL DEFAULT 0 CHECK(enabled IN (0,1)),
    provider            TEXT,
    model               TEXT,
    api_key_encrypted   TEXT NOT NULL DEFAULT '',
    base_url            TEXT,
    mode                TEXT NOT NULL DEFAULT 'suggest_only' CHECK(mode IN ('auto_update','suggest_only'))
);

CREATE TABLE notification_config (
    id              INTEGER PRIMARY KEY CHECK(id = 1),
    config_json     TEXT NOT NULL DEFAULT '{}'
);

CREATE TABLE in_app_notifications (
    id              TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    alert_id        TEXT,
    event_type      TEXT NOT NULL,
    title           TEXT NOT NULL,
    message         TEXT NOT NULL DEFAULT '',
    read            INTEGER NOT NULL DEFAULT 0 CHECK(read IN (0,1)),
    created_at      TEXT NOT NULL
);
CREATE INDEX idx_inapp_user ON in_app_notifications(user_id, created_at DESC);
