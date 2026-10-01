DROP TABLE doc_edit_proposals;
DROP TABLE topology_edges;
DROP INDEX idx_runbooks_search;
ALTER TABLE runbooks DROP COLUMN search_tsv;
DROP INDEX idx_docs_search;
ALTER TABLE docs DROP COLUMN search_tsv;
