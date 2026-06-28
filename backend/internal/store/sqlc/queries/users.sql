-- Users queries

-- name: CreateUser :exec
INSERT INTO users (id, username, display_name, email, role, auth_source, password_hash, disabled, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetUserByID :one
SELECT * FROM users WHERE id = ?;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = ?;

-- name: ListUsers :many
SELECT * FROM users ORDER BY created_at DESC;

-- name: UpdateUser :exec
UPDATE users
SET display_name = ?,
    email = ?,
    role = ?,
    disabled = ?
WHERE id = ?;

-- name: SetUserPassword :exec
UPDATE users SET password_hash = ? WHERE id = ?;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = ?;
