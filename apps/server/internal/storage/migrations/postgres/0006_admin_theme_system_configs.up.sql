ALTER TABLE themes
	ADD COLUMN IF NOT EXISTS is_enabled BOOLEAN NOT NULL DEFAULT TRUE;

CREATE INDEX IF NOT EXISTS idx_themes_is_enabled ON themes(is_enabled);

CREATE TABLE IF NOT EXISTS system_configs (
	id BIGSERIAL PRIMARY KEY,
	config_key VARCHAR(128) NOT NULL UNIQUE,
	config_value_json TEXT NOT NULL,
	version INTEGER NOT NULL DEFAULT 1 CHECK (version > 0),
	updated_by_user_id VARCHAR(26) NULL REFERENCES users(user_id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_system_configs_updated_by_user_id ON system_configs(updated_by_user_id);
CREATE INDEX IF NOT EXISTS idx_system_configs_updated_at ON system_configs(updated_at);
