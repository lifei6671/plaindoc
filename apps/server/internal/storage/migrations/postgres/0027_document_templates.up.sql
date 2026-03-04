CREATE TABLE IF NOT EXISTS document_templates (
	id BIGSERIAL PRIMARY KEY,
	template_id VARCHAR(64) NOT NULL UNIQUE,
	scene_key VARCHAR(64) NOT NULL,
	scene_name VARCHAR(80) NOT NULL,
	name VARCHAR(120) NOT NULL,
	description VARCHAR(255) NOT NULL DEFAULT '',
	default_title VARCHAR(120) NOT NULL DEFAULT '',
	content_md TEXT NOT NULL DEFAULT '',
	sort INTEGER NOT NULL DEFAULT 0,
	is_builtin BOOLEAN NOT NULL DEFAULT FALSE,
	is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
	created_by_user_id VARCHAR(26) NULL REFERENCES users(user_id) ON DELETE SET NULL,
	updated_by_user_id VARCHAR(26) NULL REFERENCES users(user_id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	CONSTRAINT chk_document_templates_template_id_non_empty CHECK (BTRIM(template_id) <> ''),
	CONSTRAINT chk_document_templates_scene_key_non_empty CHECK (BTRIM(scene_key) <> '')
);

CREATE INDEX IF NOT EXISTS idx_document_templates_scene_enabled_sort_updated_at
	ON document_templates(scene_key, is_enabled, sort, updated_at);
CREATE INDEX IF NOT EXISTS idx_document_templates_enabled_updated_at
	ON document_templates(is_enabled, updated_at);
