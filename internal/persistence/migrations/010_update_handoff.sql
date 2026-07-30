ALTER TABLE update_state ADD COLUMN installation_token TEXT NOT NULL DEFAULT '';
ALTER TABLE update_state ADD COLUMN installed_version TEXT NOT NULL DEFAULT '';
ALTER TABLE update_state ADD COLUMN installed_at TEXT NOT NULL DEFAULT '';
