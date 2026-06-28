-- Docs queries

-- name: CreateDoc :exec
INSERT INTO docs (id, title, kind, service_id, content, current_version, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetDocByID :one
SELECT * FROM docs WHERE id = ?;

-- name: GetDocByServiceID :one
SELECT * FROM docs WHERE service_id = ?;

-- name: ListDocs :many
SELECT * FROM docs ORDER BY created_at DESC;

-- name: UpdateDoc :exec
UPDATE docs
SET title = ?, content = ?, current_version = ?, updated_at = ?
WHERE id = ?;

-- name: DeleteDoc :exec
DELETE FROM docs WHERE id = ?;
