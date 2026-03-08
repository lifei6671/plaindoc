ALTER TABLE document_image_assets
	ADD COLUMN blob_id VARCHAR(26) NULL COMMENT '关联的 file_blobs.blob_id' AFTER space_id,
	ADD KEY idx_document_image_assets_blob_id (blob_id),
	ADD CONSTRAINT fk_document_image_assets_blob_id
		FOREIGN KEY (blob_id) REFERENCES file_blobs(blob_id) ON DELETE SET NULL;
