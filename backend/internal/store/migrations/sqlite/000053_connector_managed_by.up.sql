-- 000053_connector_managed_by.up.sql — connectors declared in config.yaml (#500).
-- managed_by records who owns a connector's settings: 'ui' (the default),
-- 'config' (reconciled from config.yaml at startup, locked in the UI) or
-- 'config-orphaned' (its config entry was removed; disabled, kept for review).
-- Grants declared in config get their own source so reconciling them never
-- touches a 'manual' or 'oidc' grant.

ALTER TABLE connectors ADD COLUMN managed_by TEXT NOT NULL DEFAULT 'ui'
    CHECK(managed_by IN ('ui','config','config-orphaned'));
-- config_hash fingerprints the config entry last applied to the row, so a
-- restart with an unchanged entry writes nothing and never overwrites
-- credentials the connector refreshed at runtime.
ALTER TABLE connectors ADD COLUMN config_hash TEXT NOT NULL DEFAULT '';

-- SQLite cannot alter a CHECK constraint in place, so the grants table is
-- rebuilt under a fresh name and renamed (same pattern as 000041).
CREATE TABLE user_connector_roles_new (
    id              TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    connector_id    TEXT NOT NULL REFERENCES connectors(id) ON DELETE CASCADE,
    role            TEXT NOT NULL CHECK(role IN ('viewer','operator')),
    source          TEXT NOT NULL DEFAULT 'manual' CHECK(source IN ('manual','oidc','config')),
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL,
    UNIQUE(user_id, connector_id, source)
);

INSERT INTO user_connector_roles_new (id, user_id, connector_id, role, source, created_at, updated_at)
SELECT id, user_id, connector_id, role, source, created_at, updated_at
FROM user_connector_roles;

DROP TABLE user_connector_roles;
ALTER TABLE user_connector_roles_new RENAME TO user_connector_roles;

CREATE INDEX idx_user_connector_roles_user ON user_connector_roles(user_id);
CREATE INDEX idx_user_connector_roles_connector ON user_connector_roles(connector_id);
