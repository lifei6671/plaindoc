ALTER TABLE document_templates
	ADD COLUMN IF NOT EXISTS scene_name VARCHAR(80) NOT NULL DEFAULT '';

UPDATE document_templates AS t
SET scene_name = COALESCE(s.scene_name, '')
FROM document_template_scenes AS s
WHERE s.scene_key = t.scene_key;

DROP TABLE IF EXISTS document_template_scenes;
