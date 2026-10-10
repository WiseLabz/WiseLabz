CREATE TABLE doc_links (
    source_doc_id TEXT NOT NULL REFERENCES docs(id) ON DELETE CASCADE,
    target_type TEXT NOT NULL CHECK (target_type IN ('doc', 'entity')),
    target_id TEXT NOT NULL,
    PRIMARY KEY (source_doc_id, target_type, target_id)
);
CREATE INDEX idx_doc_links_target ON doc_links(target_type, target_id);
