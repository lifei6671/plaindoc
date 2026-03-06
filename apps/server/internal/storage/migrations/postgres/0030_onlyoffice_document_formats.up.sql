ALTER TABLE documents
	ADD COLUMN IF NOT EXISTS format VARCHAR(16) NOT NULL DEFAULT 'markdown';

ALTER TABLE documents
	ADD COLUMN IF NOT EXISTS source_blob_id VARCHAR(64) NULL;

ALTER TABLE documents
	ADD COLUMN IF NOT EXISTS source_file_name VARCHAR(255) NULL;

ALTER TABLE documents
	ADD COLUMN IF NOT EXISTS source_mime_type VARCHAR(255) NULL;

ALTER TABLE documents
	ADD COLUMN IF NOT EXISTS content_version INTEGER NOT NULL DEFAULT 1;

DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1
		FROM pg_constraint
		WHERE conname = 'chk_documents_format'
	) THEN
		ALTER TABLE documents
			ADD CONSTRAINT chk_documents_format
			CHECK (format IN ('markdown', 'docx', 'xlsx'));
	END IF;
END $$;

DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1
		FROM pg_constraint
		WHERE conname = 'chk_documents_content_version'
	) THEN
		ALTER TABLE documents
			ADD CONSTRAINT chk_documents_content_version
			CHECK (content_version > 0);
	END IF;
END $$;

DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1
		FROM pg_constraint
		WHERE conname = 'fk_documents_source_blob_id'
	) THEN
		ALTER TABLE documents
			ADD CONSTRAINT fk_documents_source_blob_id
			FOREIGN KEY (source_blob_id) REFERENCES file_blobs(blob_id) ON DELETE SET NULL;
	END IF;
END $$;

UPDATE documents
SET
	format = 'markdown',
	content_version = CASE
		WHEN version IS NOT NULL AND version > 0 THEN version
		ELSE 1
	END
WHERE trim(COALESCE(format, '')) = ''
	OR content_version IS NULL
	OR content_version <= 0;

CREATE INDEX IF NOT EXISTS idx_documents_format
	ON documents(format);

CREATE INDEX IF NOT EXISTS idx_documents_source_blob_id
	ON documents(source_blob_id);

CREATE TABLE IF NOT EXISTS document_file_revisions (
	id BIGSERIAL PRIMARY KEY,
	document_file_revision_id VARCHAR(26) NOT NULL UNIQUE,
	document_id VARCHAR(26) NOT NULL REFERENCES documents(document_id) ON DELETE CASCADE,
	blob_id VARCHAR(64) NOT NULL REFERENCES file_blobs(blob_id) ON DELETE RESTRICT,
	file_name VARCHAR(255) NOT NULL,
	mime_type VARCHAR(255) NOT NULL,
	version INTEGER NOT NULL,
	base_version INTEGER NOT NULL,
	editor_user_id VARCHAR(26) NULL REFERENCES users(user_id) ON DELETE SET NULL,
	source VARCHAR(16) NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	CONSTRAINT chk_document_file_revisions_source CHECK (source IN ('local', 'remote')),
	CONSTRAINT uk_document_file_revisions_document_version UNIQUE (document_id, version)
);

CREATE INDEX IF NOT EXISTS idx_document_file_revisions_document_id
	ON document_file_revisions(document_id);

CREATE INDEX IF NOT EXISTS idx_document_file_revisions_blob_id
	ON document_file_revisions(blob_id);

CREATE INDEX IF NOT EXISTS idx_document_file_revisions_created_at
	ON document_file_revisions(created_at);

COMMENT ON TABLE document_file_revisions IS 'Office 文档文件修订历史表：保存二进制文件版本快照';
COMMENT ON COLUMN documents.format IS '文档正文格式：markdown/docx/xlsx';
COMMENT ON COLUMN documents.source_blob_id IS 'Office 正文当前 blob_id';
COMMENT ON COLUMN documents.source_file_name IS 'Office 正文文件名';
COMMENT ON COLUMN documents.source_mime_type IS 'Office 正文 MIME 类型';
COMMENT ON COLUMN documents.content_version IS '正文版本号：Markdown/Office 共用的内容版本';
