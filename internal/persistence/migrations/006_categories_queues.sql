CREATE TABLE categories (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    extensions TEXT NOT NULL DEFAULT '',
    destination_dir TEXT NOT NULL DEFAULT '',
    priority INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL
);

CREATE TABLE download_queues (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    priority INTEGER NOT NULL DEFAULT 0,
    max_parallel INTEGER NOT NULL DEFAULT 3 CHECK (max_parallel BETWEEN 1 AND 16),
    max_connections INTEGER NOT NULL DEFAULT 4 CHECK (max_connections IN (1, 2, 4, 8, 16)),
    bandwidth_limit INTEGER NOT NULL DEFAULT 0 CHECK (bandwidth_limit >= 0),
    sequential INTEGER NOT NULL DEFAULT 0,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL
);

ALTER TABLE downloads ADD COLUMN category_id TEXT NOT NULL DEFAULT '';
ALTER TABLE downloads ADD COLUMN queue_id TEXT NOT NULL DEFAULT '';
ALTER TABLE downloads ADD COLUMN queue_position INTEGER NOT NULL DEFAULT 0;
ALTER TABLE downloads ADD COLUMN priority INTEGER NOT NULL DEFAULT 0;

CREATE INDEX idx_downloads_queue_order ON downloads(queue_id, priority DESC, queue_position ASC, created_at ASC);

