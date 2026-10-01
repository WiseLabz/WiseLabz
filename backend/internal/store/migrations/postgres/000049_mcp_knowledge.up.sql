-- Full-text search over docs and runbooks (generated tsvector + GIN),
-- persisted entity-level topology edges, and reviewable doc edit proposals.

ALTER TABLE docs ADD COLUMN search_tsv tsvector
    GENERATED ALWAYS AS (to_tsvector('english', coalesce(title, '') || ' ' || coalesce(content, ''))) STORED;
CREATE INDEX idx_docs_search ON docs USING GIN (search_tsv);

ALTER TABLE runbooks ADD COLUMN search_tsv tsvector
    GENERATED ALWAYS AS (to_tsvector('english', coalesce(title, '') || ' ' || coalesce(body, ''))) STORED;
CREATE INDEX idx_runbooks_search ON runbooks USING GIN (search_tsv);

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
CREATE INDEX idx_topology_edges_src ON topology_edges(src_connector_id);
CREATE INDEX idx_topology_edges_dst ON topology_edges(dst_connector_id);

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
