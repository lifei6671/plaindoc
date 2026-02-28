CREATE TABLE IF NOT EXISTS user_sessions (
	id BIGSERIAL PRIMARY KEY,
	session_id VARCHAR(26) NOT NULL UNIQUE,
	user_id VARCHAR(26) NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
	refresh_token_hash VARCHAR(64) NOT NULL UNIQUE,
	expires_at TIMESTAMPTZ NOT NULL,
	revoked_at TIMESTAMPTZ NULL,
	replaced_by_session_id VARCHAR(26) NULL REFERENCES user_sessions(session_id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

COMMENT ON TABLE user_sessions IS '用户会话表：refresh token 会话状态与轮换链';
COMMENT ON COLUMN user_sessions.id IS '主键ID';
COMMENT ON COLUMN user_sessions.session_id IS '会话业务ID（ULID）';
COMMENT ON COLUMN user_sessions.user_id IS '用户业务ID';
COMMENT ON COLUMN user_sessions.refresh_token_hash IS 'refresh token 哈希';
COMMENT ON COLUMN user_sessions.expires_at IS '会话过期时间';
COMMENT ON COLUMN user_sessions.revoked_at IS '会话吊销时间';
COMMENT ON COLUMN user_sessions.replaced_by_session_id IS '被旋转后替换的新会话ID';
COMMENT ON COLUMN user_sessions.created_at IS '创建时间';
COMMENT ON COLUMN user_sessions.updated_at IS '更新时间';

CREATE INDEX IF NOT EXISTS idx_user_sessions_user_id ON user_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_user_sessions_expires_at ON user_sessions(expires_at);
