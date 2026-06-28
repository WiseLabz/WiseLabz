-- Sessions queries

-- name: CreateSession :exec
INSERT INTO sessions (id, user_id, token_hash, user_agent, ip, created_at, last_seen_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetSessionByTokenHash :one
SELECT * FROM sessions WHERE token_hash = ?;

-- name: ListSessionsByUser :many
SELECT * FROM sessions WHERE user_id = ? ORDER BY last_seen_at DESC;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = ?;

-- name: TouchSession :exec
UPDATE sessions SET last_seen_at = ? WHERE id = ?;
