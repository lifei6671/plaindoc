CREATE TABLE IF NOT EXISTS file_blobs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	blob_id TEXT NOT NULL UNIQUE,
	storage_provider TEXT NOT NULL DEFAULT 'local',
	object_key TEXT NOT NULL DEFAULT '',
	object_url TEXT NOT NULL DEFAULT '',
	mime_type TEXT NOT NULL DEFAULT 'application/octet-stream',
	size_bytes INTEGER NOT NULL DEFAULT 0,
	content_hash_algo TEXT NOT NULL DEFAULT 'sha256',
	content_hash TEXT NOT NULL DEFAULT '',
	deleted_at TEXT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_file_blobs_hash
	ON file_blobs(storage_provider, content_hash_algo, content_hash, size_bytes);

CREATE INDEX IF NOT EXISTS idx_file_blobs_created_at
	ON file_blobs(created_at);

CREATE TABLE IF NOT EXISTS document_attachments (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	attachment_id TEXT NOT NULL UNIQUE,
	blob_id TEXT NOT NULL,
	document_id TEXT NOT NULL,
	space_id TEXT NOT NULL,
	storage_provider TEXT NOT NULL DEFAULT 'local',
	file_name TEXT NOT NULL,
	object_key TEXT NOT NULL DEFAULT '',
	object_url TEXT NOT NULL DEFAULT '',
	mime_type TEXT NOT NULL DEFAULT 'application/octet-stream',
	size_bytes INTEGER NOT NULL DEFAULT 0,
	content_hash_algo TEXT NOT NULL DEFAULT 'sha256',
	content_hash TEXT NOT NULL DEFAULT '',
	preview_kind TEXT NOT NULL DEFAULT 'none',
	status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'deleted')),
	deleted_at TEXT NULL,
	created_by_user_id TEXT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (blob_id) REFERENCES file_blobs(blob_id) ON DELETE RESTRICT,
	FOREIGN KEY (document_id) REFERENCES documents(document_id) ON DELETE CASCADE,
	FOREIGN KEY (space_id) REFERENCES spaces(space_id) ON DELETE CASCADE,
	FOREIGN KEY (created_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_document_attachments_blob_id
	ON document_attachments(blob_id);

CREATE INDEX IF NOT EXISTS idx_document_attachments_document_id
	ON document_attachments(document_id);

CREATE INDEX IF NOT EXISTS idx_document_attachments_space_id
	ON document_attachments(space_id);

CREATE INDEX IF NOT EXISTS idx_document_attachments_status
	ON document_attachments(status);

CREATE INDEX IF NOT EXISTS idx_document_attachments_created_at
	ON document_attachments(created_at);

CREATE INDEX IF NOT EXISTS idx_document_attachments_hash
	ON document_attachments(storage_provider, content_hash_algo, content_hash, size_bytes);
