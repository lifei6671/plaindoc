CREATE TABLE IF NOT EXISTS document_template_scenes (
	id BIGSERIAL PRIMARY KEY,
	scene_key VARCHAR(64) NOT NULL UNIQUE,
	scene_name VARCHAR(80) NOT NULL,
	description VARCHAR(255) NOT NULL DEFAULT '',
	sort INTEGER NOT NULL DEFAULT 0,
	is_builtin BOOLEAN NOT NULL DEFAULT FALSE,
	created_by_user_id VARCHAR(26) NULL REFERENCES users(user_id) ON DELETE SET NULL,
	updated_by_user_id VARCHAR(26) NULL REFERENCES users(user_id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	CONSTRAINT chk_document_template_scenes_scene_key_non_empty CHECK (BTRIM(scene_key) <> '')
);

CREATE INDEX IF NOT EXISTS idx_document_template_scenes_sort_updated_at
	ON document_template_scenes(sort, updated_at);

INSERT INTO document_template_scenes (
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
	src.scene_key,
	src.scene_name,
	'',
	0,
	FALSE,
	src.created_by_user_id,
	src.updated_by_user_id,
	src.created_at,
	src.updated_at
FROM (
	SELECT DISTINCT ON (t.scene_key)
		t.scene_key,
		t.scene_name,
		t.created_by_user_id,
		t.updated_by_user_id,
		t.created_at,
		t.updated_at
	FROM document_templates AS t
	WHERE BTRIM(t.scene_key) <> ''
	ORDER BY t.scene_key ASC, t.updated_at DESC, t.id DESC
) AS src
ON CONFLICT (scene_key) DO NOTHING;

ALTER TABLE document_templates DROP COLUMN IF EXISTS scene_name;
