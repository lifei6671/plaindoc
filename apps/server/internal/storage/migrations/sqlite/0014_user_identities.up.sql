CREATE TABLE IF NOT EXISTS user_identities (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id TEXT NOT NULL,
	provider_type TEXT NOT NULL,
	provider_id TEXT NOT NULL,
	external_id TEXT NOT NULL,
	login_name TEXT NOT NULL DEFAULT '',
	last_login_at TEXT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(provider_id, external_id),
	FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_user_identities_user_id ON user_identities(user_id);
CREATE INDEX IF NOT EXISTS idx_user_identities_provider_type ON user_identities(provider_type);

