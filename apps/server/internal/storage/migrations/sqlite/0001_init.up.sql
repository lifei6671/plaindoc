PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS users (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	ulid TEXT NOT NULL UNIQUE,
	email TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	name TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS spaces (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	ulid TEXT NOT NULL UNIQUE,
	name TEXT NOT NULL,
	owner_ulid TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (owner_ulid) REFERENCES users(ulid) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS space_members (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	space_ulid TEXT NOT NULL,
	user_ulid TEXT NOT NULL,
	role TEXT NOT NULL CHECK (role IN ('owner', 'collaborator', 'reader')),
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(space_ulid, user_ulid),
	FOREIGN KEY (space_ulid) REFERENCES spaces(ulid) ON DELETE CASCADE,
	FOREIGN KEY (user_ulid) REFERENCES users(ulid) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS nodes (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	ulid TEXT NOT NULL UNIQUE,
	space_ulid TEXT NOT NULL,
	parent_ulid TEXT NULL,
	type TEXT NOT NULL CHECK (type IN ('folder', 'doc')),
	title TEXT NOT NULL,
	sort INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (space_ulid) REFERENCES spaces(ulid) ON DELETE CASCADE,
	FOREIGN KEY (parent_ulid) REFERENCES nodes(ulid) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS documents (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	ulid TEXT NOT NULL UNIQUE,
	node_ulid TEXT NOT NULL UNIQUE,
	title TEXT NOT NULL,
	content_md TEXT NOT NULL DEFAULT '',
	version INTEGER NOT NULL DEFAULT 1,
	updated_by_ulid TEXT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (node_ulid) REFERENCES nodes(ulid) ON DELETE CASCADE,
	FOREIGN KEY (updated_by_ulid) REFERENCES users(ulid) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS document_revisions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	ulid TEXT NOT NULL UNIQUE,
	document_ulid TEXT NOT NULL,
	version INTEGER NOT NULL,
	content_md TEXT NOT NULL,
	base_version INTEGER NOT NULL,
	editor_ulid TEXT NULL,
	source TEXT NOT NULL CHECK (source IN ('local', 'remote')),
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(document_ulid, version),
	FOREIGN KEY (document_ulid) REFERENCES documents(ulid) ON DELETE CASCADE,
	FOREIGN KEY (editor_ulid) REFERENCES users(ulid) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS node_permissions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	node_ulid TEXT NOT NULL,
	user_ulid TEXT NOT NULL,
	role TEXT NOT NULL CHECK (role IN ('owner', 'collaborator', 'reader')),
	granted_by_ulid TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(node_ulid, user_ulid),
	FOREIGN KEY (node_ulid) REFERENCES nodes(ulid) ON DELETE CASCADE,
	FOREIGN KEY (user_ulid) REFERENCES users(ulid) ON DELETE CASCADE,
	FOREIGN KEY (granted_by_ulid) REFERENCES users(ulid) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS document_permissions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	document_ulid TEXT NOT NULL,
	user_ulid TEXT NOT NULL,
	role TEXT NOT NULL CHECK (role IN ('owner', 'collaborator', 'reader')),
	granted_by_ulid TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(document_ulid, user_ulid),
	FOREIGN KEY (document_ulid) REFERENCES documents(ulid) ON DELETE CASCADE,
	FOREIGN KEY (user_ulid) REFERENCES users(ulid) ON DELETE CASCADE,
	FOREIGN KEY (granted_by_ulid) REFERENCES users(ulid) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_spaces_owner_ulid ON spaces(owner_ulid);
CREATE INDEX IF NOT EXISTS idx_spaces_updated_at ON spaces(updated_at);

CREATE INDEX IF NOT EXISTS idx_space_members_space_ulid ON space_members(space_ulid);
CREATE INDEX IF NOT EXISTS idx_space_members_user_ulid ON space_members(user_ulid);

CREATE INDEX IF NOT EXISTS idx_nodes_space_ulid ON nodes(space_ulid);
CREATE INDEX IF NOT EXISTS idx_nodes_parent_ulid ON nodes(parent_ulid);
CREATE INDEX IF NOT EXISTS idx_nodes_space_parent_sort ON nodes(space_ulid, parent_ulid, sort);
CREATE INDEX IF NOT EXISTS idx_nodes_type ON nodes(type);

CREATE INDEX IF NOT EXISTS idx_documents_node_ulid ON documents(node_ulid);
CREATE INDEX IF NOT EXISTS idx_documents_updated_at ON documents(updated_at);

CREATE INDEX IF NOT EXISTS idx_revisions_document_ulid ON document_revisions(document_ulid);
CREATE INDEX IF NOT EXISTS idx_revisions_document_version ON document_revisions(document_ulid, version);
CREATE INDEX IF NOT EXISTS idx_revisions_document_created_at ON document_revisions(document_ulid, created_at);

CREATE INDEX IF NOT EXISTS idx_node_permissions_node_ulid ON node_permissions(node_ulid);
CREATE INDEX IF NOT EXISTS idx_node_permissions_user_ulid ON node_permissions(user_ulid);

CREATE INDEX IF NOT EXISTS idx_document_permissions_document_ulid ON document_permissions(document_ulid);
CREATE INDEX IF NOT EXISTS idx_document_permissions_user_ulid ON document_permissions(user_ulid);
