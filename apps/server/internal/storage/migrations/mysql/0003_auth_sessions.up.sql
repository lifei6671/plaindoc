CREATE TABLE IF NOT EXISTS user_sessions (
	id BIGINT PRIMARY KEY AUTO_INCREMENT,
	session_id VARCHAR(26) NOT NULL,
	user_id VARCHAR(26) NOT NULL,
	refresh_token_hash VARCHAR(64) NOT NULL,
	expires_at DATETIME(3) NOT NULL,
	revoked_at DATETIME(3) NULL,
	replaced_by_session_id VARCHAR(26) NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	UNIQUE KEY uq_user_sessions_session_id (session_id),
	UNIQUE KEY uq_user_sessions_refresh_token_hash (refresh_token_hash),
	KEY idx_user_sessions_user_id (user_id),
	KEY idx_user_sessions_expires_at (expires_at),
	CONSTRAINT fk_user_sessions_user_id FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
	CONSTRAINT fk_user_sessions_replaced_by_session_id FOREIGN KEY (replaced_by_session_id) REFERENCES user_sessions(session_id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
