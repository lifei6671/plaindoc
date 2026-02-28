CREATE TABLE IF NOT EXISTS auth_risk_states (
	id BIGSERIAL PRIMARY KEY,
	scene VARCHAR(32) NOT NULL,
	subject_type VARCHAR(32) NOT NULL,
	subject_hash VARCHAR(128) NOT NULL,
	window_started_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
	failed_count INTEGER NOT NULL DEFAULT 0 CHECK (failed_count >= 0),
	captcha_failed_count INTEGER NOT NULL DEFAULT 0 CHECK (captcha_failed_count >= 0),
	lock_until TIMESTAMPTZ NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(scene, subject_type, subject_hash)
);

COMMENT ON TABLE auth_risk_states IS '认证风控状态表：记录主体风险窗口计数与封禁状态';
COMMENT ON COLUMN auth_risk_states.id IS '主键ID';
COMMENT ON COLUMN auth_risk_states.scene IS '风控场景：login/register';
COMMENT ON COLUMN auth_risk_states.subject_type IS '风险主体类型，如 ip、identifier、ip_identifier、email、ip_email';
COMMENT ON COLUMN auth_risk_states.subject_hash IS '风险主体哈希值（HMAC-SHA256），不存储明文';
COMMENT ON COLUMN auth_risk_states.window_started_at IS '统计窗口起始时间';
COMMENT ON COLUMN auth_risk_states.attempt_count IS '窗口内总尝试次数（含成功与失败）';
COMMENT ON COLUMN auth_risk_states.failed_count IS '窗口内失败次数';
COMMENT ON COLUMN auth_risk_states.captcha_failed_count IS '窗口内验证码校验失败次数';
COMMENT ON COLUMN auth_risk_states.lock_until IS '临时封禁截止时间；为空表示未封禁';
COMMENT ON COLUMN auth_risk_states.created_at IS '创建时间';
COMMENT ON COLUMN auth_risk_states.updated_at IS '更新时间';

CREATE INDEX IF NOT EXISTS idx_auth_risk_states_lock_until
	ON auth_risk_states(lock_until);
CREATE INDEX IF NOT EXISTS idx_auth_risk_states_updated_at
	ON auth_risk_states(updated_at);

CREATE TABLE IF NOT EXISTS auth_captcha_challenges (
	id BIGSERIAL PRIMARY KEY,
	captcha_id VARCHAR(32) NOT NULL UNIQUE,
	scene VARCHAR(32) NOT NULL,
	subject_hash VARCHAR(128) NOT NULL,
	level INTEGER NOT NULL DEFAULT 4 CHECK (level BETWEEN 1 AND 32),
	answer_hash VARCHAR(128) NOT NULL,
	answer_salt VARCHAR(64) NOT NULL,
	issued_ip_hash VARCHAR(128) NOT NULL,
	expires_at TIMESTAMPTZ NOT NULL,
	consumed_at TIMESTAMPTZ NULL,
	failed_verify_count INTEGER NOT NULL DEFAULT 0 CHECK (failed_verify_count >= 0),
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

COMMENT ON TABLE auth_captcha_challenges IS '认证验证码会话表：记录题目、有效期和消费状态';
COMMENT ON COLUMN auth_captcha_challenges.id IS '主键ID';
COMMENT ON COLUMN auth_captcha_challenges.captcha_id IS '验证码会话ID（回传前端）';
COMMENT ON COLUMN auth_captcha_challenges.scene IS '验证码适用场景：login/register';
COMMENT ON COLUMN auth_captcha_challenges.subject_hash IS '风险主体哈希值（HMAC-SHA256）';
COMMENT ON COLUMN auth_captcha_challenges.level IS '验证码字符数量（位数），例如 4/5/6';
COMMENT ON COLUMN auth_captcha_challenges.answer_hash IS '验证码答案哈希值';
COMMENT ON COLUMN auth_captcha_challenges.answer_salt IS '验证码答案哈希盐';
COMMENT ON COLUMN auth_captcha_challenges.issued_ip_hash IS '发放验证码时的请求IP哈希值';
COMMENT ON COLUMN auth_captcha_challenges.expires_at IS '验证码过期时间';
COMMENT ON COLUMN auth_captcha_challenges.consumed_at IS '验证码消费时间；为空表示未消费';
COMMENT ON COLUMN auth_captcha_challenges.failed_verify_count IS '该会话下验证码校验失败次数';
COMMENT ON COLUMN auth_captcha_challenges.created_at IS '创建时间';
COMMENT ON COLUMN auth_captcha_challenges.updated_at IS '更新时间';

CREATE INDEX IF NOT EXISTS idx_auth_captcha_challenges_scene_subject
	ON auth_captcha_challenges(scene, subject_hash);
CREATE INDEX IF NOT EXISTS idx_auth_captcha_challenges_expires_at
	ON auth_captcha_challenges(expires_at);
CREATE INDEX IF NOT EXISTS idx_auth_captcha_challenges_consumed_at
	ON auth_captcha_challenges(consumed_at);
