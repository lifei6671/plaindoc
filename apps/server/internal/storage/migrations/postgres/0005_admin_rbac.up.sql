CREATE TABLE IF NOT EXISTS user_admin_roles (
	id BIGSERIAL PRIMARY KEY,
	user_id VARCHAR(26) NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
	role VARCHAR(32) NOT NULL CHECK (role IN ('platform_admin', 'space_admin')),
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(user_id, role)
);

CREATE TABLE IF NOT EXISTS space_admin_scopes (
	id BIGSERIAL PRIMARY KEY,
	user_id VARCHAR(26) NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
	space_id VARCHAR(26) NOT NULL REFERENCES spaces(space_id) ON DELETE CASCADE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(user_id, space_id)
);

ALTER TABLE users
	ADD COLUMN status VARCHAR(16) NOT NULL DEFAULT 'active',
	ADD COLUMN banned_reason TEXT NOT NULL DEFAULT '',
	ADD COLUMN banned_at TIMESTAMPTZ NULL,
	ADD COLUMN deleted_at TIMESTAMPTZ NULL;
ALTER TABLE users
	ADD CONSTRAINT ck_users_status CHECK (status IN ('active', 'banned', 'deleted'));

ALTER TABLE spaces
	ADD COLUMN status VARCHAR(16) NOT NULL DEFAULT 'active',
	ADD COLUMN banned_reason TEXT NOT NULL DEFAULT '',
	ADD COLUMN banned_at TIMESTAMPTZ NULL,
	ADD COLUMN deleted_at TIMESTAMPTZ NULL;
ALTER TABLE spaces
	ADD CONSTRAINT ck_spaces_status CHECK (status IN ('active', 'banned', 'deleted'));

ALTER TABLE documents
	ADD COLUMN status VARCHAR(16) NOT NULL DEFAULT 'active',
	ADD COLUMN banned_reason TEXT NOT NULL DEFAULT '',
	ADD COLUMN banned_at TIMESTAMPTZ NULL,
	ADD COLUMN deleted_at TIMESTAMPTZ NULL;
ALTER TABLE documents
	ADD CONSTRAINT ck_documents_status CHECK (status IN ('active', 'banned', 'deleted'));

CREATE INDEX IF NOT EXISTS idx_user_admin_roles_user_id ON user_admin_roles(user_id);
CREATE INDEX IF NOT EXISTS idx_user_admin_roles_role ON user_admin_roles(role);
CREATE INDEX IF NOT EXISTS idx_space_admin_scopes_user_id ON space_admin_scopes(user_id);
CREATE INDEX IF NOT EXISTS idx_space_admin_scopes_space_id ON space_admin_scopes(space_id);

CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);
CREATE INDEX IF NOT EXISTS idx_spaces_status ON spaces(status);
CREATE INDEX IF NOT EXISTS idx_documents_status ON documents(status);
