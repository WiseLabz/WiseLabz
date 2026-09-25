-- 000041_connector_grant_source.up.sql — track where a connector grant came
-- from (#279 part 3). See the sqlite migration of the same number for the
-- full rationale: OIDC group->connector-role mapping now syncs grants at
-- login, and those rows must never collide with, or be silently overwritten
-- by, a grant an admin made by hand through the permissions API.

ALTER TABLE user_connector_roles ADD COLUMN source TEXT NOT NULL DEFAULT 'manual';
ALTER TABLE user_connector_roles ADD CONSTRAINT user_connector_roles_source_check CHECK(source IN ('manual','oidc'));

ALTER TABLE user_connector_roles DROP CONSTRAINT user_connector_roles_user_id_connector_id_key;
ALTER TABLE user_connector_roles ADD CONSTRAINT user_connector_roles_user_id_connector_id_source_key UNIQUE(user_id, connector_id, source);
