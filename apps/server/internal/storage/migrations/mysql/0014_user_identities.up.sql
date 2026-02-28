CREATE TABLE IF NOT EXISTS user_identities (
	id BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '主键ID',
	user_id VARCHAR(26) NOT NULL COMMENT '用户业务ID',
	provider_type VARCHAR(32) NOT NULL COMMENT '身份提供方类型，如 local/ldap',
	provider_id VARCHAR(64) NOT NULL COMMENT '身份提供方实例ID',
	external_id VARCHAR(255) NOT NULL COMMENT '外部身份唯一标识',
	login_name VARCHAR(320) NOT NULL DEFAULT '' COMMENT '用于登录展示或匹配的名称',
	last_login_at DATETIME NULL COMMENT '最近一次登录时间',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
	UNIQUE KEY uk_user_identities_provider_external (provider_id, external_id),
	KEY idx_user_identities_user_id (user_id),
	KEY idx_user_identities_provider_type (provider_type),
	CONSTRAINT fk_user_identities_user_id FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户身份映射表：关联本地账号与外部身份提供方';

