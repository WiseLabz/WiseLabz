-- Template sections queries

-- name: CreateTemplateSection :exec
INSERT INTO template_sections (id, template_id, title, ord, body)
VALUES (?, ?, ?, ?, ?);

-- name: ListTemplateSections :many
SELECT * FROM template_sections WHERE template_id = ? ORDER BY ord;

-- name: DeleteTemplateSections :exec
DELETE FROM template_sections WHERE template_id = ?;
