ALTER TABLE spaces
	ADD COLUMN category TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_spaces_category ON spaces(category);
