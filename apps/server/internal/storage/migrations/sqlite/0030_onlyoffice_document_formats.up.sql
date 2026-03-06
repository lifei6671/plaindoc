ALTER TABLE documents
	ADD COLUMN format TEXT NOT NULL DEFAULT 'markdown' CHECK (format IN ('markdown', 'docx', 'xlsx'));

ALTER TABLE documents
	ADD COLUMN source_blob_id TEXT NULL REFERENCES file_blobs(blob_id) ON DELETE SET NULL;

ALTER TABLE documents
	ADD COLUMN source_file_name TEXT NULL;

ALTER TABLE documents
	ADD COLUMN source_mime_type TEXT NULL;

ALTER TABLE documents
	ADD COLUMN content_version INTEGER NOT NULL DEFAULT 1 CHECK (content_version > 0);

UPDATE documents
SET
	format = 'markdown',
	content_version = CASE
		WHEN version IS NOT NULL AND version > 0 THEN version
		ELSE 1
	END
WHERE format IS NULL
	OR trim(format) = ''
	OR content_version IS NULL
	OR content_version <= 0;

CREATE INDEX IF NOT EXISTS idx_documents_format
	ON documents(format);

CREATE INDEX IF NOT EXISTS idx_documents_source_blob_id
	ON documents(source_blob_id);

CREATE TABLE IF NOT EXISTS document_file_revisions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	document_file_revision_id TEXT NOT NULL UNIQUE,
	document_id TEXT NOT NULL,
	blob_id TEXT NOT NULL,
	file_name TEXT NOT NULL,
	mime_type TEXT NOT NULL,
	version INTEGER NOT NULL,
	base_version INTEGER NOT NULL,
	editor_user_id TEXT NULL,
	source TEXT NOT NULL CHECK (source IN ('local', 'remote')),
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(document_id, version),
	FOREIGN KEY (document_id) REFERENCES documents(document_id) ON DELETE CASCADE,
	FOREIGN KEY (blob_id) REFERENCES file_blobs(blob_id) ON DELETE RESTRICT,
	FOREIGN KEY (editor_user_id) REFERENCES users(user_id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_document_file_revisions_document_id
	ON document_file_revisions(document_id);

CREATE INDEX IF NOT EXISTS idx_document_file_revisions_blob_id
	ON document_file_revisions(blob_id);

CREATE INDEX IF NOT EXISTS idx_document_file_revisions_created_at
	ON document_file_revisions(created_at);
