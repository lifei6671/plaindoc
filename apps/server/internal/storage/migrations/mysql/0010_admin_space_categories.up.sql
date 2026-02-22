ALTER TABLE spaces
	ADD COLUMN category VARCHAR(120) NOT NULL DEFAULT '' AFTER description;

CREATE INDEX idx_spaces_category ON spaces(category);
