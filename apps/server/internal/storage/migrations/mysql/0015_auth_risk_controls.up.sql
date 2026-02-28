CREATE TABLE IF NOT EXISTS auth_risk_states (
	id BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '主键ID',
	scene VARCHAR(32) NOT NULL COMMENT '风控场景：login/register',
	subject_type VARCHAR(32) NOT NULL COMMENT '风险主体类型，如 ip、identifier、ip_identifier、email、ip_email',
	subject_hash VARCHAR(128) NOT NULL COMMENT '风险主体哈希值（HMAC-SHA256），不存储明文',
	window_started_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '统计窗口起始时间',
	attempt_count INT NOT NULL DEFAULT 0 COMMENT '窗口内总尝试次数（含成功与失败）',
	failed_count INT NOT NULL DEFAULT 0 COMMENT '窗口内失败次数',
	captcha_failed_count INT NOT NULL DEFAULT 0 COMMENT '窗口内验证码校验失败次数',
	lock_until DATETIME(6) NULL COMMENT '临时封禁截止时间；为空表示未封禁',
	created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间',
	updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '更新时间',
	UNIQUE KEY uk_auth_risk_states_scene_subject (scene, subject_type, subject_hash),
	CONSTRAINT ck_auth_risk_states_attempt_count CHECK (attempt_count >= 0),
	CONSTRAINT ck_auth_risk_states_failed_count CHECK (failed_count >= 0),
	CONSTRAINT ck_auth_risk_states_captcha_failed_count CHECK (captcha_failed_count >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='认证风控状态表：记录主体风险窗口计数与封禁状态';

CREATE INDEX idx_auth_risk_states_lock_until ON auth_risk_states(lock_until);
CREATE INDEX idx_auth_risk_states_updated_at ON auth_risk_states(updated_at);

CREATE TABLE IF NOT EXISTS auth_captcha_challenges (
	id BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '主键ID',
	captcha_id VARCHAR(32) NOT NULL UNIQUE COMMENT '验证码挑战ID（回传前端）',
	scene VARCHAR(32) NOT NULL COMMENT '验证码适用场景：login/register',
	subject_hash VARCHAR(128) NOT NULL COMMENT '风险主体哈希值（HMAC-SHA256）',
	level INT NOT NULL DEFAULT 4 COMMENT '验证码字符数量（位数），例如 4/5/6',
	answer_hash VARCHAR(128) NOT NULL COMMENT '验证码答案哈希值',
	answer_salt VARCHAR(64) NOT NULL COMMENT '验证码答案哈希盐',
	issued_ip_hash VARCHAR(128) NOT NULL COMMENT '发放验证码时的请求IP哈希值',
	expires_at DATETIME(6) NOT NULL COMMENT '验证码过期时间',
	consumed_at DATETIME(6) NULL COMMENT '验证码消费时间；为空表示未消费',
	failed_verify_count INT NOT NULL DEFAULT 0 COMMENT '该挑战下验证码校验失败次数',
	created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间',
	updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '更新时间',
	CONSTRAINT ck_auth_captcha_challenges_level CHECK (level BETWEEN 1 AND 32),
	CONSTRAINT ck_auth_captcha_challenges_failed_verify_count CHECK (failed_verify_count >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='认证验证码挑战表：记录题目、有效期和消费状态';

CREATE INDEX idx_auth_captcha_challenges_scene_subject
	ON auth_captcha_challenges(scene, subject_hash);
CREATE INDEX idx_auth_captcha_challenges_expires_at
	ON auth_captcha_challenges(expires_at);
CREATE INDEX idx_auth_captcha_challenges_consumed_at
	ON auth_captcha_challenges(consumed_at);
