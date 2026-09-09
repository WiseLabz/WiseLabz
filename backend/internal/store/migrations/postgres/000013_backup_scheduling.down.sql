-- 000013_backup_scheduling.down.sql

DROP INDEX IF EXISTS idx_backup_runs_created;
DROP TABLE IF EXISTS backup_runs;
DROP TABLE IF EXISTS backup_schedule;
