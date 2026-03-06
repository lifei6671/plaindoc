DROP TABLE IF EXISTS document_file_revisions;

DROP INDEX IF EXISTS idx_documents_source_blob_id;
DROP INDEX IF EXISTS idx_documents_format;

ALTER TABLE documents DROP CONSTRAINT IF EXISTS fk_documents_source_blob_id;
ALTER TABLE documents DROP CONSTRAINT IF EXISTS chk_documents_content_version;
ALTER TABLE documents DROP CONSTRAINT IF EXISTS chk_documents_format;

ALTER TABLE documents DROP COLUMN IF EXISTS content_version;
ALTER TABLE documents DROP COLUMN IF EXISTS source_mime_type;
ALTER TABLE documents DROP COLUMN IF EXISTS source_file_name;
ALTER TABLE documents DROP COLUMN IF EXISTS source_blob_id;
ALTER TABLE documents DROP COLUMN IF EXISTS format;
