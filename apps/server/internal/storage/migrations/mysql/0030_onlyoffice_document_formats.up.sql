ALTER TABLE documents
	ADD COLUMN format VARCHAR(16) NOT NULL DEFAULT 'markdown' COMMENT '文档正文格式：markdown/docx/xlsx' AFTER title,
	ADD COLUMN source_blob_id VARCHAR(26) NULL COMMENT 'Office 正文当前 blob_id' AFTER version,
	ADD COLUMN source_file_name VARCHAR(255) NULL COMMENT 'Office 正文文件名' AFTER source_blob_id,
	ADD COLUMN source_mime_type VARCHAR(255) NULL COMMENT 'Office 正文 MIME 类型' AFTER source_file_name,
	ADD COLUMN content_version INT NOT NULL DEFAULT 1 COMMENT '正文版本号：Markdown/Office 共用的内容版本' AFTER source_mime_type,
	ADD CONSTRAINT ck_documents_format CHECK (format IN ('markdown', 'docx', 'xlsx')),
	ADD CONSTRAINT ck_documents_content_version CHECK (content_version > 0),
	ADD KEY idx_documents_format (format),
	ADD KEY idx_documents_source_blob_id (source_blob_id),
	ADD CONSTRAINT fk_documents_source_blob_id FOREIGN KEY (source_blob_id) REFERENCES file_blobs(blob_id) ON DELETE SET NULL;

UPDATE documents
SET
	format = 'markdown',
	content_version = CASE
		WHEN version IS NOT NULL AND version > 0 THEN version
		ELSE 1
	END
WHERE TRIM(IFNULL(format, '')) = ''
	OR content_version IS NULL
	OR content_version <= 0;

CREATE TABLE IF NOT EXISTS document_file_revisions (
	id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
	document_file_revision_id VARCHAR(26) NOT NULL,
	document_id VARCHAR(26) NOT NULL,
	blob_id VARCHAR(26) NOT NULL,
	file_name VARCHAR(255) NOT NULL,
	mime_type VARCHAR(255) NOT NULL,
	version INT NOT NULL,
	base_version INT NOT NULL,
	editor_user_id VARCHAR(26) NULL,
	source VARCHAR(16) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	PRIMARY KEY (id),
	UNIQUE KEY uk_document_file_revisions_id (document_file_revision_id),
	UNIQUE KEY uk_document_file_revisions_document_version (document_id, version),
	KEY idx_document_file_revisions_document_id (document_id),
	KEY idx_document_file_revisions_blob_id (blob_id),
	KEY idx_document_file_revisions_created_at (created_at),
	CONSTRAINT ck_document_file_revisions_source CHECK (source IN ('local', 'remote')),
	CONSTRAINT fk_document_file_revisions_document_id FOREIGN KEY (document_id) REFERENCES documents(document_id) ON DELETE CASCADE,
	CONSTRAINT fk_document_file_revisions_blob_id FOREIGN KEY (blob_id) REFERENCES file_blobs(blob_id) ON DELETE RESTRICT,
	CONSTRAINT fk_document_file_revisions_editor_user_id FOREIGN KEY (editor_user_id) REFERENCES users(user_id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Office 文档文件修订历史表：保存二进制文件版本快照';
