ALTER TABLE themes
	ADD COLUMN is_enabled TINYINT(1) NOT NULL DEFAULT 1;

CREATE INDEX idx_themes_is_enabled ON themes(is_enabled);

CREATE TABLE IF NOT EXISTS system_configs (
	id BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '主键ID',
	config_key VARCHAR(128) NOT NULL UNIQUE COMMENT '配置键（唯一）',
	config_value_json LONGTEXT NOT NULL COMMENT '配置值JSON',
	version INT NOT NULL DEFAULT 1 COMMENT '配置版本号（从 1 递增）',
	updated_by_user_id VARCHAR(26) NULL COMMENT '最后更新人用户ID',
	created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间',
	updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '更新时间',
	CONSTRAINT ck_system_configs_version CHECK (version > 0),
	CONSTRAINT fk_system_configs_updated_by_user_id FOREIGN KEY (updated_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL
) COMMENT='系统配置中心表：按 config_key 存储 JSON 配置与版本';

CREATE INDEX idx_system_configs_updated_by_user_id ON system_configs(updated_by_user_id);
CREATE INDEX idx_system_configs_updated_at ON system_configs(updated_at);
