CREATE TABLE doc_locks (
    doc_id      TEXT PRIMARY KEY REFERENCES docs(id) ON DELETE CASCADE,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    acquired_at TEXT NOT NULL,
    expires_at  TEXT NOT NULL
);
CREATE INDEX idx_doc_locks_expires_at ON doc_locks(expires_at);
