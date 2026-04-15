PRAGMA foreign_keys = OFF;

ALTER TABLE users RENAME TO users_old_0033;

CREATE TABLE users (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id TEXT NOT NULL UNIQUE,
	email TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	name TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'banned', 'deleted')),
	banned_reason TEXT NOT NULL DEFAULT '',
	banned_at TIMESTAMP NULL,
	deleted_at TIMESTAMP NULL,
	avatar_url TEXT NOT NULL DEFAULT ''
);

INSERT INTO users (
	id,
	user_id,
	email,
	password_hash,
	name,
	created_at,
	updated_at,
	status,
	banned_reason,
	banned_at,
	deleted_at,
	avatar_url
)
SELECT
	id,
	user_id,
	email,
	password_hash,
	name,
	created_at,
	updated_at,
	status,
	banned_reason,
	banned_at,
	deleted_at,
	avatar_url
FROM users_old_0033;

DROP TABLE users_old_0033;

CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);

ALTER TABLE themes RENAME TO themes_old_0033;

CREATE TABLE themes (
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
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	is_enabled INTEGER NOT NULL DEFAULT 1 CHECK (is_enabled IN (0, 1))
);

INSERT INTO themes (
	id,
	theme_id,
	name,
	description,
	variables_json,
	syntax_theme,
	code_block_style_json,
	code_block_code_style_json,
	inline_code_style_json,
	custom_css,
	is_builtin,
	created_at,
	updated_at,
	is_enabled
)
SELECT
	id,
	theme_id,
	name,
	description,
	variables_json,
	syntax_theme,
	code_block_style_json,
	code_block_code_style_json,
	inline_code_style_json,
	custom_css,
	is_builtin,
	created_at,
	updated_at,
	is_enabled
FROM themes_old_0033;

DROP TABLE themes_old_0033;

CREATE INDEX IF NOT EXISTS idx_themes_updated_at ON themes(updated_at);
CREATE INDEX IF NOT EXISTS idx_themes_is_enabled ON themes(is_enabled);

ALTER TABLE space_categories RENAME TO space_categories_old_0033;

