-- 000053_connector_managed_by.up.sql — connectors declared in config.yaml (#500).
-- See the sqlite migration of the same number for the full rationale.

ALTER TABLE connectors ADD COLUMN managed_by TEXT NOT NULL DEFAULT 'ui';
ALTER TABLE connectors ADD CONSTRAINT connectors_managed_by_check
    CHECK(managed_by IN ('ui','config','config-orphaned'));
-- config_hash fingerprints the config entry last applied to the row, so a
-- restart with an unchanged entry writes nothing and never overwrites
-- credentials the connector refreshed at runtime.
ALTER TABLE connectors ADD COLUMN config_hash TEXT NOT NULL DEFAULT '';

ALTER TABLE user_connector_roles DROP CONSTRAINT user_connector_roles_source_check;
ALTER TABLE user_connector_roles ADD CONSTRAINT user_connector_roles_source_check
    CHECK(source IN ('manual','oidc','config'));
