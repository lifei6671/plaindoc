ALTER TABLE spaces
	ADD COLUMN visibility TEXT NOT NULL DEFAULT 'member' CHECK (visibility IN ('public', 'authenticated', 'member'));

ALTER TABLE documents
	ADD COLUMN visibility TEXT NOT NULL DEFAULT 'member' CHECK (visibility IN ('public', 'authenticated', 'member'));

CREATE INDEX IF NOT EXISTS idx_spaces_visibility ON spaces(visibility);
CREATE INDEX IF NOT EXISTS idx_documents_visibility ON documents(visibility);
