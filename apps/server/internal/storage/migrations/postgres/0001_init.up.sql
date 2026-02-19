CREATE TABLE IF NOT EXISTS users (
	id BIGSERIAL PRIMARY KEY,
	ulid VARCHAR(26) NOT NULL UNIQUE,
	email VARCHAR(320) NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	name VARCHAR(128) NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS spaces (
	id BIGSERIAL PRIMARY KEY,
	ulid VARCHAR(26) NOT NULL UNIQUE,
	name VARCHAR(255) NOT NULL,
	owner_ulid VARCHAR(26) NOT NULL REFERENCES users(ulid) ON DELETE CASCADE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS space_members (
	id BIGSERIAL PRIMARY KEY,
	space_ulid VARCHAR(26) NOT NULL REFERENCES spaces(ulid) ON DELETE CASCADE,
	user_ulid VARCHAR(26) NOT NULL REFERENCES users(ulid) ON DELETE CASCADE,
	role VARCHAR(16) NOT NULL CHECK (role IN ('owner', 'collaborator', 'reader')),
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(space_ulid, user_ulid)
);

CREATE TABLE IF NOT EXISTS nodes (
	id BIGSERIAL PRIMARY KEY,
	ulid VARCHAR(26) NOT NULL UNIQUE,
	space_ulid VARCHAR(26) NOT NULL REFERENCES spaces(ulid) ON DELETE CASCADE,
	parent_ulid VARCHAR(26) NULL REFERENCES nodes(ulid) ON DELETE CASCADE,
	type VARCHAR(16) NOT NULL CHECK (type IN ('folder', 'doc')),
	title VARCHAR(255) NOT NULL,
	sort INTEGER NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS documents (
	id BIGSERIAL PRIMARY KEY,
	ulid VARCHAR(26) NOT NULL UNIQUE,
	node_ulid VARCHAR(26) NOT NULL UNIQUE REFERENCES nodes(ulid) ON DELETE CASCADE,
	title VARCHAR(255) NOT NULL,
	content_md TEXT NOT NULL DEFAULT '',
	version INTEGER NOT NULL DEFAULT 1,
	updated_by_ulid VARCHAR(26) NULL REFERENCES users(ulid) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS document_revisions (
	id BIGSERIAL PRIMARY KEY,
	ulid VARCHAR(26) NOT NULL UNIQUE,
	document_ulid VARCHAR(26) NOT NULL REFERENCES documents(ulid) ON DELETE CASCADE,
	version INTEGER NOT NULL,
	content_md TEXT NOT NULL,
	base_version INTEGER NOT NULL,
	editor_ulid VARCHAR(26) NULL REFERENCES users(ulid) ON DELETE SET NULL,
	source VARCHAR(16) NOT NULL CHECK (source IN ('local', 'remote')),
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(document_ulid, version)
);

CREATE TABLE IF NOT EXISTS node_permissions (
	id BIGSERIAL PRIMARY KEY,
	node_ulid VARCHAR(26) NOT NULL REFERENCES nodes(ulid) ON DELETE CASCADE,
	user_ulid VARCHAR(26) NOT NULL REFERENCES users(ulid) ON DELETE CASCADE,
	role VARCHAR(16) NOT NULL CHECK (role IN ('owner', 'collaborator', 'reader')),
	granted_by_ulid VARCHAR(26) NOT NULL REFERENCES users(ulid) ON DELETE CASCADE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(node_ulid, user_ulid)
);

CREATE TABLE IF NOT EXISTS document_permissions (
	id BIGSERIAL PRIMARY KEY,
	document_ulid VARCHAR(26) NOT NULL REFERENCES documents(ulid) ON DELETE CASCADE,
	user_ulid VARCHAR(26) NOT NULL REFERENCES users(ulid) ON DELETE CASCADE,
	role VARCHAR(16) NOT NULL CHECK (role IN ('owner', 'collaborator', 'reader')),
	granted_by_ulid VARCHAR(26) NOT NULL REFERENCES users(ulid) ON DELETE CASCADE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(document_ulid, user_ulid)
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
