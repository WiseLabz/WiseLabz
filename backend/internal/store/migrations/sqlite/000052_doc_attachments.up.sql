CREATE TABLE doc_attachments (
    id TEXT PRIMARY KEY,
    doc_id TEXT NOT NULL REFERENCES docs(id) ON DELETE CASCADE,
    sha256 TEXT NOT NULL CHECK (length(sha256) = 64),
    filename TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size BIGINT NOT NULL CHECK (size >= 0),
    created_by TEXT NOT NULL,
    created_at TEXT NOT NULL
);
CREATE INDEX idx_doc_attachments_doc ON doc_attachments(doc_id);
CREATE INDEX idx_doc_attachments_sha ON doc_attachments(sha256);
