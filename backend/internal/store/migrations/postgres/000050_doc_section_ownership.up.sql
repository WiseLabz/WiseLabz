-- Section ownership (#478): sync merges generated blocks instead of
-- overwriting docs. origin=human docs are never touched by sync; gen_keys
-- is NULL until a doc is first rendered with wl:gen block markers.
ALTER TABLE docs ADD COLUMN origin TEXT NOT NULL DEFAULT 'generated' CHECK(origin IN ('generated','human'));
ALTER TABLE docs ADD COLUMN template_id TEXT REFERENCES templates(id) ON DELETE SET NULL;
ALTER TABLE docs ADD COLUMN last_synced_at TEXT;
ALTER TABLE docs ADD COLUMN gen_keys TEXT;
