CREATE TABLE update_state (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    channel TEXT NOT NULL DEFAULT 'stable',
    auto_download INTEGER NOT NULL DEFAULT 1,
    phase TEXT NOT NULL DEFAULT 'idle',
    available_version TEXT NOT NULL DEFAULT '',
    release_notes_url TEXT NOT NULL DEFAULT '',
    installer_path TEXT NOT NULL DEFAULT '',
    installer_sha256 TEXT NOT NULL DEFAULT '',
    downloaded_bytes INTEGER NOT NULL DEFAULT 0,
    total_bytes INTEGER NOT NULL DEFAULT 0,
    last_checked_at TEXT NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO update_state(id) VALUES (1);
