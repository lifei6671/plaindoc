CREATE TABLE IF NOT EXISTS space_categories (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	category_id TEXT NOT NULL UNIQUE,
	name TEXT NOT NULL UNIQUE,
	is_default INTEGER NOT NULL DEFAULT 0 CHECK (is_default IN (0, 1)),
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT OR IGNORE INTO space_categories (
	category_id,
	name,
	is_default,
	created_at,
	updated_at
) VALUES (
	'01jmf4v2x7m7f1m6qv5kh0t2mn',
	'未分类',
	1,
	CURRENT_TIMESTAMP,
	CURRENT_TIMESTAMP
);

ALTER TABLE spaces
	ADD COLUMN category_id TEXT NOT NULL DEFAULT '01jmf4v2x7m7f1m6qv5kh0t2mn';

UPDATE spaces
SET category = '未分类'
WHERE TRIM(COALESCE(category, '')) = '';

CREATE INDEX IF NOT EXISTS idx_spaces_category_id ON spaces(category_id);
CREATE INDEX IF NOT EXISTS idx_space_categories_is_default ON space_categories(is_default);
