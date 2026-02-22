CREATE TABLE IF NOT EXISTS space_categories (
	id BIGINT AUTO_INCREMENT PRIMARY KEY,
	category_id VARCHAR(26) NOT NULL UNIQUE,
	name VARCHAR(120) NOT NULL UNIQUE,
	is_default TINYINT(1) NOT NULL DEFAULT 0,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO space_categories (
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
) ON DUPLICATE KEY UPDATE
	name = VALUES(name),
	is_default = VALUES(is_default),
	updated_at = CURRENT_TIMESTAMP;

ALTER TABLE spaces
	ADD COLUMN category_id VARCHAR(26) NOT NULL DEFAULT '01jmf4v2x7m7f1m6qv5kh0t2mn' AFTER category;

UPDATE spaces
SET category = '未分类'
WHERE TRIM(COALESCE(category, '')) = '';

CREATE INDEX idx_spaces_category_id ON spaces(category_id);
CREATE INDEX idx_space_categories_is_default ON space_categories(is_default);

ALTER TABLE spaces
	ADD CONSTRAINT fk_spaces_category_id
	FOREIGN KEY (category_id) REFERENCES space_categories(category_id)
	ON UPDATE RESTRICT
	ON DELETE RESTRICT;
