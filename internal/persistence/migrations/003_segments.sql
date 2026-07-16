CREATE TABLE segments (
    id TEXT PRIMARY KEY,
    download_id TEXT NOT NULL REFERENCES downloads(id) ON DELETE CASCADE,
    segment_index INTEGER NOT NULL,
    start_byte INTEGER NOT NULL DEFAULT 0,
    end_byte INTEGER NOT NULL DEFAULT -1,
    current_byte INTEGER NOT NULL DEFAULT 0,
    state TEXT NOT NULL DEFAULT 'pending',
    retry_count INTEGER NOT NULL DEFAULT 0,
    temp_path TEXT NOT NULL,
    last_error TEXT NOT NULL DEFAULT '',
    UNIQUE(download_id, segment_index)
);

INSERT INTO segments (
    id, download_id, segment_index, start_byte, end_byte, current_byte,
    state, retry_count, temp_path, last_error
)
SELECT
    id || ':0', id, 0, 0,
    CASE WHEN total_bytes > 0 THEN total_bytes - 1 ELSE -1 END,
    downloaded_bytes,
    CASE
        WHEN state = 'completed' THEN 'completed'
        WHEN state = 'downloading' THEN 'downloading'
        WHEN state = 'failed' THEN 'failed'
        ELSE 'pending'
    END,
    retry_count, temp_path, last_error
FROM downloads;

CREATE INDEX segments_download_idx ON segments(download_id, segment_index);
