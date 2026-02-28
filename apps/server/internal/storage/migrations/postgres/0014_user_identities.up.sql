CREATE TABLE IF NOT EXISTS user_identities (
	id BIGSERIAL PRIMARY KEY,
	user_id VARCHAR(26) NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
	provider_type VARCHAR(32) NOT NULL,
	provider_id VARCHAR(64) NOT NULL,
	external_id VARCHAR(255) NOT NULL,
	login_name VARCHAR(320) NOT NULL DEFAULT '',
	last_login_at TIMESTAMP NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(provider_id, external_id)
);

COMMENT ON TABLE user_identities IS '用户身份映射表：关联本地账号与外部身份提供方';
COMMENT ON COLUMN user_identities.id IS '主键ID';
COMMENT ON COLUMN user_identities.user_id IS '用户业务ID';
COMMENT ON COLUMN user_identities.provider_type IS '身份提供方类型，如 local/ldap';
COMMENT ON COLUMN user_identities.provider_id IS '身份提供方实例ID';
COMMENT ON COLUMN user_identities.external_id IS '外部身份唯一标识';
COMMENT ON COLUMN user_identities.login_name IS '用于登录展示或匹配的名称';
COMMENT ON COLUMN user_identities.last_login_at IS '最近一次登录时间';
COMMENT ON COLUMN user_identities.created_at IS '创建时间';
COMMENT ON COLUMN user_identities.updated_at IS '更新时间';

CREATE INDEX IF NOT EXISTS idx_user_identities_user_id ON user_identities(user_id);
CREATE INDEX IF NOT EXISTS idx_user_identities_provider_type ON user_identities(provider_type);

