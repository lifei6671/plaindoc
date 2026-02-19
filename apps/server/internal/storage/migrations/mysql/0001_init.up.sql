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

CREATE TABLE IF NOT EXISTS themes (
	id BIGINT AUTO_INCREMENT PRIMARY KEY,
	theme_id VARCHAR(64) NOT NULL UNIQUE,
	name VARCHAR(128) NOT NULL,
	description TEXT NOT NULL,
	variables_json MEDIUMTEXT NOT NULL,
	syntax_theme VARCHAR(32) NOT NULL,
	code_block_style_json MEDIUMTEXT NOT NULL,
	code_block_code_style_json MEDIUMTEXT NOT NULL,
	inline_code_style_json MEDIUMTEXT NOT NULL,
	custom_css MEDIUMTEXT NOT NULL,
	is_builtin TINYINT(1) NOT NULL DEFAULT 0,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	CONSTRAINT ck_themes_syntax_theme CHECK (syntax_theme IN ('one-light', 'one-dark'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO themes (
	theme_id,
	name,
	description,
	variables_json,
	syntax_theme,
	code_block_style_json,
	code_block_code_style_json,
	inline_code_style_json,
	custom_css,
	is_builtin
) VALUES (
	'default',
	'内置默认',
	'通用文档风格',
	'{"--pd-preview-padding":"30px","--pd-preview-font-family":"Optima-Regular, Optima, PingFangSC-light, PingFangTC-light, PingFang SC, Cambria, Cochin, Georgia, Times, Times New Roman, serif","--pd-preview-text-color":"rgb(89, 89, 89)","--pd-preview-link-color":"rgb(71, 193, 168)","--pd-preview-inline-code-color":"rgb(71, 193, 168)","--pd-preview-font-size":"16px","--pd-preview-line-height":"26px"}',
	'one-light',
	'{"margin":"16px 0","padding":"14px 16px","borderRadius":"10px","border":"1px solid #dbe2ea","boxShadow":"0 1px 2px rgba(15, 23, 42, 0.06)","overflowX":"auto","fontSize":"13px","lineHeight":"1.65","background":"#f8fafc"}',
	'{"fontFamily":"Google Sans Code, Operator Mono, SFMono-Regular, Menlo, Monaco, Consolas, Liberation Mono, Courier New, monospace"}',
	'{"padding":"1px 6px","borderRadius":"5px","border":"1px solid #dbe2ea","background":"#f1f5f9","color":"#0f172a","fontSize":"0.92em","fontFamily":"Google Sans Code, Operator Mono, SFMono-Regular, Menlo, Monaco, Consolas, Liberation Mono, Courier New, monospace"}',
	'',
	1
)
ON DUPLICATE KEY UPDATE theme_id = theme_id;

CREATE TABLE IF NOT EXISTS documents (
	id BIGINT AUTO_INCREMENT PRIMARY KEY,
	document_id VARCHAR(26) NOT NULL UNIQUE,
	node_id VARCHAR(26) NOT NULL UNIQUE,
	theme_id VARCHAR(64) NOT NULL DEFAULT 'default',
	title VARCHAR(255) NOT NULL,
	content_md MEDIUMTEXT NOT NULL,
	version INT NOT NULL DEFAULT 1,
	updated_by_user_id VARCHAR(26) NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	CONSTRAINT fk_documents_node_id FOREIGN KEY (node_id) REFERENCES nodes(node_id) ON DELETE CASCADE,
	CONSTRAINT fk_documents_theme_id FOREIGN KEY (theme_id) REFERENCES themes(theme_id) ON DELETE RESTRICT,
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

CREATE INDEX idx_themes_updated_at ON themes(updated_at);

CREATE INDEX idx_documents_node_id ON documents(node_id);
CREATE INDEX idx_documents_theme_id ON documents(theme_id);
CREATE INDEX idx_documents_updated_at ON documents(updated_at);

CREATE INDEX idx_revisions_document_id ON document_revisions(document_id);
CREATE INDEX idx_revisions_document_version ON document_revisions(document_id, version);
CREATE INDEX idx_revisions_document_created_at ON document_revisions(document_id, created_at);

CREATE INDEX idx_node_permissions_node_id ON node_permissions(node_id);
CREATE INDEX idx_node_permissions_user_id ON node_permissions(user_id);

CREATE INDEX idx_document_permissions_document_id ON document_permissions(document_id);
CREATE INDEX idx_document_permissions_user_id ON document_permissions(user_id);
