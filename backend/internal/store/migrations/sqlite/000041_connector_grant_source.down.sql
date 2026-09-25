-- 000041_connector_grant_source.down.sql
-- Collapses back to one row per (user, connector), keeping the higher role
-- when both a manual and an oidc row exist for the same pair, then drops
-- the source column and restores the original UNIQUE(user_id, connector_id).

CREATE TABLE user_connector_roles_new (
    id              TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    connector_id    TEXT NOT NULL REFERENCES connectors(id) ON DELETE CASCADE,
    role            TEXT NOT NULL CHECK(role IN ('viewer','operator')),
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL,
    UNIQUE(user_id, connector_id)
);

INSERT INTO user_connector_roles_new (id, user_id, connector_id, role, created_at, updated_at)
SELECT id, user_id, connector_id, role, created_at, updated_at
FROM user_connector_roles r
WHERE r.id = (
    SELECT r2.id FROM user_connector_roles r2
    WHERE r2.user_id = r.user_id AND r2.connector_id = r.connector_id
    ORDER BY CASE r2.role WHEN 'operator' THEN 2 WHEN 'viewer' THEN 1 ELSE 0 END DESC, r2.created_at ASC
    LIMIT 1
);

DROP TABLE user_connector_roles;
ALTER TABLE user_connector_roles_new RENAME TO user_connector_roles;

CREATE INDEX idx_user_connector_roles_user ON user_connector_roles(user_id);
CREATE INDEX idx_user_connector_roles_connector ON user_connector_roles(connector_id);
