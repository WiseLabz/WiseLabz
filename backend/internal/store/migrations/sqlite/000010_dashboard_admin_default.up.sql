-- 000010_dashboard_admin_default.up.sql — admin-defined default dashboard layout (issue #94)

ALTER TABLE users ADD COLUMN can_manage_dashboard_defaults INTEGER NOT NULL DEFAULT 0 CHECK(can_manage_dashboard_defaults IN (0,1));

CREATE TABLE dashboard_admin_default (
    id      INTEGER PRIMARY KEY CHECK (id = 1),
    widgets TEXT NOT NULL DEFAULT '[]'
);

INSERT INTO dashboard_admin_default (id, widgets) VALUES (1, '[
    {"id":"roster","type":"service_status","x":0,"y":0,"w":4,"h":1},
    {"id":"alerts","type":"alert_summary","x":4,"y":0,"w":2,"h":1},
    {"id":"changes","type":"recent_changes","x":0,"y":1,"w":3,"h":1},
    {"id":"sync","type":"sync_activity","x":3,"y":1,"w":3,"h":1},
    {"id":"docs","type":"docs_health","x":0,"y":2,"w":6,"h":1}
]');
