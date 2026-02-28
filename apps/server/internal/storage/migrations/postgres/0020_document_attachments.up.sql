CREATE TABLE IF NOT EXISTS document_attachments (
	id BIGSERIAL PRIMARY KEY,
	attachment_id VARCHAR(26) NOT NULL UNIQUE,
	document_id VARCHAR(26) NOT NULL,
	space_id VARCHAR(26) NOT NULL,
	storage_provider VARCHAR(32) NOT NULL DEFAULT 'local',
	file_name VARCHAR(255) NOT NULL,
	object_key VARCHAR(512) NOT NULL DEFAULT '',
	object_url TEXT NOT NULL DEFAULT '',
	mime_type VARCHAR(128) NOT NULL DEFAULT 'application/octet-stream',
	size_bytes BIGINT NOT NULL DEFAULT 0,
	preview_kind VARCHAR(32) NOT NULL DEFAULT 'none',
	status VARCHAR(16) NOT NULL DEFAULT 'active',
	deleted_at TIMESTAMPTZ NULL,
	created_by_user_id VARCHAR(26) NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	CONSTRAINT chk_document_attachments_status CHECK (status IN ('active', 'deleted')),
	CONSTRAINT fk_document_attachments_document_id FOREIGN KEY (document_id) REFERENCES documents(document_id) ON DELETE CASCADE,
	CONSTRAINT fk_document_attachments_space_id FOREIGN KEY (space_id) REFERENCES spaces(space_id) ON DELETE CASCADE,
	CONSTRAINT fk_document_attachments_created_by_user_id FOREIGN KEY (created_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_document_attachments_document_id
	ON document_attachments(document_id);

CREATE INDEX IF NOT EXISTS idx_document_attachments_space_id
	ON document_attachments(space_id);

CREATE INDEX IF NOT EXISTS idx_document_attachments_status
	ON document_attachments(status);

CREATE INDEX IF NOT EXISTS idx_document_attachments_created_at
	ON document_attachments(created_at);
