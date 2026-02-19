CREATE TABLE IF NOT EXISTS users (
	id BIGINT AUTO_INCREMENT PRIMARY KEY,
	user_id VARCHAR(26) NOT NULL UNIQUE,
	email VARCHAR(320) NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	name VARCHAR(128) NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS spaces (
	id BIGINT AUTO_INCREMENT PRIMARY KEY,
	space_id VARCHAR(26) NOT NULL UNIQUE,
	name VARCHAR(255) NOT NULL,
	owner_user_id VARCHAR(26) NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	CONSTRAINT fk_spaces_owner_user_id FOREIGN KEY (owner_user_id) REFERENCES users(user_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS space_members (
	id BIGINT AUTO_INCREMENT PRIMARY KEY,
	space_id VARCHAR(26) NOT NULL,
	user_id VARCHAR(26) NOT NULL,
	role VARCHAR(16) NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	UNIQUE KEY uk_space_members_space_user (space_id, user_id),
	CONSTRAINT ck_space_members_role CHECK (role IN ('owner', 'collaborator', 'reader')),
	CONSTRAINT fk_space_members_space_id FOREIGN KEY (space_id) REFERENCES spaces(space_id) ON DELETE CASCADE,
	CONSTRAINT fk_space_members_user_id FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS nodes (
	id BIGINT AUTO_INCREMENT PRIMARY KEY,
	node_id VARCHAR(26) NOT NULL UNIQUE,
	space_id VARCHAR(26) NOT NULL,
	parent_node_id VARCHAR(26) NULL,
	type VARCHAR(16) NOT NULL,
	title VARCHAR(255) NOT NULL,
	sort INT NOT NULL DEFAULT 0,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	CONSTRAINT ck_nodes_type CHECK (type IN ('folder', 'doc')),
	CONSTRAINT fk_nodes_space_id FOREIGN KEY (space_id) REFERENCES spaces(space_id) ON DELETE CASCADE,
	CONSTRAINT fk_nodes_parent_node_id FOREIGN KEY (parent_node_id) REFERENCES nodes(node_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS documents (
	id BIGINT AUTO_INCREMENT PRIMARY KEY,
	document_id VARCHAR(26) NOT NULL UNIQUE,
	node_id VARCHAR(26) NOT NULL UNIQUE,
	title VARCHAR(255) NOT NULL,
	content_md MEDIUMTEXT NOT NULL,
	version INT NOT NULL DEFAULT 1,
	updated_by_user_id VARCHAR(26) NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	CONSTRAINT fk_documents_node_id FOREIGN KEY (node_id) REFERENCES nodes(node_id) ON DELETE CASCADE,
	CONSTRAINT fk_documents_updated_by_user_id FOREIGN KEY (updated_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS document_revisions (
	id BIGINT AUTO_INCREMENT PRIMARY KEY,
	document_revision_id VARCHAR(26) NOT NULL UNIQUE,
	document_id VARCHAR(26) NOT NULL,
	version INT NOT NULL,
	content_md MEDIUMTEXT NOT NULL,
	base_version INT NOT NULL,
	editor_user_id VARCHAR(26) NULL,
	source VARCHAR(16) NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE KEY uk_document_revisions_document_version (document_id, version),
	CONSTRAINT ck_document_revisions_source CHECK (source IN ('local', 'remote')),
	CONSTRAINT fk_document_revisions_document_id FOREIGN KEY (document_id) REFERENCES documents(document_id) ON DELETE CASCADE,
	CONSTRAINT fk_document_revisions_editor_user_id FOREIGN KEY (editor_user_id) REFERENCES users(user_id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS node_permissions (
	id BIGINT AUTO_INCREMENT PRIMARY KEY,
	node_id VARCHAR(26) NOT NULL,
	user_id VARCHAR(26) NOT NULL,
	role VARCHAR(16) NOT NULL,
	granted_by_user_id VARCHAR(26) NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	UNIQUE KEY uk_node_permissions_node_user (node_id, user_id),
	CONSTRAINT ck_node_permissions_role CHECK (role IN ('owner', 'collaborator', 'reader')),
	CONSTRAINT fk_node_permissions_node_id FOREIGN KEY (node_id) REFERENCES nodes(node_id) ON DELETE CASCADE,
	CONSTRAINT fk_node_permissions_user_id FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
	CONSTRAINT fk_node_permissions_granted_by_user_id FOREIGN KEY (granted_by_user_id) REFERENCES users(user_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS document_permissions (
	id BIGINT AUTO_INCREMENT PRIMARY KEY,
	document_id VARCHAR(26) NOT NULL,
	user_id VARCHAR(26) NOT NULL,
	role VARCHAR(16) NOT NULL,
	granted_by_user_id VARCHAR(26) NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	UNIQUE KEY uk_document_permissions_doc_user (document_id, user_id),
	CONSTRAINT ck_document_permissions_role CHECK (role IN ('owner', 'collaborator', 'reader')),
	CONSTRAINT fk_document_permissions_document_id FOREIGN KEY (document_id) REFERENCES documents(document_id) ON DELETE CASCADE,
	CONSTRAINT fk_document_permissions_user_id FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
	CONSTRAINT fk_document_permissions_granted_by_user_id FOREIGN KEY (granted_by_user_id) REFERENCES users(user_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE INDEX idx_spaces_owner_user_id ON spaces(owner_user_id);
CREATE INDEX idx_spaces_updated_at ON spaces(updated_at);

CREATE INDEX idx_space_members_space_id ON space_members(space_id);
CREATE INDEX idx_space_members_user_id ON space_members(user_id);

CREATE INDEX idx_nodes_space_id ON nodes(space_id);
CREATE INDEX idx_nodes_parent_node_id ON nodes(parent_node_id);
CREATE INDEX idx_nodes_space_parent_sort ON nodes(space_id, parent_node_id, sort);
CREATE INDEX idx_nodes_type ON nodes(type);

CREATE INDEX idx_documents_node_id ON documents(node_id);
CREATE INDEX idx_documents_updated_at ON documents(updated_at);

CREATE INDEX idx_revisions_document_id ON document_revisions(document_id);
CREATE INDEX idx_revisions_document_version ON document_revisions(document_id, version);
CREATE INDEX idx_revisions_document_created_at ON document_revisions(document_id, created_at);

CREATE INDEX idx_node_permissions_node_id ON node_permissions(node_id);
CREATE INDEX idx_node_permissions_user_id ON node_permissions(user_id);

CREATE INDEX idx_document_permissions_document_id ON document_permissions(document_id);
CREATE INDEX idx_document_permissions_user_id ON document_permissions(user_id);
