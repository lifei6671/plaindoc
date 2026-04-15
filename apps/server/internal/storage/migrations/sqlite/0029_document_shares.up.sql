CREATE TABLE IF NOT EXISTS document_shares (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	share_id TEXT NOT NULL UNIQUE,
	document_id TEXT NOT NULL UNIQUE,
	space_id TEXT NOT NULL,
	mode TEXT NOT NULL DEFAULT 'public' CHECK (mode IN ('public', 'password')),
	password_hash TEXT NULL,
	password_hint TEXT NOT NULL DEFAULT '',
	expires_at TIMESTAMP NULL,
	disabled_at TIMESTAMP NULL,
	access_version INTEGER NOT NULL DEFAULT 1 CHECK (access_version > 0),
	created_by_user_id TEXT NULL,
	updated_by_user_id TEXT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (document_id) REFERENCES documents(document_id) ON DELETE CASCADE,
	FOREIGN KEY (space_id) REFERENCES spaces(space_id) ON DELETE CASCADE,
	FOREIGN KEY (created_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL,
	FOREIGN KEY (updated_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL,
	CHECK (length(trim(share_id)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_document_shares_space_disabled_expires
	ON document_shares(space_id, disabled_at, expires_at);
CREATE INDEX IF NOT EXISTS idx_document_shares_mode
	ON document_shares(mode);
CREATE INDEX IF NOT EXISTS idx_document_shares_created_by_user_id
	ON document_shares(created_by_user_id);
