-- Connectors queries

-- name: CreateConnector :exec
INSERT INTO connectors (id, name, category, type, url, verify_tls, config_data, enabled, status, status_message, last_sync_at, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetConnectorByID :one
SELECT * FROM connectors WHERE id = ?;

-- name: ListConnectors :many
SELECT * FROM connectors ORDER BY created_at DESC;

-- name: UpdateConnector :exec
UPDATE connectors
SET name = ?,
    url = ?,
    verify_tls = ?,
    config_data = ?,
    updated_at = ?
WHERE id = ?;

-- name: DeleteConnector :exec
DELETE FROM connectors WHERE id = ?;

-- name: SetConnectorEnabled :exec
UPDATE connectors SET enabled = ?, updated_at = ? WHERE id = ?;

-- name: SetConnectorStatus :exec
UPDATE connectors SET status = ?, status_message = ?, last_sync_at = ? WHERE id = ?;
