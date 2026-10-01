-- 000047_retention_scan_indexes.up.sql — single-column indexes for the
-- retention batch deletes, which filter on the timestamp alone and otherwise
-- rescan the table on every batch (#468).
CREATE INDEX IF NOT EXISTS idx_health_checks_checked_at ON health_checks(checked_at);
CREATE INDEX IF NOT EXISTS idx_inapp_created ON in_app_notifications(created_at);
