-- 000010_dashboard_admin_default.down.sql — revert admin-defined default dashboard layout (issue #94)

DROP TABLE dashboard_admin_default;
ALTER TABLE users DROP COLUMN can_manage_dashboard_defaults;
