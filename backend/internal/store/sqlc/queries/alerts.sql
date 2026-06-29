-- Alerts queries

-- name: CreateAlert :exec
INSERT INTO alerts (id, change_id, service_id, severity, title, description, status, snoozed_until, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetAlertByID :one
SELECT * FROM alerts WHERE id = ?;

-- name: ListAlerts :many
SELECT * FROM alerts
WHERE (LENGTH(?1) = 0 OR severity = ?1)
  AND (LENGTH(?2) = 0 OR status = ?2)
ORDER BY created_at DESC
LIMIT ?4 OFFSET ?3;

-- name: CountAlerts :one
SELECT COUNT(*) FROM alerts
WHERE (LENGTH(?1) = 0 OR severity = ?1)
  AND (LENGTH(?2) = 0 OR status = ?2);

-- name: UpdateAlertStatus :exec
UPDATE alerts SET status = ?, snoozed_until = ? WHERE id = ?;
