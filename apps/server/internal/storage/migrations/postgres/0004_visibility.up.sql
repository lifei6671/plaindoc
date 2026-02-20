ALTER TABLE spaces
	ADD COLUMN visibility VARCHAR(32) NOT NULL DEFAULT 'member';

ALTER TABLE spaces
	ADD CONSTRAINT ck_spaces_visibility
	CHECK (visibility IN ('public', 'authenticated', 'member'));

ALTER TABLE documents
	ADD COLUMN visibility VARCHAR(32) NOT NULL DEFAULT 'member';

ALTER TABLE documents
	ADD CONSTRAINT ck_documents_visibility
	CHECK (visibility IN ('public', 'authenticated', 'member'));

CREATE INDEX IF NOT EXISTS idx_spaces_visibility ON spaces(visibility);
CREATE INDEX IF NOT EXISTS idx_documents_visibility ON documents(visibility);
