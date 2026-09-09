-- 000011_flexible_doc_trigger.down.sql — Restore the original CHECK constraint (SQLite)

-- Drop the old index
DROP INDEX IF EXISTS idx_doc_versions_doc_rev;

-- Rename old table
ALTER TABLE doc_versions RENAME TO doc_versions_old;

-- Recreate table with CHECK constraint
CREATE TABLE doc_versions (
    id              TEXT PRIMARY KEY,
    doc_id          TEXT NOT NULL REFERENCES docs(id) ON DELETE CASCADE,
    rev             INTEGER NOT NULL,
    content         TEXT NOT NULL,
    author          TEXT,
    trigger         TEXT NOT NULL CHECK(trigger IN ('ai','template','manual')),
    created_at      TEXT NOT NULL
);

-- Copy data from old table
INSERT INTO doc_versions (id, doc_id, rev, content, author, trigger, created_at)
SELECT id, doc_id, rev, content, author, trigger, created_at FROM doc_versions_old;

-- Drop old table
DROP TABLE doc_versions_old;

-- Recreate the index
CREATE UNIQUE INDEX idx_doc_versions_doc_rev ON doc_versions(doc_id, rev DESC);
