ALTER TABLE spaces
	ADD COLUMN category VARCHAR(120) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_spaces_category ON spaces(category);
