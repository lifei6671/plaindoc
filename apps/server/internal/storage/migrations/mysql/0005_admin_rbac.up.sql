CREATE TABLE IF NOT EXISTS user_admin_roles (
	id BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '主键ID',
	user_id VARCHAR(26) NOT NULL COMMENT '用户业务ID',
	role VARCHAR(32) NOT NULL COMMENT '管理员角色：platform_admin/space_admin',
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
	UNIQUE KEY uk_user_admin_roles_user_role (user_id, role),
	KEY idx_user_admin_roles_user_id (user_id),
	KEY idx_user_admin_roles_role (role),
	CONSTRAINT ck_user_admin_roles_role CHECK (role IN ('platform_admin', 'space_admin')),
	CONSTRAINT fk_user_admin_roles_user_id FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='管理员角色表：绑定平台级管理员角色';

CREATE TABLE IF NOT EXISTS space_admin_scopes (
	id BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '主键ID',
	user_id VARCHAR(26) NOT NULL COMMENT '管理员用户ID',
	space_id VARCHAR(26) NOT NULL COMMENT '可管理空间ID',
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
	UNIQUE KEY uk_space_admin_scopes_user_space (user_id, space_id),
	KEY idx_space_admin_scopes_user_id (user_id),
	KEY idx_space_admin_scopes_space_id (space_id),
	CONSTRAINT fk_space_admin_scopes_user_id FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
	CONSTRAINT fk_space_admin_scopes_space_id FOREIGN KEY (space_id) REFERENCES spaces(space_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='空间管理员范围表：约束 space_admin 可管理的空间';

ALTER TABLE users
	ADD COLUMN status VARCHAR(16) NOT NULL DEFAULT 'active',
	ADD COLUMN banned_reason VARCHAR(255) NOT NULL DEFAULT '',
	ADD COLUMN banned_at DATETIME(3) NULL,
	ADD COLUMN deleted_at DATETIME(3) NULL,
	ADD CONSTRAINT ck_users_status CHECK (status IN ('active', 'banned', 'deleted'));

ALTER TABLE spaces
	ADD COLUMN status VARCHAR(16) NOT NULL DEFAULT 'active',
	ADD COLUMN banned_reason VARCHAR(255) NOT NULL DEFAULT '',
	ADD COLUMN banned_at DATETIME(3) NULL,
	ADD COLUMN deleted_at DATETIME(3) NULL,
	ADD CONSTRAINT ck_spaces_status CHECK (status IN ('active', 'banned', 'deleted'));

ALTER TABLE documents
	ADD COLUMN status VARCHAR(16) NOT NULL DEFAULT 'active',
	ADD COLUMN banned_reason VARCHAR(255) NOT NULL DEFAULT '',
	ADD COLUMN banned_at DATETIME(3) NULL,
	ADD COLUMN deleted_at DATETIME(3) NULL,
	ADD CONSTRAINT ck_documents_status CHECK (status IN ('active', 'banned', 'deleted'));

CREATE INDEX idx_users_status ON users(status);
CREATE INDEX idx_spaces_status ON spaces(status);
CREATE INDEX idx_documents_status ON documents(status);
