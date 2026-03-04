CREATE TABLE IF NOT EXISTS document_template_scenes (
	id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
	scene_key VARCHAR(64) NOT NULL,
	scene_name VARCHAR(80) NOT NULL,
	description VARCHAR(255) NOT NULL DEFAULT '',
	sort INT NOT NULL DEFAULT 0,
	is_builtin TINYINT(1) NOT NULL DEFAULT 0,
	created_by_user_id VARCHAR(26) NULL,
	updated_by_user_id VARCHAR(26) NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	PRIMARY KEY (id),
	UNIQUE KEY uk_document_template_scenes_scene_key (scene_key),
	KEY idx_document_template_scenes_sort_updated_at (sort, updated_at),
	CONSTRAINT fk_document_template_scenes_created_by_user_id FOREIGN KEY (created_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL,
	CONSTRAINT fk_document_template_scenes_updated_by_user_id FOREIGN KEY (updated_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL
);

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
	t.scene_key,
	MAX(t.scene_name) AS scene_name,
	'' AS description,
	0 AS sort,
	0 AS is_builtin,
	MAX(t.created_by_user_id) AS created_by_user_id,
	MAX(t.updated_by_user_id) AS updated_by_user_id,
	MIN(t.created_at) AS created_at,
	MAX(t.updated_at) AS updated_at
FROM document_templates AS t
WHERE TRIM(t.scene_key) <> ''
GROUP BY t.scene_key
ON DUPLICATE KEY UPDATE
	scene_name = VALUES(scene_name),
	updated_at = GREATEST(document_template_scenes.updated_at, VALUES(updated_at));

ALTER TABLE document_templates DROP COLUMN scene_name;
