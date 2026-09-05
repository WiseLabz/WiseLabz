-- 000003_saved_views.up.sql — saved operational views/filters (issue #26)

CREATE TABLE saved_views (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL,
    surface     TEXT NOT NULL,
    name        TEXT NOT NULL,
    filters     TEXT NOT NULL DEFAULT '{}',
    created_at  TEXT NOT NULL
);
CREATE INDEX idx_saved_views_user_surface ON saved_views(user_id, surface);
