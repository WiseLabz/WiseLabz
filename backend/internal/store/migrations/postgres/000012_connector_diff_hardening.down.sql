-- 000012_connector_diff_hardening.down.sql

DROP INDEX IF EXISTS idx_changes_pattern;
ALTER TABLE changes DROP COLUMN pattern_id;
ALTER TABLE changes DROP COLUMN related_service_ids;
ALTER TABLE connectors DROP COLUMN credential_expires_at;
