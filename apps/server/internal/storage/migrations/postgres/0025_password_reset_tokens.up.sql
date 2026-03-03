CREATE TABLE IF NOT EXISTS password_reset_tokens (
	id BIGSERIAL PRIMARY KEY,
	token_id VARCHAR(26) NOT NULL,
	token_secret_hash VARCHAR(128) NOT NULL,
	user_id VARCHAR(26) NOT NULL,
	source VARCHAR(32) NOT NULL DEFAULT 'self_service',
	requested_by_user_id VARCHAR(26) NULL,
	request_ip_hash VARCHAR(128) NOT NULL DEFAULT '',
	expires_at TIMESTAMPTZ NOT NULL,
	consumed_at TIMESTAMPTZ NULL,
	invalidated_at TIMESTAMPTZ NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	CONSTRAINT uk_password_reset_tokens_token_id UNIQUE (token_id),
	CONSTRAINT chk_password_reset_tokens_source CHECK (source IN ('self_service', 'admin_initiated')),
	CONSTRAINT fk_password_reset_tokens_user_id FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
	CONSTRAINT fk_password_reset_tokens_requested_by_user_id FOREIGN KEY (requested_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user_active
	ON password_reset_tokens(user_id, consumed_at, invalidated_at, expires_at);

CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_expires_at
	ON password_reset_tokens(expires_at);

CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_request_ip_created
	ON password_reset_tokens(request_ip_hash, created_at);
