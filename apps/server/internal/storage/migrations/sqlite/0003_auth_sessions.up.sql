CREATE TABLE IF NOT EXISTS user_sessions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	session_id VARCHAR(26) NOT NULL UNIQUE,
	user_id VARCHAR(26) NOT NULL,
	refresh_token_hash VARCHAR(64) NOT NULL UNIQUE,
	expires_at TIMESTAMP NOT NULL,
	revoked_at TIMESTAMP NULL,
	replaced_by_session_id VARCHAR(26) NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
	FOREIGN KEY (replaced_by_session_id) REFERENCES user_sessions(session_id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_user_sessions_user_id ON user_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_user_sessions_expires_at ON user_sessions(expires_at);