CREATE TABLE space_categories (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	category_id TEXT NOT NULL UNIQUE,
	name TEXT NOT NULL UNIQUE,
	is_default INTEGER NOT NULL DEFAULT 0 CHECK (is_default IN (0, 1)),
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO space_categories (
	id,
	category_id,
	name,
	is_default,
	created_at,
	updated_at
)
SELECT
	id,
	category_id,
	name,
	is_default,
	created_at,
	updated_at
FROM space_categories_old_0033;

DROP TABLE space_categories_old_0033;

CREATE INDEX IF NOT EXISTS idx_space_categories_is_default ON space_categories(is_default);

ALTER TABLE spaces RENAME TO spaces_old_0033;

CREATE TABLE spaces (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	space_id TEXT NOT NULL UNIQUE,
	name TEXT NOT NULL,
	owner_user_id TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	visibility TEXT NOT NULL DEFAULT 'member' CHECK (visibility IN ('public', 'authenticated', 'member')),
	status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'banned', 'deleted')),
	banned_reason TEXT NOT NULL DEFAULT '',
	banned_at TIMESTAMP NULL,
	deleted_at TIMESTAMP NULL,
	description TEXT NOT NULL DEFAULT '',
	cover_asset_id TEXT NULL,
	cover_key TEXT NOT NULL DEFAULT '',
	cover_url TEXT NOT NULL DEFAULT '',
	cover_width INTEGER NOT NULL DEFAULT 0,
	cover_height INTEGER NOT NULL DEFAULT 0,
	cover_source TEXT NOT NULL DEFAULT '' CHECK (cover_source IN ('', 'user_upload', 'system_generated')),
	category TEXT NOT NULL DEFAULT '',
	category_id TEXT NOT NULL DEFAULT '01jmf4v2x7m7f1m6qv5kh0t2mn',
	FOREIGN KEY (owner_user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

INSERT INTO spaces (
	id,
	space_id,
	name,
	owner_user_id,
	created_at,
	updated_at,
	visibility,
	status,
	banned_reason,
	banned_at,
	deleted_at,
	description,
	cover_asset_id,
	cover_key,
	cover_url,
	cover_width,
	cover_height,
	cover_source,
	category,
	category_id
)
SELECT
	id,
	space_id,
	name,
	owner_user_id,
	created_at,
	updated_at,
	visibility,
	status,
	banned_reason,
	banned_at,
	deleted_at,
	description,
	cover_asset_id,
	cover_key,
	cover_url,
	cover_width,
	cover_height,
	cover_source,
	category,
	category_id
FROM spaces_old_0033;

DROP TABLE spaces_old_0033;

CREATE INDEX IF NOT EXISTS idx_spaces_owner_user_id ON spaces(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_spaces_updated_at ON spaces(updated_at);
CREATE INDEX IF NOT EXISTS idx_spaces_visibility ON spaces(visibility);
CREATE INDEX IF NOT EXISTS idx_spaces_status ON spaces(status);
CREATE INDEX IF NOT EXISTS idx_spaces_cover_asset_id ON spaces(cover_asset_id);
CREATE INDEX IF NOT EXISTS idx_spaces_category ON spaces(category);
CREATE INDEX IF NOT EXISTS idx_spaces_category_id ON spaces(category_id);

ALTER TABLE space_members RENAME TO space_members_old_0033;

CREATE TABLE space_members (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	space_id TEXT NOT NULL,
	user_id TEXT NOT NULL,
	role TEXT NOT NULL CHECK (role IN ('owner', 'collaborator', 'reader')),
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(space_id, user_id),
	FOREIGN KEY (space_id) REFERENCES spaces(space_id) ON DELETE CASCADE,
	FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

INSERT INTO space_members (
	id,
	space_id,
	user_id,
	role,
	created_at,
	updated_at
)
SELECT
	id,
	space_id,
	user_id,
	role,
	created_at,
	updated_at
FROM space_members_old_0033;

DROP TABLE space_members_old_0033;

CREATE INDEX IF NOT EXISTS idx_space_members_space_id ON space_members(space_id);
CREATE INDEX IF NOT EXISTS idx_space_members_user_id ON space_members(user_id);

ALTER TABLE nodes RENAME TO nodes_old_0033;

CREATE TABLE nodes (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	node_id TEXT NOT NULL UNIQUE,
	space_id TEXT NOT NULL,
	parent_node_id TEXT NULL,
	type TEXT NOT NULL CHECK (type IN ('folder', 'doc')),
	title TEXT NOT NULL,
	sort INTEGER NOT NULL DEFAULT 0,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	created_by_user_id TEXT NULL REFERENCES users(user_id) ON DELETE SET NULL,
	updated_by_user_id TEXT NULL REFERENCES users(user_id) ON DELETE SET NULL,
	reader_slug TEXT NULL,
	FOREIGN KEY (space_id) REFERENCES spaces(space_id) ON DELETE CASCADE,
	FOREIGN KEY (parent_node_id) REFERENCES nodes(node_id) ON DELETE CASCADE
);

INSERT INTO nodes (
	id,
	node_id,
	space_id,
	parent_node_id,
	type,
	title,
	sort,
	created_at,
	updated_at,
	created_by_user_id,
	updated_by_user_id,
	reader_slug
)
SELECT
	id,
	node_id,
	space_id,
	parent_node_id,
	type,
	title,
	sort,
	created_at,
	updated_at,
	created_by_user_id,
	updated_by_user_id,
	reader_slug
FROM nodes_old_0033;

DROP TABLE nodes_old_0033;

CREATE INDEX IF NOT EXISTS idx_nodes_space_id ON nodes(space_id);
CREATE INDEX IF NOT EXISTS idx_nodes_parent_node_id ON nodes(parent_node_id);
CREATE INDEX IF NOT EXISTS idx_nodes_space_parent_sort ON nodes(space_id, parent_node_id, sort);
CREATE INDEX IF NOT EXISTS idx_nodes_type ON nodes(type);
CREATE INDEX IF NOT EXISTS idx_nodes_created_by_user_id ON nodes(created_by_user_id);
CREATE INDEX IF NOT EXISTS idx_nodes_updated_by_user_id ON nodes(updated_by_user_id);
CREATE UNIQUE INDEX IF NOT EXISTS uk_nodes_space_reader_slug
	ON nodes(space_id, reader_slug)
	WHERE reader_slug IS NOT NULL;

ALTER TABLE user_identities RENAME TO user_identities_old_0033;

CREATE TABLE user_identities (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id TEXT NOT NULL,
	provider_type TEXT NOT NULL,
	provider_id TEXT NOT NULL,
	external_id TEXT NOT NULL,
	login_name TEXT NOT NULL DEFAULT '',
	last_login_at TIMESTAMP NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(provider_id, external_id),
	FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

INSERT INTO user_identities (
	id,
	user_id,
	provider_type,
	provider_id,
	external_id,
	login_name,
	last_login_at,
	created_at,
	updated_at
)
SELECT
	id,
	user_id,
	provider_type,
	provider_id,
	external_id,
	login_name,
	last_login_at,
	created_at,
	updated_at
FROM user_identities_old_0033;

DROP TABLE user_identities_old_0033;

CREATE INDEX IF NOT EXISTS idx_user_identities_user_id ON user_identities(user_id);
CREATE INDEX IF NOT EXISTS idx_user_identities_provider_type ON user_identities(provider_type);

ALTER TABLE auth_risk_states RENAME TO auth_risk_states_old_0033;

CREATE TABLE auth_risk_states (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	scene TEXT NOT NULL,
	subject_type TEXT NOT NULL,
	subject_hash TEXT NOT NULL,
	window_started_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
	failed_count INTEGER NOT NULL DEFAULT 0 CHECK (failed_count >= 0),
	captcha_failed_count INTEGER NOT NULL DEFAULT 0 CHECK (captcha_failed_count >= 0),
	lock_until TIMESTAMP NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(scene, subject_type, subject_hash)
);

INSERT INTO auth_risk_states (
	id,
	scene,
	subject_type,
	subject_hash,
	window_started_at,
	attempt_count,
	failed_count,
	captcha_failed_count,
	lock_until,
	created_at,
	updated_at
)
SELECT
	id,
	scene,
	subject_type,
	subject_hash,
	window_started_at,
	attempt_count,
	failed_count,
	captcha_failed_count,
	lock_until,
	created_at,
	updated_at
FROM auth_risk_states_old_0033;

DROP TABLE auth_risk_states_old_0033;

CREATE INDEX IF NOT EXISTS idx_auth_risk_states_scene_subject
	ON auth_risk_states(scene, subject_type, subject_hash);
CREATE INDEX IF NOT EXISTS idx_auth_risk_states_lock_until
	ON auth_risk_states(lock_until);
CREATE INDEX IF NOT EXISTS idx_auth_risk_states_updated_at
	ON auth_risk_states(updated_at);

ALTER TABLE auth_captcha_challenges RENAME TO auth_captcha_challenges_old_0033;

CREATE TABLE auth_captcha_challenges (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	captcha_id TEXT NOT NULL UNIQUE,
	scene TEXT NOT NULL,
	subject_hash TEXT NOT NULL,
	level INTEGER NOT NULL DEFAULT 4 CHECK (level BETWEEN 1 AND 32),
	answer_hash TEXT NOT NULL,
	answer_salt TEXT NOT NULL,
	issued_ip_hash TEXT NOT NULL,
	expires_at TIMESTAMP NOT NULL,
	consumed_at TIMESTAMP NULL,
	failed_verify_count INTEGER NOT NULL DEFAULT 0 CHECK (failed_verify_count >= 0),
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO auth_captcha_challenges (
	id,
	captcha_id,
	scene,
	subject_hash,
	level,
	answer_hash,
	answer_salt,
	issued_ip_hash,
	expires_at,
	consumed_at,
	failed_verify_count,
	created_at,
	updated_at
)
SELECT
	id,
	captcha_id,
	scene,
	subject_hash,
	level,
	answer_hash,
	answer_salt,
	issued_ip_hash,
	expires_at,
	consumed_at,
	failed_verify_count,
	created_at,
	updated_at
FROM auth_captcha_challenges_old_0033;

DROP TABLE auth_captcha_challenges_old_0033;

CREATE INDEX IF NOT EXISTS idx_auth_captcha_challenges_scene_subject
	ON auth_captcha_challenges(scene, subject_hash);
CREATE INDEX IF NOT EXISTS idx_auth_captcha_challenges_expires_at
	ON auth_captcha_challenges(expires_at);
CREATE INDEX IF NOT EXISTS idx_auth_captcha_challenges_consumed_at
	ON auth_captcha_challenges(consumed_at);

ALTER TABLE file_blobs RENAME TO file_blobs_old_0033;

CREATE TABLE file_blobs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	blob_id TEXT NOT NULL UNIQUE,
	storage_provider TEXT NOT NULL DEFAULT 'local',
	object_key TEXT NOT NULL DEFAULT '',
	object_url TEXT NOT NULL DEFAULT '',
	mime_type TEXT NOT NULL DEFAULT 'application/octet-stream',
	size_bytes INTEGER NOT NULL DEFAULT 0,
	content_hash_algo TEXT NOT NULL DEFAULT 'sha256',
	content_hash TEXT NOT NULL DEFAULT '',
	deleted_at TIMESTAMP NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO file_blobs (
	id,
	blob_id,
	storage_provider,
	object_key,
	object_url,
	mime_type,
	size_bytes,
	content_hash_algo,
	content_hash,
	deleted_at,
	created_at,
	updated_at
)
SELECT
	id,
	blob_id,
	storage_provider,
	object_key,
	object_url,
	mime_type,
	size_bytes,
	content_hash_algo,
	content_hash,
	deleted_at,
	created_at,
	updated_at
FROM file_blobs_old_0033;

DROP TABLE file_blobs_old_0033;

CREATE UNIQUE INDEX IF NOT EXISTS uk_file_blobs_hash
	ON file_blobs(storage_provider, content_hash_algo, content_hash, size_bytes);
CREATE INDEX IF NOT EXISTS idx_file_blobs_created_at
	ON file_blobs(created_at);

ALTER TABLE documents RENAME TO documents_old_0033;

CREATE TABLE documents (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	document_id TEXT NOT NULL UNIQUE,
	node_id TEXT NOT NULL UNIQUE,
	theme_id TEXT NOT NULL DEFAULT 'default',
	title TEXT NOT NULL,
	content_md TEXT NOT NULL DEFAULT '',
	version INTEGER NOT NULL DEFAULT 1,
	updated_by_user_id TEXT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	visibility TEXT NOT NULL DEFAULT 'member' CHECK (visibility IN ('public', 'authenticated', 'member')),
	status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'banned', 'deleted')),
	banned_reason TEXT NOT NULL DEFAULT '',
	banned_at TIMESTAMP NULL,
	deleted_at TIMESTAMP NULL,
	created_by_user_id TEXT NULL REFERENCES users(user_id) ON DELETE SET NULL,
	format TEXT NOT NULL DEFAULT 'markdown' CHECK (format IN ('markdown', 'docx', 'xlsx')),
	source_blob_id TEXT NULL REFERENCES file_blobs(blob_id) ON DELETE SET NULL,
	source_file_name TEXT NULL,
	source_mime_type TEXT NULL,
	content_version INTEGER NOT NULL DEFAULT 1 CHECK (content_version > 0),
	render_status TEXT NOT NULL DEFAULT 'idle' CHECK (render_status IN ('idle', 'pending', 'success', 'failed')),
	render_error TEXT NOT NULL DEFAULT '',
	rendered_at TIMESTAMP NULL,
	FOREIGN KEY (node_id) REFERENCES nodes(node_id) ON DELETE CASCADE,
	FOREIGN KEY (theme_id) REFERENCES themes(theme_id) ON DELETE RESTRICT,
	FOREIGN KEY (updated_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL
);

INSERT INTO documents (
	id,
	document_id,
	node_id,
	theme_id,
	title,
	content_md,
	version,
	updated_by_user_id,
	created_at,
	updated_at,
	visibility,
	status,
	banned_reason,
	banned_at,
	deleted_at,
	created_by_user_id,
	format,
	source_blob_id,
	source_file_name,
	source_mime_type,
	content_version,
	render_status,
	render_error,
	rendered_at
)
SELECT
	id,
	document_id,
	node_id,
	theme_id,
	title,
	content_md,
	version,
	updated_by_user_id,
	created_at,
	updated_at,
	visibility,
	status,
	banned_reason,
	banned_at,
	deleted_at,
	created_by_user_id,
	format,
	source_blob_id,
	source_file_name,
	source_mime_type,
	content_version,
	render_status,
	render_error,
	rendered_at
FROM documents_old_0033;

DROP TABLE documents_old_0033;

CREATE INDEX IF NOT EXISTS idx_documents_node_id ON documents(node_id);
CREATE INDEX IF NOT EXISTS idx_documents_theme_id ON documents(theme_id);
CREATE INDEX IF NOT EXISTS idx_documents_updated_at ON documents(updated_at);
CREATE INDEX IF NOT EXISTS idx_documents_visibility ON documents(visibility);
CREATE INDEX IF NOT EXISTS idx_documents_status ON documents(status);
CREATE INDEX IF NOT EXISTS idx_documents_created_by_user_id ON documents(created_by_user_id);
CREATE INDEX IF NOT EXISTS idx_documents_format ON documents(format);
CREATE INDEX IF NOT EXISTS idx_documents_source_blob_id ON documents(source_blob_id);
CREATE INDEX IF NOT EXISTS idx_documents_render_status ON documents(render_status);

ALTER TABLE document_revisions RENAME TO document_revisions_old_0033;

CREATE TABLE document_revisions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	document_revision_id TEXT NOT NULL UNIQUE,
	document_id TEXT NOT NULL,
	version INTEGER NOT NULL,
	content_md TEXT NOT NULL,
	base_version INTEGER NOT NULL,
	editor_user_id TEXT NULL,
	source TEXT NOT NULL CHECK (source IN ('local', 'remote')),
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(document_id, version),
	FOREIGN KEY (document_id) REFERENCES documents(document_id) ON DELETE CASCADE,
	FOREIGN KEY (editor_user_id) REFERENCES users(user_id) ON DELETE SET NULL
);

INSERT INTO document_revisions (
	id,
	document_revision_id,
	document_id,
	version,
	content_md,
	base_version,
	editor_user_id,
	source,
	created_at
)
SELECT
	id,
	document_revision_id,
	document_id,
	version,
	content_md,
	base_version,
	editor_user_id,
	source,
	created_at
FROM document_revisions_old_0033;

DROP TABLE document_revisions_old_0033;

CREATE INDEX IF NOT EXISTS idx_revisions_document_id ON document_revisions(document_id);
CREATE INDEX IF NOT EXISTS idx_revisions_document_version ON document_revisions(document_id, version);
CREATE INDEX IF NOT EXISTS idx_revisions_document_created_at ON document_revisions(document_id, created_at);

ALTER TABLE node_permissions RENAME TO node_permissions_old_0033;

CREATE TABLE node_permissions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	node_id TEXT NOT NULL,
	user_id TEXT NOT NULL,
	role TEXT NOT NULL CHECK (role IN ('owner', 'collaborator', 'reader')),
	granted_by_user_id TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(node_id, user_id),
	FOREIGN KEY (node_id) REFERENCES nodes(node_id) ON DELETE CASCADE,
	FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
	FOREIGN KEY (granted_by_user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

INSERT INTO node_permissions (
	id,
	node_id,
	user_id,
	role,
	granted_by_user_id,
	created_at,
	updated_at
)
SELECT
	id,
	node_id,
	user_id,
	role,
	granted_by_user_id,
	created_at,
	updated_at
FROM node_permissions_old_0033;

DROP TABLE node_permissions_old_0033;

CREATE INDEX IF NOT EXISTS idx_node_permissions_node_id ON node_permissions(node_id);
CREATE INDEX IF NOT EXISTS idx_node_permissions_user_id ON node_permissions(user_id);

ALTER TABLE document_permissions RENAME TO document_permissions_old_0033;

CREATE TABLE document_permissions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	document_id TEXT NOT NULL,
	user_id TEXT NOT NULL,
	role TEXT NOT NULL CHECK (role IN ('owner', 'collaborator', 'reader')),
	granted_by_user_id TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(document_id, user_id),
	FOREIGN KEY (document_id) REFERENCES documents(document_id) ON DELETE CASCADE,
	FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
	FOREIGN KEY (granted_by_user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

INSERT INTO document_permissions (
	id,
	document_id,
	user_id,
	role,
	granted_by_user_id,
	created_at,
	updated_at
)
SELECT
	id,
	document_id,
	user_id,
	role,
	granted_by_user_id,
	created_at,
	updated_at
FROM document_permissions_old_0033;

DROP TABLE document_permissions_old_0033;

CREATE INDEX IF NOT EXISTS idx_document_permissions_document_id ON document_permissions(document_id);
CREATE INDEX IF NOT EXISTS idx_document_permissions_user_id ON document_permissions(user_id);

ALTER TABLE document_attachments RENAME TO document_attachments_old_0033;

CREATE TABLE document_attachments (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	attachment_id TEXT NOT NULL UNIQUE,
	blob_id TEXT NOT NULL,
	document_id TEXT NOT NULL,
	space_id TEXT NOT NULL,
	storage_provider TEXT NOT NULL DEFAULT 'local',
	file_name TEXT NOT NULL,
	object_key TEXT NOT NULL DEFAULT '',
	object_url TEXT NOT NULL DEFAULT '',
	mime_type TEXT NOT NULL DEFAULT 'application/octet-stream',
	size_bytes INTEGER NOT NULL DEFAULT 0,
	content_hash_algo TEXT NOT NULL DEFAULT 'sha256',
	content_hash TEXT NOT NULL DEFAULT '',
	preview_kind TEXT NOT NULL DEFAULT 'none',
	status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'deleted')),
	deleted_at TIMESTAMP NULL,
	created_by_user_id TEXT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (blob_id) REFERENCES file_blobs(blob_id) ON DELETE RESTRICT,
	FOREIGN KEY (document_id) REFERENCES documents(document_id) ON DELETE CASCADE,
	FOREIGN KEY (space_id) REFERENCES spaces(space_id) ON DELETE CASCADE,
	FOREIGN KEY (created_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL
);

INSERT INTO document_attachments (
	id,
	attachment_id,
	blob_id,
	document_id,
	space_id,
	storage_provider,
	file_name,
	object_key,
	object_url,
	mime_type,
	size_bytes,
	content_hash_algo,
	content_hash,
	preview_kind,
	status,
	deleted_at,
	created_by_user_id,
	created_at,
	updated_at
)
SELECT
	id,
	attachment_id,
	blob_id,
	document_id,
	space_id,
	storage_provider,
	file_name,
	object_key,
	object_url,
	mime_type,
	size_bytes,
	content_hash_algo,
	content_hash,
	preview_kind,
	status,
	deleted_at,
	created_by_user_id,
	created_at,
	updated_at
FROM document_attachments_old_0033;

DROP TABLE document_attachments_old_0033;

CREATE INDEX IF NOT EXISTS idx_document_attachments_blob_id
	ON document_attachments(blob_id);
CREATE INDEX IF NOT EXISTS idx_document_attachments_document_id
	ON document_attachments(document_id);
CREATE INDEX IF NOT EXISTS idx_document_attachments_space_id
	ON document_attachments(space_id);
CREATE INDEX IF NOT EXISTS idx_document_attachments_status
	ON document_attachments(status);
CREATE INDEX IF NOT EXISTS idx_document_attachments_created_at
	ON document_attachments(created_at);
CREATE INDEX IF NOT EXISTS idx_document_attachments_hash
	ON document_attachments(storage_provider, content_hash_algo, content_hash, size_bytes);

ALTER TABLE document_image_assets RENAME TO document_image_assets_old_0033;

CREATE TABLE document_image_assets (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	image_asset_id TEXT NOT NULL UNIQUE,
	document_id TEXT NOT NULL,
	space_id TEXT NOT NULL,
	storage_provider TEXT NOT NULL DEFAULT 'local',
	object_key TEXT NOT NULL DEFAULT '',
	object_url TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'pending_cleanup', 'deleted')),
	pending_cleanup_at TIMESTAMP NULL,
	deleted_at TIMESTAMP NULL,
	last_referenced_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	blob_id TEXT NULL,
	FOREIGN KEY (document_id) REFERENCES documents(document_id) ON DELETE CASCADE,
	FOREIGN KEY (space_id) REFERENCES spaces(space_id) ON DELETE CASCADE,
	FOREIGN KEY (blob_id) REFERENCES file_blobs(blob_id) ON DELETE SET NULL
);

INSERT INTO document_image_assets (
	id,
	image_asset_id,
	document_id,
	space_id,
	storage_provider,
	object_key,
	object_url,
	status,
	pending_cleanup_at,
	deleted_at,
	last_referenced_at,
	created_at,
	updated_at,
	blob_id
)
SELECT
	id,
	image_asset_id,
	document_id,
	space_id,
	storage_provider,
	object_key,
	object_url,
	status,
	pending_cleanup_at,
	deleted_at,
	last_referenced_at,
	created_at,
	updated_at,
	blob_id
FROM document_image_assets_old_0033;

DROP TABLE document_image_assets_old_0033;

CREATE UNIQUE INDEX IF NOT EXISTS uk_document_image_assets_doc_object
	ON document_image_assets(document_id, storage_provider, object_key);
CREATE INDEX IF NOT EXISTS idx_document_image_assets_pending
	ON document_image_assets(status, pending_cleanup_at);
CREATE INDEX IF NOT EXISTS idx_document_image_assets_object
	ON document_image_assets(storage_provider, object_key, status);
CREATE INDEX IF NOT EXISTS idx_document_image_assets_space_id
	ON document_image_assets(space_id);
CREATE INDEX IF NOT EXISTS idx_document_image_assets_created_at
	ON document_image_assets(created_at);
CREATE INDEX IF NOT EXISTS idx_document_image_assets_blob_id
	ON document_image_assets(blob_id);

ALTER TABLE search_analyzer_dict_entries RENAME TO search_analyzer_dict_entries_old_0033;

CREATE TABLE search_analyzer_dict_entries (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	analyzer TEXT NOT NULL,
	term TEXT NOT NULL,
	weight INTEGER NULL,
	tag TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'deleted')),
	created_by_user_id TEXT NULL,
	updated_by_user_id TEXT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (created_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL,
	FOREIGN KEY (updated_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL
);

INSERT INTO search_analyzer_dict_entries (
	id,
	analyzer,
	term,
	weight,
	tag,
	status,
	created_by_user_id,
	updated_by_user_id,
	created_at,
	updated_at
)
SELECT
	id,
	analyzer,
	term,
	weight,
	tag,
	status,
	created_by_user_id,
	updated_by_user_id,
	created_at,
	updated_at
FROM search_analyzer_dict_entries_old_0033;

DROP TABLE search_analyzer_dict_entries_old_0033;

CREATE UNIQUE INDEX IF NOT EXISTS uk_search_analyzer_dict_entries_analyzer_term
	ON search_analyzer_dict_entries(analyzer, term);
CREATE INDEX IF NOT EXISTS idx_search_analyzer_dict_entries_active_lookup
	ON search_analyzer_dict_entries(analyzer, status, updated_at);

ALTER TABLE search_index_jobs RENAME TO search_index_jobs_old_0033;

CREATE TABLE search_index_jobs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	job_id TEXT NOT NULL UNIQUE,
	provider TEXT NOT NULL DEFAULT '',
	job_type TEXT NOT NULL,
	dedupe_key TEXT NOT NULL,
	payload_json TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'success', 'failed')),
	priority INTEGER NOT NULL DEFAULT 100,
	retry_count INTEGER NOT NULL DEFAULT 0,
	next_run_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	started_at TIMESTAMP NULL,
	last_error TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO search_index_jobs (
	id,
	job_id,
	provider,
	job_type,
	dedupe_key,
	payload_json,
	status,
	priority,
	retry_count,
	next_run_at,
	started_at,
	last_error,
	created_at,
	updated_at
)
SELECT
	id,
	job_id,
	provider,
	job_type,
	dedupe_key,
	payload_json,
	status,
	priority,
	retry_count,
	next_run_at,
	started_at,
	last_error,
	created_at,
	updated_at
