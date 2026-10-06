ALTER TABLE retention_settings ADD COLUMN runbook_open_run_hours INTEGER NOT NULL DEFAULT 24;
ALTER TABLE retention_settings ADD COLUMN runbook_run_days INTEGER NOT NULL DEFAULT 90;
