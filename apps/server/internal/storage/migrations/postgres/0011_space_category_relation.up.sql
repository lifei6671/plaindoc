CREATE TABLE IF NOT EXISTS space_categories (
	id BIGSERIAL PRIMARY KEY,
	category_id VARCHAR(26) NOT NULL UNIQUE,
	name VARCHAR(120) NOT NULL UNIQUE,
	is_default BOOLEAN NOT NULL DEFAULT FALSE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO space_categories (
	category_id,
	name,
	is_default,
	created_at,
	updated_at
) VALUES (
	'01jmf4v2x7m7f1m6qv5kh0t2mn',
	'未分类',
	TRUE,
	CURRENT_TIMESTAMP,
	CURRENT_TIMESTAMP
) ON CONFLICT (category_id) DO UPDATE
SET
	name = EXCLUDED.name,
	is_default = EXCLUDED.is_default,
	updated_at = CURRENT_TIMESTAMP;

ALTER TABLE spaces
	ADD COLUMN category_id VARCHAR(26) NOT NULL DEFAULT '01jmf4v2x7m7f1m6qv5kh0t2mn';

UPDATE spaces
SET category = '未分类'
WHERE TRIM(COALESCE(category, '')) = '';

CREATE INDEX IF NOT EXISTS idx_spaces_category_id ON spaces(category_id);
CREATE INDEX IF NOT EXISTS idx_space_categories_is_default ON space_categories(is_default);

ALTER TABLE spaces
	ADD CONSTRAINT fk_spaces_category_id
	FOREIGN KEY (category_id) REFERENCES space_categories(category_id)
	ON UPDATE RESTRICT
	ON DELETE RESTRICT;
