CREATE TABLE IF NOT EXISTS users (
	id BIGSERIAL PRIMARY KEY,
	user_id VARCHAR(26) NOT NULL UNIQUE,
	email VARCHAR(320) NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	name VARCHAR(128) NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS spaces (
	id BIGSERIAL PRIMARY KEY,
	space_id VARCHAR(26) NOT NULL UNIQUE,
	name VARCHAR(255) NOT NULL,
	owner_user_id VARCHAR(26) NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS space_members (
	id BIGSERIAL PRIMARY KEY,
	space_id VARCHAR(26) NOT NULL REFERENCES spaces(space_id) ON DELETE CASCADE,
	user_id VARCHAR(26) NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
	role VARCHAR(16) NOT NULL CHECK (role IN ('owner', 'collaborator', 'reader')),
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(space_id, user_id)
);

CREATE TABLE IF NOT EXISTS nodes (
	id BIGSERIAL PRIMARY KEY,
	node_id VARCHAR(26) NOT NULL UNIQUE,
	space_id VARCHAR(26) NOT NULL REFERENCES spaces(space_id) ON DELETE CASCADE,
	parent_node_id VARCHAR(26) NULL REFERENCES nodes(node_id) ON DELETE CASCADE,
	type VARCHAR(16) NOT NULL CHECK (type IN ('folder', 'doc')),
	title VARCHAR(255) NOT NULL,
	sort INTEGER NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS documents (
	id BIGSERIAL PRIMARY KEY,
	document_id VARCHAR(26) NOT NULL UNIQUE,
	node_id VARCHAR(26) NOT NULL UNIQUE REFERENCES nodes(node_id) ON DELETE CASCADE,
	title VARCHAR(255) NOT NULL,
	content_md TEXT NOT NULL DEFAULT '',
	version INTEGER NOT NULL DEFAULT 1,
	updated_by_user_id VARCHAR(26) NULL REFERENCES users(user_id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS document_revisions (
	id BIGSERIAL PRIMARY KEY,
	document_revision_id VARCHAR(26) NOT NULL UNIQUE,
	document_id VARCHAR(26) NOT NULL REFERENCES documents(document_id) ON DELETE CASCADE,
	version INTEGER NOT NULL,
	content_md TEXT NOT NULL,
	base_version INTEGER NOT NULL,
	editor_user_id VARCHAR(26) NULL REFERENCES users(user_id) ON DELETE SET NULL,
	source VARCHAR(16) NOT NULL CHECK (source IN ('local', 'remote')),
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(document_id, version)
);

CREATE TABLE IF NOT EXISTS node_permissions (
	id BIGSERIAL PRIMARY KEY,
	node_id VARCHAR(26) NOT NULL REFERENCES nodes(node_id) ON DELETE CASCADE,
	user_id VARCHAR(26) NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
	role VARCHAR(16) NOT NULL CHECK (role IN ('owner', 'collaborator', 'reader')),
	granted_by_user_id VARCHAR(26) NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(node_id, user_id)
);

CREATE TABLE IF NOT EXISTS document_permissions (
	id BIGSERIAL PRIMARY KEY,
	document_id VARCHAR(26) NOT NULL REFERENCES documents(document_id) ON DELETE CASCADE,
	user_id VARCHAR(26) NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
	role VARCHAR(16) NOT NULL CHECK (role IN ('owner', 'collaborator', 'reader')),
	granted_by_user_id VARCHAR(26) NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(document_id, user_id)
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
