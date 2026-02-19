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

CREATE TABLE IF NOT EXISTS themes (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	theme_id TEXT NOT NULL UNIQUE,
	name TEXT NOT NULL,
	description TEXT NOT NULL,
	variables_json TEXT NOT NULL,
	syntax_theme TEXT NOT NULL CHECK (syntax_theme IN ('one-light', 'one-dark')),
	code_block_style_json TEXT NOT NULL,
	code_block_code_style_json TEXT NOT NULL,
	inline_code_style_json TEXT NOT NULL,
	custom_css TEXT NOT NULL DEFAULT '',
	is_builtin INTEGER NOT NULL DEFAULT 0 CHECK (is_builtin IN (0, 1)),
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT OR IGNORE INTO themes (
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
);

CREATE TABLE IF NOT EXISTS documents (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	document_id TEXT NOT NULL UNIQUE,
	node_id TEXT NOT NULL UNIQUE,
	theme_id TEXT NOT NULL DEFAULT 'default',
	title TEXT NOT NULL,
	content_md TEXT NOT NULL DEFAULT '',
	version INTEGER NOT NULL DEFAULT 1,
	updated_by_user_id TEXT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (node_id) REFERENCES nodes(node_id) ON DELETE CASCADE,
	FOREIGN KEY (theme_id) REFERENCES themes(theme_id) ON DELETE RESTRICT,
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

CREATE INDEX IF NOT EXISTS idx_themes_updated_at ON themes(updated_at);

CREATE INDEX IF NOT EXISTS idx_documents_node_id ON documents(node_id);
CREATE INDEX IF NOT EXISTS idx_documents_theme_id ON documents(theme_id);
CREATE INDEX IF NOT EXISTS idx_documents_updated_at ON documents(updated_at);

CREATE INDEX IF NOT EXISTS idx_revisions_document_id ON document_revisions(document_id);
CREATE INDEX IF NOT EXISTS idx_revisions_document_version ON document_revisions(document_id, version);
CREATE INDEX IF NOT EXISTS idx_revisions_document_created_at ON document_revisions(document_id, created_at);

CREATE INDEX IF NOT EXISTS idx_node_permissions_node_id ON node_permissions(node_id);
CREATE INDEX IF NOT EXISTS idx_node_permissions_user_id ON node_permissions(user_id);

CREATE INDEX IF NOT EXISTS idx_document_permissions_document_id ON document_permissions(document_id);
CREATE INDEX IF NOT EXISTS idx_document_permissions_user_id ON document_permissions(user_id);
