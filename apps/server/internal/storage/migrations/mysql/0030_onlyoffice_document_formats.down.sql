DROP TABLE IF EXISTS document_file_revisions;

ALTER TABLE documents
	DROP FOREIGN KEY fk_documents_source_blob_id,
	DROP INDEX idx_documents_source_blob_id,
	DROP INDEX idx_documents_format,
	DROP CHECK ck_documents_content_version,
	DROP CHECK ck_documents_format,
	DROP COLUMN content_version,
	DROP COLUMN source_mime_type,
	DROP COLUMN source_file_name,
	DROP COLUMN source_blob_id,
	DROP COLUMN format;
