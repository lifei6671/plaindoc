CREATE TABLE IF NOT EXISTS document_image_assets (
	id BIGSERIAL PRIMARY KEY,
	image_asset_id VARCHAR(26) NOT NULL UNIQUE,
	document_id VARCHAR(26) NOT NULL,
	space_id VARCHAR(26) NOT NULL,
	storage_provider VARCHAR(32) NOT NULL DEFAULT 'local',
	object_key VARCHAR(512) NOT NULL DEFAULT '',
	object_url TEXT NOT NULL DEFAULT '',
	status VARCHAR(32) NOT NULL DEFAULT 'active',
	pending_cleanup_at TIMESTAMPTZ NULL,
	deleted_at TIMESTAMPTZ NULL,
	last_referenced_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	CONSTRAINT chk_document_image_assets_status CHECK (status IN ('active', 'pending_cleanup', 'deleted')),
	CONSTRAINT fk_document_image_assets_document_id FOREIGN KEY (document_id) REFERENCES documents(document_id) ON DELETE CASCADE,
	CONSTRAINT fk_document_image_assets_space_id FOREIGN KEY (space_id) REFERENCES spaces(space_id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_document_image_assets_doc_object
	ON document_image_assets(document_id, storage_provider, object_key);

CREATE INDEX IF NOT EXISTS idx_document_image_assets_pending
	ON document_image_assets(status, pending_cleanup_at);

CREATE INDEX IF NOT EXISTS idx_document_image_assets_object
	ON document_image_assets(storage_provider, object_key, status);

CREATE INDEX IF NOT EXISTS idx_document_image_assets_space_id
	ON document_image_assets(space_id);

CREATE INDEX IF NOT EXISTS idx_document_image_assets_created_at
	ON document_image_assets(created_at);
