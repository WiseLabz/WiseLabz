-- Templates queries

-- name: CreateTemplate :exec
INSERT INTO templates (id, name, description, applies_to, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: GetTemplateByID :one
SELECT * FROM templates WHERE id = ?;

-- name: ListTemplates :many
SELECT * FROM templates ORDER BY created_at DESC;

-- name: UpdateTemplate :exec
UPDATE templates
SET name = ?, description = ?, applies_to = ?, updated_at = ?
WHERE id = ?;

-- name: DeleteTemplate :exec
DELETE FROM templates WHERE id = ?;
