-- 000041_connector_grant_source.down.sql
-- Collapses back to one row per (user, connector), keeping the higher role
-- when both a manual and an oidc row exist for the same pair, then drops
-- the source column and restores the original UNIQUE(user_id, connector_id).

DELETE FROM user_connector_roles r
WHERE r.id NOT IN (
    SELECT DISTINCT ON (r2.user_id, r2.connector_id) r2.id
    FROM user_connector_roles r2
    ORDER BY r2.user_id, r2.connector_id,
        CASE r2.role WHEN 'operator' THEN 2 WHEN 'viewer' THEN 1 ELSE 0 END DESC, r2.created_at ASC
);

ALTER TABLE user_connector_roles DROP CONSTRAINT user_connector_roles_user_id_connector_id_source_key;
ALTER TABLE user_connector_roles ADD CONSTRAINT user_connector_roles_user_id_connector_id_key UNIQUE(user_id, connector_id);

ALTER TABLE user_connector_roles DROP CONSTRAINT user_connector_roles_source_check;
ALTER TABLE user_connector_roles DROP COLUMN source;
