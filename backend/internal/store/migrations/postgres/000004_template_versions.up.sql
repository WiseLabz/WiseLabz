ALTER TABLE templates ADD COLUMN current_version INTEGER NOT NULL DEFAULT 1;

CREATE TABLE template_versions (
    id              TEXT PRIMARY KEY,
    template_id     TEXT NOT NULL REFERENCES templates(id) ON DELETE CASCADE,
    rev             INTEGER NOT NULL,
    name            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    applies_to      TEXT,
    sections        TEXT NOT NULL,
    author          TEXT,
    trigger         TEXT NOT NULL CHECK(trigger IN ('save','restore')),
    created_at      TEXT NOT NULL
);
CREATE UNIQUE INDEX idx_template_versions_tmpl_rev ON template_versions(template_id, rev DESC);
