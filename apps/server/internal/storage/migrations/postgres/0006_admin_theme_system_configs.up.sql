ALTER TABLE themes
	ADD COLUMN IF NOT EXISTS is_enabled BOOLEAN NOT NULL DEFAULT TRUE;
COMMENT ON COLUMN themes.is_enabled IS '是否启用';

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

COMMENT ON TABLE system_configs IS '系统配置中心表：按 config_key 存储 JSON 配置与版本';
COMMENT ON COLUMN system_configs.id IS '主键ID';
COMMENT ON COLUMN system_configs.config_key IS '配置键（唯一）';
COMMENT ON COLUMN system_configs.config_value_json IS '配置值JSON';
COMMENT ON COLUMN system_configs.version IS '配置版本号（从 1 递增）';
COMMENT ON COLUMN system_configs.updated_by_user_id IS '最后更新人用户ID';
COMMENT ON COLUMN system_configs.created_at IS '创建时间';
COMMENT ON COLUMN system_configs.updated_at IS '更新时间';

CREATE INDEX IF NOT EXISTS idx_system_configs_updated_by_user_id ON system_configs(updated_by_user_id);
CREATE INDEX IF NOT EXISTS idx_system_configs_updated_at ON system_configs(updated_at);
