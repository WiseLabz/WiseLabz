-- Doc versions queries

-- name: CreateDocVersion :exec
INSERT INTO doc_versions (id, doc_id, rev, content, author, trigger, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: ListDocVersions :many
SELECT * FROM doc_versions WHERE doc_id = ? ORDER BY rev DESC;

-- name: GetDocVersionByRev :one
SELECT * FROM doc_versions WHERE doc_id = ? AND rev = ?;
