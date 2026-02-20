ALTER TABLE themes
	ADD COLUMN is_enabled TINYINT(1) NOT NULL DEFAULT 1;

CREATE INDEX idx_themes_is_enabled ON themes(is_enabled);

CREATE TABLE IF NOT EXISTS system_configs (
	id BIGINT PRIMARY KEY AUTO_INCREMENT,
	config_key VARCHAR(128) NOT NULL UNIQUE,
	config_value_json LONGTEXT NOT NULL,
	version INT NOT NULL DEFAULT 1,
	updated_by_user_id VARCHAR(26) NULL,
	created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
	updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
	CONSTRAINT ck_system_configs_version CHECK (version > 0),
	CONSTRAINT fk_system_configs_updated_by_user_id FOREIGN KEY (updated_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL
);

CREATE INDEX idx_system_configs_updated_by_user_id ON system_configs(updated_by_user_id);
CREATE INDEX idx_system_configs_updated_at ON system_configs(updated_at);
