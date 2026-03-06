DROP TABLE IF EXISTS document_file_revisions;

DROP INDEX IF EXISTS idx_documents_source_blob_id;
DROP INDEX IF EXISTS idx_documents_format;

ALTER TABLE documents DROP COLUMN content_version;
ALTER TABLE documents DROP COLUMN source_mime_type;
ALTER TABLE documents DROP COLUMN source_file_name;
ALTER TABLE documents DROP COLUMN source_blob_id;
ALTER TABLE documents DROP COLUMN format;
