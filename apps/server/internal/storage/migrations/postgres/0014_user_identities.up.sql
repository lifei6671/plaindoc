CREATE TABLE IF NOT EXISTS user_identities (
	id BIGSERIAL PRIMARY KEY,
	user_id VARCHAR(26) NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
	provider_type VARCHAR(32) NOT NULL,
	provider_id VARCHAR(64) NOT NULL,
	external_id VARCHAR(255) NOT NULL,
	login_name VARCHAR(320) NOT NULL DEFAULT '',
	last_login_at TIMESTAMP NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(provider_id, external_id)
);

CREATE INDEX IF NOT EXISTS idx_user_identities_user_id ON user_identities(user_id);
CREATE INDEX IF NOT EXISTS idx_user_identities_provider_type ON user_identities(provider_type);

