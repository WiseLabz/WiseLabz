-- 000017_retention_settings.down.sql

DROP INDEX IF EXISTS idx_audit_log_target_type;
DROP TABLE IF EXISTS retention_settings;
