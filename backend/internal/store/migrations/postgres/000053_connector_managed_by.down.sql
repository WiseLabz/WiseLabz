-- 000053_connector_managed_by.down.sql
-- Drops config-sourced grants (the older schema cannot hold them) and returns
-- every connector to plain UI management.

DELETE FROM user_connector_roles WHERE source = 'config';
ALTER TABLE user_connector_roles DROP CONSTRAINT user_connector_roles_source_check;
ALTER TABLE user_connector_roles ADD CONSTRAINT user_connector_roles_source_check
    CHECK(source IN ('manual','oidc'));

ALTER TABLE connectors DROP CONSTRAINT connectors_managed_by_check;
ALTER TABLE connectors DROP COLUMN config_hash;
ALTER TABLE connectors DROP COLUMN managed_by;
