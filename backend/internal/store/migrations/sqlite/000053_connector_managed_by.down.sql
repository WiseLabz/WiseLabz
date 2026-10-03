-- 000053_connector_managed_by.down.sql
-- Drops config-sourced grants (the older schema cannot hold them) and returns
-- every connector to plain UI management.

CREATE TABLE user_connector_roles_new (
    id              TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    connector_id    TEXT NOT NULL REFERENCES connectors(id) ON DELETE CASCADE,
    role            TEXT NOT NULL CHECK(role IN ('viewer','operator')),
    source          TEXT NOT NULL DEFAULT 'manual' CHECK(source IN ('manual','oidc')),
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL,
    UNIQUE(user_id, connector_id, source)
);

INSERT INTO user_connector_roles_new (id, user_id, connector_id, role, source, created_at, updated_at)
SELECT id, user_id, connector_id, role, source, created_at, updated_at
FROM user_connector_roles
WHERE source <> 'config';

DROP TABLE user_connector_roles;
ALTER TABLE user_connector_roles_new RENAME TO user_connector_roles;

CREATE INDEX idx_user_connector_roles_user ON user_connector_roles(user_id);
CREATE INDEX idx_user_connector_roles_connector ON user_connector_roles(connector_id);

ALTER TABLE connectors DROP COLUMN config_hash;
ALTER TABLE connectors DROP COLUMN managed_by;
