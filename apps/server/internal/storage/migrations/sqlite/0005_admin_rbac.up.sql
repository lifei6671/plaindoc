CREATE TABLE IF NOT EXISTS user_admin_roles (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id TEXT NOT NULL,
	role TEXT NOT NULL CHECK (role IN ('platform_admin', 'space_admin')),
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(user_id, role),
	FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS space_admin_scopes (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id TEXT NOT NULL,
	space_id TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(user_id, space_id),
	FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
	FOREIGN KEY (space_id) REFERENCES spaces(space_id) ON DELETE CASCADE
);

ALTER TABLE users
	ADD COLUMN status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'banned', 'deleted'));
ALTER TABLE users
	ADD COLUMN banned_reason TEXT NOT NULL DEFAULT '';
ALTER TABLE users
	ADD COLUMN banned_at TIMESTAMP NULL;
ALTER TABLE users
	ADD COLUMN deleted_at TIMESTAMP NULL;

ALTER TABLE spaces
	ADD COLUMN status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'banned', 'deleted'));
ALTER TABLE spaces
	ADD COLUMN banned_reason TEXT NOT NULL DEFAULT '';
ALTER TABLE spaces
	ADD COLUMN banned_at TIMESTAMP NULL;
ALTER TABLE spaces
	ADD COLUMN deleted_at TIMESTAMP NULL;

ALTER TABLE documents
	ADD COLUMN status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'banned', 'deleted'));
ALTER TABLE documents
	ADD COLUMN banned_reason TEXT NOT NULL DEFAULT '';
ALTER TABLE documents
	ADD COLUMN banned_at TIMESTAMP NULL;
ALTER TABLE documents
	ADD COLUMN deleted_at TIMESTAMP NULL;

CREATE INDEX IF NOT EXISTS idx_user_admin_roles_user_id ON user_admin_roles(user_id);
CREATE INDEX IF NOT EXISTS idx_user_admin_roles_role ON user_admin_roles(role);
CREATE INDEX IF NOT EXISTS idx_space_admin_scopes_user_id ON space_admin_scopes(user_id);
CREATE INDEX IF NOT EXISTS idx_space_admin_scopes_space_id ON space_admin_scopes(space_id);

CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);
CREATE INDEX IF NOT EXISTS idx_spaces_status ON spaces(status);
CREATE INDEX IF NOT EXISTS idx_documents_status ON documents(status);
