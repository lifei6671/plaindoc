PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS users (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id TEXT NOT NULL UNIQUE,
	email TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	name TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS spaces (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	space_id TEXT NOT NULL UNIQUE,
	name TEXT NOT NULL,
	owner_user_id TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (owner_user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS space_members (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	space_id TEXT NOT NULL,
	user_id TEXT NOT NULL,
	role TEXT NOT NULL CHECK (role IN ('owner', 'collaborator', 'reader')),
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(space_id, user_id),
	FOREIGN KEY (space_id) REFERENCES spaces(space_id) ON DELETE CASCADE,
	FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS nodes (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	node_id TEXT NOT NULL UNIQUE,
	space_id TEXT NOT NULL,
	parent_node_id TEXT NULL,
	type TEXT NOT NULL CHECK (type IN ('folder', 'doc')),
	title TEXT NOT NULL,
	sort INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (space_id) REFERENCES spaces(space_id) ON DELETE CASCADE,
	FOREIGN KEY (parent_node_id) REFERENCES nodes(node_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS documents (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	document_id TEXT NOT NULL UNIQUE,
	node_id TEXT NOT NULL UNIQUE,
	title TEXT NOT NULL,
	content_md TEXT NOT NULL DEFAULT '',
	version INTEGER NOT NULL DEFAULT 1,
	updated_by_user_id TEXT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (node_id) REFERENCES nodes(node_id) ON DELETE CASCADE,
	FOREIGN KEY (updated_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS document_revisions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	document_revision_id TEXT NOT NULL UNIQUE,
	document_id TEXT NOT NULL,
	version INTEGER NOT NULL,
	content_md TEXT NOT NULL,
	base_version INTEGER NOT NULL,
	editor_user_id TEXT NULL,
	source TEXT NOT NULL CHECK (source IN ('local', 'remote')),
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(document_id, version),
	FOREIGN KEY (document_id) REFERENCES documents(document_id) ON DELETE CASCADE,
	FOREIGN KEY (editor_user_id) REFERENCES users(user_id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS node_permissions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	node_id TEXT NOT NULL,
	user_id TEXT NOT NULL,
	role TEXT NOT NULL CHECK (role IN ('owner', 'collaborator', 'reader')),
	granted_by_user_id TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(node_id, user_id),
	FOREIGN KEY (node_id) REFERENCES nodes(node_id) ON DELETE CASCADE,
	FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
	FOREIGN KEY (granted_by_user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS document_permissions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	document_id TEXT NOT NULL,
	user_id TEXT NOT NULL,
	role TEXT NOT NULL CHECK (role IN ('owner', 'collaborator', 'reader')),
	granted_by_user_id TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(document_id, user_id),
	FOREIGN KEY (document_id) REFERENCES documents(document_id) ON DELETE CASCADE,
	FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
	FOREIGN KEY (granted_by_user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_spaces_owner_user_id ON spaces(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_spaces_updated_at ON spaces(updated_at);

CREATE INDEX IF NOT EXISTS idx_space_members_space_id ON space_members(space_id);
CREATE INDEX IF NOT EXISTS idx_space_members_user_id ON space_members(user_id);

CREATE INDEX IF NOT EXISTS idx_nodes_space_id ON nodes(space_id);
CREATE INDEX IF NOT EXISTS idx_nodes_parent_node_id ON nodes(parent_node_id);
CREATE INDEX IF NOT EXISTS idx_nodes_space_parent_sort ON nodes(space_id, parent_node_id, sort);
CREATE INDEX IF NOT EXISTS idx_nodes_type ON nodes(type);

CREATE INDEX IF NOT EXISTS idx_documents_node_id ON documents(node_id);
CREATE INDEX IF NOT EXISTS idx_documents_updated_at ON documents(updated_at);

CREATE INDEX IF NOT EXISTS idx_revisions_document_id ON document_revisions(document_id);
CREATE INDEX IF NOT EXISTS idx_revisions_document_version ON document_revisions(document_id, version);
CREATE INDEX IF NOT EXISTS idx_revisions_document_created_at ON document_revisions(document_id, created_at);

CREATE INDEX IF NOT EXISTS idx_node_permissions_node_id ON node_permissions(node_id);
CREATE INDEX IF NOT EXISTS idx_node_permissions_user_id ON node_permissions(user_id);

CREATE INDEX IF NOT EXISTS idx_document_permissions_document_id ON document_permissions(document_id);
CREATE INDEX IF NOT EXISTS idx_document_permissions_user_id ON document_permissions(user_id);
