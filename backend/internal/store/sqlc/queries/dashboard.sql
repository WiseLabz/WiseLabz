-- Dashboard queries

-- name: GetDashboardLayout :one
SELECT * FROM dashboard_layouts WHERE user_id = ?;

-- name: UpsertDashboardLayout :exec
INSERT INTO dashboard_layouts (user_id, widgets) VALUES (?, ?)
ON CONFLICT(user_id) DO UPDATE SET widgets = excluded.widgets;
