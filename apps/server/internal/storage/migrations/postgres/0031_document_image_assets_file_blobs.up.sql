ALTER TABLE document_image_assets
	ADD COLUMN IF NOT EXISTS blob_id VARCHAR(26) NULL;

DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1
		FROM pg_constraint
		WHERE conname = 'fk_document_image_assets_blob_id'
	) THEN
		ALTER TABLE document_image_assets
			ADD CONSTRAINT fk_document_image_assets_blob_id
			FOREIGN KEY (blob_id) REFERENCES file_blobs(blob_id) ON DELETE SET NULL;
	END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_document_image_assets_blob_id
	ON document_image_assets(blob_id);
