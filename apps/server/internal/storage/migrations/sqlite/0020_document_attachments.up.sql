CREATE TABLE IF NOT EXISTS document_attachments (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	attachment_id TEXT NOT NULL UNIQUE,
	document_id TEXT NOT NULL,
	space_id TEXT NOT NULL,
	storage_provider TEXT NOT NULL DEFAULT 'local',
	file_name TEXT NOT NULL,
	object_key TEXT NOT NULL DEFAULT '',
	object_url TEXT NOT NULL DEFAULT '',
	mime_type TEXT NOT NULL DEFAULT 'application/octet-stream',
	size_bytes INTEGER NOT NULL DEFAULT 0,
	preview_kind TEXT NOT NULL DEFAULT 'none',
	status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'deleted')),
	deleted_at TEXT NULL,
	created_by_user_id TEXT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (document_id) REFERENCES documents(document_id) ON DELETE CASCADE,
	FOREIGN KEY (space_id) REFERENCES spaces(space_id) ON DELETE CASCADE,
	FOREIGN KEY (created_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_document_attachments_document_id
	ON document_attachments(document_id);

CREATE INDEX IF NOT EXISTS idx_document_attachments_space_id
	ON document_attachments(space_id);

CREATE INDEX IF NOT EXISTS idx_document_attachments_status
	ON document_attachments(status);

CREATE INDEX IF NOT EXISTS idx_document_attachments_created_at
	ON document_attachments(created_at);
