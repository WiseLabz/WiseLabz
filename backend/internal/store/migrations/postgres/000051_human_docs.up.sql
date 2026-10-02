ALTER TABLE docs ADD COLUMN parent_id TEXT REFERENCES docs(id) ON DELETE SET NULL;
ALTER TABLE docs ADD COLUMN deleted_at TEXT;
ALTER TABLE docs ADD COLUMN created_by TEXT;
CREATE INDEX idx_docs_scope_parent ON docs(service_id, parent_id);
CREATE INDEX idx_docs_deleted_at ON docs(deleted_at);
ALTER TABLE retention_settings ADD COLUMN deleted_docs_days INTEGER NOT NULL DEFAULT 30;
