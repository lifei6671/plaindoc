CREATE TABLE IF NOT EXISTS document_template_scenes (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	scene_key TEXT NOT NULL UNIQUE,
	scene_name TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	sort INTEGER NOT NULL DEFAULT 0,
	is_builtin INTEGER NOT NULL DEFAULT 0 CHECK (is_builtin IN (0, 1)),
	created_by_user_id TEXT NULL,
	updated_by_user_id TEXT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (created_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL,
	FOREIGN KEY (updated_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL,
	CHECK (length(trim(scene_key)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_document_template_scenes_sort_updated_at
	ON document_template_scenes(sort, updated_at);

INSERT OR IGNORE INTO document_template_scenes (
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
	t.scene_key,
	t.scene_name,
	'',
	0,
	0,
	t.created_by_user_id,
	t.updated_by_user_id,
	t.created_at,
	t.updated_at
FROM document_templates AS t
WHERE length(trim(t.scene_key)) > 0
	AND t.id = (
		SELECT t2.id
		FROM document_templates AS t2
		WHERE t2.scene_key = t.scene_key
		ORDER BY t2.updated_at DESC, t2.id DESC
		LIMIT 1
	);

PRAGMA foreign_keys = OFF;

CREATE TABLE document_templates_new (
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
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (created_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL,
	FOREIGN KEY (updated_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL,
	CHECK (length(trim(template_id)) > 0),
	CHECK (length(trim(scene_key)) > 0)
);

INSERT INTO document_templates_new (
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
FROM document_templates;

DROP TABLE document_templates;
ALTER TABLE document_templates_new RENAME TO document_templates;

CREATE INDEX IF NOT EXISTS idx_document_templates_scene_enabled_sort_updated_at
	ON document_templates(scene_key, is_enabled, sort, updated_at);
CREATE INDEX IF NOT EXISTS idx_document_templates_enabled_updated_at
	ON document_templates(is_enabled, updated_at);

PRAGMA foreign_keys = ON;
