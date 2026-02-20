ALTER TABLE themes
	ADD COLUMN is_enabled INTEGER NOT NULL DEFAULT 1 CHECK (is_enabled IN (0, 1));

CREATE INDEX IF NOT EXISTS idx_themes_is_enabled ON themes(is_enabled);

CREATE TABLE IF NOT EXISTS system_configs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	config_key TEXT NOT NULL UNIQUE,
	config_value_json TEXT NOT NULL,
	version INTEGER NOT NULL DEFAULT 1 CHECK (version > 0),
	updated_by_user_id TEXT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (updated_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_system_configs_updated_by_user_id ON system_configs(updated_by_user_id);
CREATE INDEX IF NOT EXISTS idx_system_configs_updated_at ON system_configs(updated_at);
