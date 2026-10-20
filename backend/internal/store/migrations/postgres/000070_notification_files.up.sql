CREATE TABLE notification_files (
    notification_id TEXT PRIMARY KEY REFERENCES in_app_notifications(id) ON DELETE CASCADE,
    filename TEXT NOT NULL,
    content_type TEXT NOT NULL,
    sha256 TEXT NOT NULL CHECK (length(sha256) = 64),
    size BIGINT NOT NULL CHECK (size >= 0),
    created_at TEXT NOT NULL
);
CREATE INDEX idx_notification_files_sha ON notification_files(sha256);
