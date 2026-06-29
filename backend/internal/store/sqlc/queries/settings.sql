-- Settings queries

-- name: GetAuthConfig :one
SELECT * FROM auth_config WHERE id = 1;

-- name: UpsertAuthConfig :exec
INSERT INTO auth_config (id, local_enabled, access_token_ttl, refresh_token_ttl, step_up_for_destructive)
VALUES (1, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    local_enabled = excluded.local_enabled,
    access_token_ttl = excluded.access_token_ttl,
    refresh_token_ttl = excluded.refresh_token_ttl,
    step_up_for_destructive = excluded.step_up_for_destructive;

-- name: GetOidcProviderFlag :one
SELECT * FROM oidc_provider_flags WHERE provider_id = ?;

-- name: UpsertOidcProviderFlag :exec
INSERT INTO oidc_provider_flags (provider_id, enabled) VALUES (?, ?)
ON CONFLICT(provider_id) DO UPDATE SET enabled = excluded.enabled;

-- name: GetAiConfig :one
SELECT * FROM ai_config WHERE id = 1;

-- name: UpsertAiConfig :exec
INSERT INTO ai_config (id, enabled, provider, model, api_key_encrypted, base_url, mode)
VALUES (1, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    enabled = excluded.enabled,
    provider = excluded.provider,
    model = excluded.model,
    api_key_encrypted = excluded.api_key_encrypted,
    base_url = excluded.base_url,
    mode = excluded.mode;

-- name: GetNotificationConfig :one
SELECT * FROM notification_config WHERE id = 1;

-- name: UpsertNotificationConfig :exec
INSERT INTO notification_config (id, config_json) VALUES (1, ?)
ON CONFLICT(id) DO UPDATE SET config_json = excluded.config_json;

-- name: CreateInAppNotification :exec
INSERT INTO in_app_notifications (id, user_id, alert_id, event_type, title, message, read, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListInAppNotifications :many
SELECT * FROM in_app_notifications WHERE user_id = ? ORDER BY created_at DESC
LIMIT ?2 OFFSET ?1;

-- name: MarkNotificationRead :exec
UPDATE in_app_notifications SET read = 1 WHERE id = ?;
