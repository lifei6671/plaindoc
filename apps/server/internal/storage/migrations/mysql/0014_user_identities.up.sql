CREATE TABLE IF NOT EXISTS user_identities (
	id BIGINT AUTO_INCREMENT PRIMARY KEY,
	user_id VARCHAR(26) NOT NULL,
	provider_type VARCHAR(32) NOT NULL,
	provider_id VARCHAR(64) NOT NULL,
	external_id VARCHAR(255) NOT NULL,
	login_name VARCHAR(320) NOT NULL DEFAULT '',
	last_login_at DATETIME NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	UNIQUE KEY uk_user_identities_provider_external (provider_id, external_id),
	KEY idx_user_identities_user_id (user_id),
	KEY idx_user_identities_provider_type (provider_type),
	CONSTRAINT fk_user_identities_user_id FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

