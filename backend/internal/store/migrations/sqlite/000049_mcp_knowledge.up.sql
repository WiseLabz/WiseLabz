-- Full-text search over docs and runbooks (standalone FTS5 tables kept in sync
-- by triggers so every write path, including backup import, is covered),
-- persisted entity-level topology edges, and reviewable doc edit proposals.

-- id is UNINDEXED (stored, not tokenized), so the trigger DELETE ... WHERE id = ?
-- scans the FTS table. That is deliberate: mapping to docs.rowid instead would
-- be O(log n) but docs/runbooks have TEXT primary keys, so SQLite may renumber
-- their implicit rowids on VACUUM and silently desynchronize the index. The
-- corpus is small (docs and runbooks of one lab), so correctness wins.
CREATE VIRTUAL TABLE docs_fts USING fts5(id UNINDEXED, title, content, tokenize = 'porter unicode61');
INSERT INTO docs_fts (id, title, content) SELECT id, title, content FROM docs;

CREATE TRIGGER docs_fts_ai AFTER INSERT ON docs BEGIN
    INSERT INTO docs_fts (id, title, content) VALUES (new.id, new.title, new.content);
END;
CREATE TRIGGER docs_fts_au AFTER UPDATE OF title, content ON docs BEGIN
    DELETE FROM docs_fts WHERE id = old.id;
    INSERT INTO docs_fts (id, title, content) VALUES (new.id, new.title, new.content);
END;
CREATE TRIGGER docs_fts_ad AFTER DELETE ON docs BEGIN
    DELETE FROM docs_fts WHERE id = old.id;
END;

CREATE VIRTUAL TABLE runbooks_fts USING fts5(id UNINDEXED, title, body, tokenize = 'porter unicode61');
INSERT INTO runbooks_fts (id, title, body) SELECT id, title, body FROM runbooks;

CREATE TRIGGER runbooks_fts_ai AFTER INSERT ON runbooks BEGIN
    INSERT INTO runbooks_fts (id, title, body) VALUES (new.id, new.title, new.body);
END;
CREATE TRIGGER runbooks_fts_au AFTER UPDATE OF title, body ON runbooks BEGIN
    DELETE FROM runbooks_fts WHERE id = old.id;
    INSERT INTO runbooks_fts (id, title, body) VALUES (new.id, new.title, new.body);
END;
CREATE TRIGGER runbooks_fts_ad AFTER DELETE ON runbooks BEGIN
    DELETE FROM runbooks_fts WHERE id = old.id;
END;

CREATE TABLE topology_edges (
    id               TEXT PRIMARY KEY,
    connector_id     TEXT NOT NULL REFERENCES connectors(id) ON DELETE CASCADE,
    src_connector_id TEXT NOT NULL REFERENCES connectors(id) ON DELETE CASCADE,
    src_kind         TEXT NOT NULL,
    src_name         TEXT NOT NULL,
    src_ref          TEXT NOT NULL DEFAULT '',
    dst_connector_id TEXT NOT NULL REFERENCES connectors(id) ON DELETE CASCADE,
    dst_kind         TEXT NOT NULL,
    dst_name         TEXT NOT NULL,
    dst_ref          TEXT NOT NULL DEFAULT '',
    kind             TEXT NOT NULL,
    source           TEXT NOT NULL DEFAULT '',
    created_at       TEXT NOT NULL
);
CREATE INDEX idx_topology_edges_connector ON topology_edges(connector_id);
-- (connector, kind) in both directions: ListTopologyEdges filters on the
-- endpoint connectors, and the same_as cleanup in
-- ReplaceTopologyEdgesForConnector matches kind plus either endpoint.
CREATE INDEX idx_topology_edges_src ON topology_edges(src_connector_id, kind);
CREATE INDEX idx_topology_edges_dst ON topology_edges(dst_connector_id, kind);

CREATE TABLE doc_edit_proposals (
    id           TEXT PRIMARY KEY,
    doc_id       TEXT NOT NULL REFERENCES docs(id) ON DELETE CASCADE,
    base_version INTEGER NOT NULL,
    content      TEXT NOT NULL,
    summary      TEXT NOT NULL DEFAULT '',
    author_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status       TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','approved','rejected')),
    reviewer_id  TEXT REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at  TEXT,
    created_at   TEXT NOT NULL
);
CREATE INDEX idx_doc_edit_proposals_doc ON doc_edit_proposals(doc_id, status);
CREATE INDEX idx_doc_edit_proposals_status ON doc_edit_proposals(status, created_at);
