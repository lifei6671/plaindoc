CREATE TABLE IF NOT EXISTS document_image_assets (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	image_asset_id TEXT NOT NULL UNIQUE,
	document_id TEXT NOT NULL,
	space_id TEXT NOT NULL,
	storage_provider TEXT NOT NULL DEFAULT 'local',
	object_key TEXT NOT NULL DEFAULT '',
	object_url TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'pending_cleanup', 'deleted')),
	pending_cleanup_at TIMESTAMP NULL,
	deleted_at TIMESTAMP NULL,
	last_referenced_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (document_id) REFERENCES documents(document_id) ON DELETE CASCADE,
	FOREIGN KEY (space_id) REFERENCES spaces(space_id) ON DELETE CASCADE
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
