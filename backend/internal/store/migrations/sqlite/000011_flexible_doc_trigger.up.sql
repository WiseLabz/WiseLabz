-- 000011_flexible_doc_trigger.up.sql — Allow flexible trigger values for doc versions (SQLite)
-- SQLite doesn't support direct ALTER TABLE constraint drops, so we recreate the table
-- without the CHECK constraint.

-- Drop the old index
DROP INDEX IF EXISTS idx_doc_versions_doc_rev;

-- Rename old table
ALTER TABLE doc_versions RENAME TO doc_versions_old;

-- Create new table without CHECK constraint
CREATE TABLE doc_versions (
    id              TEXT PRIMARY KEY,
    doc_id          TEXT NOT NULL REFERENCES docs(id) ON DELETE CASCADE,
    rev             INTEGER NOT NULL,
    content         TEXT NOT NULL,
    author          TEXT,
    trigger         TEXT NOT NULL,
    created_at      TEXT NOT NULL
);

-- Copy data from old table
INSERT INTO doc_versions (id, doc_id, rev, content, author, trigger, created_at)
SELECT id, doc_id, rev, content, author, trigger, created_at FROM doc_versions_old;

-- Drop old table
DROP TABLE doc_versions_old;

-- Recreate the index
CREATE UNIQUE INDEX idx_doc_versions_doc_rev ON doc_versions(doc_id, rev DESC);
