DROP INDEX idx_docs_scope_parent;
DROP INDEX idx_docs_deleted_at;
ALTER TABLE docs DROP COLUMN parent_id;
ALTER TABLE docs DROP COLUMN deleted_at;
ALTER TABLE docs DROP COLUMN created_by;
ALTER TABLE retention_settings DROP COLUMN deleted_docs_days;
