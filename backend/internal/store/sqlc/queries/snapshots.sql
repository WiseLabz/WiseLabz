-- Snapshots queries

-- name: CreateSnapshot :exec
INSERT INTO service_snapshots (id, connector_id, data, fetched_at)
VALUES (?, ?, ?, ?);

-- name: GetLatestSnapshot :one
SELECT * FROM service_snapshots WHERE connector_id = ? ORDER BY fetched_at DESC LIMIT 1;

-- name: DeleteSnapshotsByConnector :exec
DELETE FROM service_snapshots WHERE connector_id = ?;
