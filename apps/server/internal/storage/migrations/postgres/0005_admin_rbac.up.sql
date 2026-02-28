CREATE TABLE IF NOT EXISTS user_admin_roles (
	id BIGSERIAL PRIMARY KEY,
	user_id VARCHAR(26) NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
	role VARCHAR(32) NOT NULL CHECK (role IN ('platform_admin', 'space_admin')),
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(user_id, role)
);

COMMENT ON TABLE user_admin_roles IS '管理员角色表：绑定平台级管理员角色';
COMMENT ON COLUMN user_admin_roles.id IS '主键ID';
COMMENT ON COLUMN user_admin_roles.user_id IS '用户业务ID';
COMMENT ON COLUMN user_admin_roles.role IS '管理员角色：platform_admin/space_admin';
COMMENT ON COLUMN user_admin_roles.created_at IS '创建时间';
COMMENT ON COLUMN user_admin_roles.updated_at IS '更新时间';

CREATE TABLE IF NOT EXISTS space_admin_scopes (
	id BIGSERIAL PRIMARY KEY,
	user_id VARCHAR(26) NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
	space_id VARCHAR(26) NOT NULL REFERENCES spaces(space_id) ON DELETE CASCADE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(user_id, space_id)
);

COMMENT ON TABLE space_admin_scopes IS '空间管理员范围表：约束 space_admin 可管理的空间';
COMMENT ON COLUMN space_admin_scopes.id IS '主键ID';
COMMENT ON COLUMN space_admin_scopes.user_id IS '管理员用户ID';
COMMENT ON COLUMN space_admin_scopes.space_id IS '可管理空间ID';
COMMENT ON COLUMN space_admin_scopes.created_at IS '创建时间';
COMMENT ON COLUMN space_admin_scopes.updated_at IS '更新时间';

ALTER TABLE users
	ADD COLUMN status VARCHAR(16) NOT NULL DEFAULT 'active',
	ADD COLUMN banned_reason TEXT NOT NULL DEFAULT '',
	ADD COLUMN banned_at TIMESTAMPTZ NULL,
	ADD COLUMN deleted_at TIMESTAMPTZ NULL;
COMMENT ON COLUMN users.status IS '用户状态：active/banned/deleted';
COMMENT ON COLUMN users.banned_reason IS '封禁原因';
COMMENT ON COLUMN users.banned_at IS '封禁时间';
COMMENT ON COLUMN users.deleted_at IS '逻辑删除时间';
ALTER TABLE users
	ADD CONSTRAINT ck_users_status CHECK (status IN ('active', 'banned', 'deleted'));

ALTER TABLE spaces
	ADD COLUMN status VARCHAR(16) NOT NULL DEFAULT 'active',
	ADD COLUMN banned_reason TEXT NOT NULL DEFAULT '',
	ADD COLUMN banned_at TIMESTAMPTZ NULL,
	ADD COLUMN deleted_at TIMESTAMPTZ NULL;
COMMENT ON COLUMN spaces.status IS '空间状态：active/banned/deleted';
COMMENT ON COLUMN spaces.banned_reason IS '空间封禁原因';
COMMENT ON COLUMN spaces.banned_at IS '空间封禁时间';
COMMENT ON COLUMN spaces.deleted_at IS '空间逻辑删除时间';
ALTER TABLE spaces
	ADD CONSTRAINT ck_spaces_status CHECK (status IN ('active', 'banned', 'deleted'));

ALTER TABLE documents
	ADD COLUMN status VARCHAR(16) NOT NULL DEFAULT 'active',
	ADD COLUMN banned_reason TEXT NOT NULL DEFAULT '',
	ADD COLUMN banned_at TIMESTAMPTZ NULL,
	ADD COLUMN deleted_at TIMESTAMPTZ NULL;
COMMENT ON COLUMN documents.status IS '文档状态：active/banned/deleted';
COMMENT ON COLUMN documents.banned_reason IS '文档封禁原因';
COMMENT ON COLUMN documents.banned_at IS '文档封禁时间';
COMMENT ON COLUMN documents.deleted_at IS '文档逻辑删除时间';
ALTER TABLE documents
	ADD CONSTRAINT ck_documents_status CHECK (status IN ('active', 'banned', 'deleted'));

CREATE INDEX IF NOT EXISTS idx_user_admin_roles_user_id ON user_admin_roles(user_id);
CREATE INDEX IF NOT EXISTS idx_user_admin_roles_role ON user_admin_roles(role);
CREATE INDEX IF NOT EXISTS idx_space_admin_scopes_user_id ON space_admin_scopes(user_id);
CREATE INDEX IF NOT EXISTS idx_space_admin_scopes_space_id ON space_admin_scopes(space_id);

CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);
CREATE INDEX IF NOT EXISTS idx_spaces_status ON spaces(status);
CREATE INDEX IF NOT EXISTS idx_documents_status ON documents(status);