FROM search_index_jobs_old_0033;

DROP TABLE search_index_jobs_old_0033;

CREATE INDEX IF NOT EXISTS idx_search_index_jobs_status_next_priority
	ON search_index_jobs(status, next_run_at, priority, id);
CREATE INDEX IF NOT EXISTS idx_search_index_jobs_dedupe_status
	ON search_index_jobs(dedupe_key, status, id);
CREATE INDEX IF NOT EXISTS idx_search_index_jobs_created_at
	ON search_index_jobs(created_at);

ALTER TABLE password_reset_tokens RENAME TO password_reset_tokens_old_0033;

CREATE TABLE password_reset_tokens (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	token_id TEXT NOT NULL UNIQUE,
	token_secret_hash TEXT NOT NULL,
	user_id TEXT NOT NULL,
	source TEXT NOT NULL DEFAULT 'self_service' CHECK (source IN ('self_service', 'admin_initiated')),
	requested_by_user_id TEXT NULL,
	request_ip_hash TEXT NOT NULL DEFAULT '',
	expires_at TIMESTAMP NOT NULL,
	consumed_at TIMESTAMP NULL,
	invalidated_at TIMESTAMP NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
	FOREIGN KEY (requested_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL
);

INSERT INTO password_reset_tokens (
	id,
	token_id,
	token_secret_hash,
	user_id,
	source,
	requested_by_user_id,
	request_ip_hash,
	expires_at,
	consumed_at,
	invalidated_at,
	created_at,
	updated_at
)
SELECT
	id,
	token_id,
	token_secret_hash,
	user_id,
	source,
	requested_by_user_id,
	request_ip_hash,
	expires_at,
	consumed_at,
	invalidated_at,
	created_at,
	updated_at
FROM password_reset_tokens_old_0033;

DROP TABLE password_reset_tokens_old_0033;

CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user_active
	ON password_reset_tokens(user_id, consumed_at, invalidated_at, expires_at);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_expires_at
	ON password_reset_tokens(expires_at);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_request_ip_created
	ON password_reset_tokens(request_ip_hash, created_at);

ALTER TABLE document_template_scenes RENAME TO document_template_scenes_old_0033;

CREATE TABLE document_template_scenes (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	scene_key TEXT NOT NULL UNIQUE,
	scene_name TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	sort INTEGER NOT NULL DEFAULT 0,
	is_builtin INTEGER NOT NULL DEFAULT 0 CHECK (is_builtin IN (0, 1)),
	created_by_user_id TEXT NULL,
	updated_by_user_id TEXT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (created_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL,
	FOREIGN KEY (updated_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL,
	CHECK (length(trim(scene_key)) > 0)
);

INSERT INTO document_template_scenes (
	id,
	scene_key,
	scene_name,
	description,
	sort,
	is_builtin,
	created_by_user_id,
	updated_by_user_id,
	created_at,
	updated_at
)
SELECT
	id,
	scene_key,
	scene_name,
	description,
	sort,
	is_builtin,
	created_by_user_id,
	updated_by_user_id,
	created_at,
	updated_at
FROM document_template_scenes_old_0033;

DROP TABLE document_template_scenes_old_0033;

CREATE INDEX IF NOT EXISTS idx_document_template_scenes_sort_updated_at
	ON document_template_scenes(sort, updated_at);

ALTER TABLE document_templates RENAME TO document_templates_old_0033;

CREATE TABLE document_templates (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	template_id TEXT NOT NULL UNIQUE,
	scene_key TEXT NOT NULL,
	name TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	default_title TEXT NOT NULL DEFAULT '',
	content_md TEXT NOT NULL DEFAULT '',
	sort INTEGER NOT NULL DEFAULT 0,
	is_builtin INTEGER NOT NULL DEFAULT 0 CHECK (is_builtin IN (0, 1)),
	is_enabled INTEGER NOT NULL DEFAULT 1 CHECK (is_enabled IN (0, 1)),
	created_by_user_id TEXT NULL,
	updated_by_user_id TEXT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (created_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL,
	FOREIGN KEY (updated_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL,
	CHECK (length(trim(template_id)) > 0),
	CHECK (length(trim(scene_key)) > 0)
);

INSERT INTO document_templates (
	id,
	template_id,
	scene_key,
	name,
	description,
	default_title,
	content_md,
	sort,
	is_builtin,
	is_enabled,
	created_by_user_id,
	updated_by_user_id,
	created_at,
	updated_at
)
SELECT
	id,
	template_id,
	scene_key,
	name,
	description,
	default_title,
	content_md,
	sort,
	is_builtin,
	is_enabled,
	created_by_user_id,
	updated_by_user_id,
	created_at,
	updated_at
FROM document_templates_old_0033;

DROP TABLE document_templates_old_0033;

CREATE INDEX IF NOT EXISTS idx_document_templates_scene_enabled_sort_updated_at
	ON document_templates(scene_key, is_enabled, sort, updated_at);
CREATE INDEX IF NOT EXISTS idx_document_templates_enabled_updated_at
	ON document_templates(is_enabled, updated_at);

ALTER TABLE document_shares RENAME TO document_shares_old_0033;

CREATE TABLE document_shares (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	share_id TEXT NOT NULL UNIQUE,
	document_id TEXT NOT NULL UNIQUE,
	space_id TEXT NOT NULL,
	mode TEXT NOT NULL DEFAULT 'public' CHECK (mode IN ('public', 'password')),
	password_hash TEXT NULL,
	password_hint TEXT NOT NULL DEFAULT '',
	expires_at TIMESTAMP NULL,
	disabled_at TIMESTAMP NULL,
	access_version INTEGER NOT NULL DEFAULT 1 CHECK (access_version > 0),
	created_by_user_id TEXT NULL,
	updated_by_user_id TEXT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (document_id) REFERENCES documents(document_id) ON DELETE CASCADE,
	FOREIGN KEY (space_id) REFERENCES spaces(space_id) ON DELETE CASCADE,
	FOREIGN KEY (created_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL,
	FOREIGN KEY (updated_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL,
	CHECK (length(trim(share_id)) > 0)
);

INSERT INTO document_shares (
	id,
	share_id,
	document_id,
	space_id,
	mode,
	password_hash,
	password_hint,
	expires_at,
	disabled_at,
	access_version,
	created_by_user_id,
	updated_by_user_id,
	created_at,
	updated_at
)
SELECT
	id,
	share_id,
	document_id,
	space_id,
	mode,
	password_hash,
	password_hint,
	expires_at,
	disabled_at,
	access_version,
	created_by_user_id,
	updated_by_user_id,
	created_at,
	updated_at
FROM document_shares_old_0033;

DROP TABLE document_shares_old_0033;

CREATE INDEX IF NOT EXISTS idx_document_shares_space_disabled_expires
	ON document_shares(space_id, disabled_at, expires_at);
CREATE INDEX IF NOT EXISTS idx_document_shares_mode
	ON document_shares(mode);
CREATE INDEX IF NOT EXISTS idx_document_shares_created_by_user_id
	ON document_shares(created_by_user_id);

ALTER TABLE document_file_revisions RENAME TO document_file_revisions_old_0033;

CREATE TABLE document_file_revisions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	document_file_revision_id TEXT NOT NULL UNIQUE,
	document_id TEXT NOT NULL,
	blob_id TEXT NOT NULL,
	file_name TEXT NOT NULL,
	mime_type TEXT NOT NULL,
	version INTEGER NOT NULL,
	base_version INTEGER NOT NULL,
	editor_user_id TEXT NULL,
	source TEXT NOT NULL CHECK (source IN ('local', 'remote')),
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(document_id, version),
	FOREIGN KEY (document_id) REFERENCES documents(document_id) ON DELETE CASCADE,
	FOREIGN KEY (blob_id) REFERENCES file_blobs(blob_id) ON DELETE RESTRICT,
	FOREIGN KEY (editor_user_id) REFERENCES users(user_id) ON DELETE SET NULL
);

INSERT INTO document_file_revisions (
	id,
	document_file_revision_id,
	document_id,
	blob_id,
	file_name,
	mime_type,
	version,
	base_version,
	editor_user_id,
	source,
	created_at
)
SELECT
	id,
	document_file_revision_id,
	document_id,
	blob_id,
	file_name,
	mime_type,
	version,
	base_version,
	editor_user_id,
	source,
	created_at
FROM document_file_revisions_old_0033;

DROP TABLE document_file_revisions_old_0033;

CREATE INDEX IF NOT EXISTS idx_document_file_revisions_document_id
	ON document_file_revisions(document_id);
CREATE INDEX IF NOT EXISTS idx_document_file_revisions_blob_id
	ON document_file_revisions(blob_id);
CREATE INDEX IF NOT EXISTS idx_document_file_revisions_created_at
	ON document_file_revisions(created_at);

PRAGMA foreign_keys = ON;
