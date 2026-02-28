CREATE TABLE IF NOT EXISTS user_sessions (
	id BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '主键ID',
	session_id VARCHAR(26) NOT NULL COMMENT '会话业务ID（ULID）',
	user_id VARCHAR(26) NOT NULL COMMENT '用户业务ID',
	refresh_token_hash VARCHAR(64) NOT NULL COMMENT 'refresh token 哈希',
	expires_at DATETIME(3) NOT NULL COMMENT '会话过期时间',
	revoked_at DATETIME(3) NULL COMMENT '会话吊销时间',
	replaced_by_session_id VARCHAR(26) NULL COMMENT '被旋转后替换的新会话ID',
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
	UNIQUE KEY uq_user_sessions_session_id (session_id),
	UNIQUE KEY uq_user_sessions_refresh_token_hash (refresh_token_hash),
	KEY idx_user_sessions_user_id (user_id),
	KEY idx_user_sessions_expires_at (expires_at),
	CONSTRAINT fk_user_sessions_user_id FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
	CONSTRAINT fk_user_sessions_replaced_by_session_id FOREIGN KEY (replaced_by_session_id) REFERENCES user_sessions(session_id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户会话表：refresh token 会话状态与轮换链';
