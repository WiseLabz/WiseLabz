-- 000041_connector_grant_source.up.sql — track where a connector grant came
-- from (#279 part 3). OIDC group->connector-role mapping now syncs grants at
-- login; those rows must never collide with, or be silently overwritten by,
-- a grant an admin made by hand through the permissions API. Each (user,
-- connector) pair may now hold at most one 'manual' row and one 'oidc' row;
-- GetUserConnectorRole takes the highest of the two.
--
-- SQLite can't add a CHECK-constrained column and widen a UNIQUE constraint
-- in place, so the table is rebuilt under a fresh name and renamed into
-- place (same pattern as 000022_dns_connector_category / 000024).

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
SELECT id, user_id, connector_id, role, 'manual', created_at, updated_at
FROM user_connector_roles;

DROP TABLE user_connector_roles;
ALTER TABLE user_connector_roles_new RENAME TO user_connector_roles;

CREATE INDEX idx_user_connector_roles_user ON user_connector_roles(user_id);
CREATE INDEX idx_user_connector_roles_connector ON user_connector_roles(connector_id);
