CREATE TABLE downloads (
    id TEXT PRIMARY KEY,
    url TEXT NOT NULL,
    final_url TEXT NOT NULL DEFAULT '',
    file_name TEXT NOT NULL,
    destination_path TEXT NOT NULL,
    temp_path TEXT NOT NULL,
    state TEXT NOT NULL,
    total_bytes INTEGER NOT NULL DEFAULT -1,
    downloaded_bytes INTEGER NOT NULL DEFAULT 0,
    range_supported INTEGER NOT NULL DEFAULT 0,
    etag TEXT NOT NULL DEFAULT '',
    last_modified TEXT NOT NULL DEFAULT '',
    mime_type TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    started_at TEXT,
    completed_at TEXT,
    last_error TEXT NOT NULL DEFAULT '',
    retry_count INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX downloads_state_created_idx ON downloads(state, created_at DESC);
