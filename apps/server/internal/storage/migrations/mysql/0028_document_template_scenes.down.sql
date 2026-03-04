ALTER TABLE document_templates ADD COLUMN scene_name VARCHAR(80) NOT NULL DEFAULT '' AFTER scene_key;

UPDATE document_templates AS t
LEFT JOIN document_template_scenes AS s
	ON s.scene_key = t.scene_key
SET t.scene_name = COALESCE(s.scene_name, '');

DROP TABLE IF EXISTS document_template_scenes;
