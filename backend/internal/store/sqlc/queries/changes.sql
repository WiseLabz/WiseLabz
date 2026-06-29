-- Changes queries

-- name: CreateChange :exec
INSERT INTO changes (id, service_id, change_type, severity, summary, diff, status, detected_at, affected_doc_ids)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetChangeByID :one
SELECT * FROM changes WHERE id = ?;

-- name: ListChanges :many
SELECT * FROM changes
WHERE (LENGTH(?1) = 0 OR service_id = ?1)
  AND (LENGTH(?2) = 0 OR severity = ?2)
ORDER BY detected_at DESC
LIMIT ?4 OFFSET ?3;

-- name: CountChanges :one
SELECT COUNT(*) FROM changes
WHERE (LENGTH(?1) = 0 OR service_id = ?1)
  AND (LENGTH(?2) = 0 OR severity = ?2);

-- name: UpdateChangeStatus :exec
UPDATE changes SET status = ? WHERE id = ?;
