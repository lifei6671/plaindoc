CREATE TABLE IF NOT EXISTS document_shares (
	id BIGSERIAL PRIMARY KEY,
	share_id VARCHAR(26) NOT NULL UNIQUE,
	document_id VARCHAR(26) NOT NULL UNIQUE REFERENCES documents(document_id) ON DELETE CASCADE,
	space_id VARCHAR(26) NOT NULL REFERENCES spaces(space_id) ON DELETE CASCADE,
	mode VARCHAR(16) NOT NULL DEFAULT 'public',
	password_hash TEXT NULL,
	password_hint VARCHAR(120) NOT NULL DEFAULT '',
	expires_at TIMESTAMPTZ NULL,
	disabled_at TIMESTAMPTZ NULL,
	access_version INTEGER NOT NULL DEFAULT 1,
	created_by_user_id VARCHAR(26) NULL REFERENCES users(user_id) ON DELETE SET NULL,
	updated_by_user_id VARCHAR(26) NULL REFERENCES users(user_id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	CONSTRAINT chk_document_shares_mode CHECK (mode IN ('public', 'password')),
	CONSTRAINT chk_document_shares_access_version CHECK (access_version > 0)
);

CREATE INDEX IF NOT EXISTS idx_document_shares_space_disabled_expires
	ON document_shares(space_id, disabled_at, expires_at);
CREATE INDEX IF NOT EXISTS idx_document_shares_mode
	ON document_shares(mode);
CREATE INDEX IF NOT EXISTS idx_document_shares_created_by_user_id
	ON document_shares(created_by_user_id);
