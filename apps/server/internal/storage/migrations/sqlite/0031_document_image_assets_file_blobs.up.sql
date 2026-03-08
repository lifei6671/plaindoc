ALTER TABLE document_image_assets
	ADD COLUMN blob_id TEXT NULL REFERENCES file_blobs(blob_id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_document_image_assets_blob_id
	ON document_image_assets(blob_id);
