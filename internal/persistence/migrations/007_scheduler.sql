CREATE TABLE schedules (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    weekdays TEXT NOT NULL DEFAULT '0,1,2,3,4,5,6',
    time_of_day TEXT NOT NULL,
    action TEXT NOT NULL,
    queue_id TEXT NOT NULL DEFAULT '',
    speed_limit INTEGER NOT NULL DEFAULT 0,
    missed_policy TEXT NOT NULL DEFAULT 'skip',
    post_action TEXT NOT NULL DEFAULT 'none',
    created_at TEXT NOT NULL
);

CREATE TABLE schedule_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    schedule_id TEXT NOT NULL,
    run_key TEXT NOT NULL,
    scheduled_for TEXT NOT NULL,
    executed_at TEXT NOT NULL,
    status TEXT NOT NULL,
    detail TEXT NOT NULL DEFAULT '',
    UNIQUE(schedule_id, run_key),
    FOREIGN KEY(schedule_id) REFERENCES schedules(id) ON DELETE CASCADE
);

CREATE INDEX idx_schedule_history_time ON schedule_history(executed_at DESC);

