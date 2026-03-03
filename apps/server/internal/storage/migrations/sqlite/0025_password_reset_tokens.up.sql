CREATE TABLE IF NOT EXISTS password_reset_tokens (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	token_id TEXT NOT NULL UNIQUE,
	token_secret_hash TEXT NOT NULL,
	user_id TEXT NOT NULL,
	source TEXT NOT NULL DEFAULT 'self_service' CHECK (source IN ('self_service', 'admin_initiated')),
	requested_by_user_id TEXT NULL,
	request_ip_hash TEXT NOT NULL DEFAULT '',
	expires_at TEXT NOT NULL,
	consumed_at TEXT NULL,
	invalidated_at TEXT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
	FOREIGN KEY (requested_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user_active
	ON password_reset_tokens(user_id, consumed_at, invalidated_at, expires_at);

CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_expires_at
	ON password_reset_tokens(expires_at);

CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_request_ip_created
	ON password_reset_tokens(request_ip_hash, created_at);
