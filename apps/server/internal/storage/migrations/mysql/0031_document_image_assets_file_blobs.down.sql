ALTER TABLE document_image_assets
	DROP FOREIGN KEY fk_document_image_assets_blob_id,
	DROP KEY idx_document_image_assets_blob_id,
	DROP COLUMN blob_id;
