DROP INDEX IF EXISTS idx_document_image_assets_blob_id;

ALTER TABLE document_image_assets
	DROP CONSTRAINT IF EXISTS fk_document_image_assets_blob_id;

ALTER TABLE document_image_assets
	DROP COLUMN IF EXISTS blob_id;
