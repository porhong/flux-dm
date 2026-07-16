CREATE TABLE site_profiles (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    host_pattern TEXT NOT NULL UNIQUE,
    auth_type TEXT NOT NULL DEFAULT 'none',
    proxy_url TEXT NOT NULL DEFAULT '',
    encrypted_secrets BLOB NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE download_secrets (
    download_id TEXT PRIMARY KEY,
    encrypted_secrets BLOB NOT NULL,
    created_at TEXT NOT NULL
);

ALTER TABLE downloads ADD COLUMN site_profile_id TEXT NOT NULL DEFAULT '';

