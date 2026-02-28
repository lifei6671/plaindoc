ALTER TABLE auth_captcha_challenges RENAME TO auth_captcha_challenges_old;

CREATE TABLE auth_captcha_challenges (
	id BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '主键ID',
	captcha_id VARCHAR(32) NOT NULL UNIQUE COMMENT '验证码会话ID（回传前端）',
	scene VARCHAR(32) NOT NULL COMMENT '验证码适用场景：login/register',
	subject_hash VARCHAR(128) NOT NULL COMMENT '风险主体哈希值（HMAC-SHA256）',
	level INT NOT NULL DEFAULT 4 COMMENT '验证码字符数量（位数），例如 4/5/6',
	answer_hash VARCHAR(128) NOT NULL COMMENT '验证码答案哈希值',
	answer_salt VARCHAR(64) NOT NULL COMMENT '验证码答案哈希盐',
	issued_ip_hash VARCHAR(128) NOT NULL COMMENT '发放验证码时的请求IP哈希值',
	expires_at DATETIME(6) NOT NULL COMMENT '验证码过期时间',
	consumed_at DATETIME(6) NULL COMMENT '验证码消费时间；为空表示未消费',
	failed_verify_count INT NOT NULL DEFAULT 0 COMMENT '该会话下验证码校验失败次数',
	created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间',
	updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '更新时间',
	CONSTRAINT ck_auth_captcha_challenges_level CHECK (level BETWEEN 1 AND 32),
	CONSTRAINT ck_auth_captcha_challenges_failed_verify_count CHECK (failed_verify_count >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='认证验证码会话表：记录题目、有效期和消费状态';

INSERT INTO auth_captcha_challenges (
	id,
	captcha_id,
	scene,
	subject_hash,
	level,
	answer_hash,
	answer_salt,
	issued_ip_hash,
	expires_at,
	consumed_at,
	failed_verify_count,
	created_at,
	updated_at
)
SELECT
	id,
	captcha_id,
	scene,
	subject_hash,
	CASE
		WHEN level < 1 THEN 1
		WHEN level > 32 THEN 32
		ELSE level
	END AS level,
	answer_hash,
	answer_salt,
	issued_ip_hash,
	expires_at,
	consumed_at,
	failed_verify_count,
	created_at,
	updated_at
FROM auth_captcha_challenges_old;

DROP TABLE auth_captcha_challenges_old;

CREATE INDEX idx_auth_captcha_challenges_scene_subject
	ON auth_captcha_challenges(scene, subject_hash);
CREATE INDEX idx_auth_captcha_challenges_expires_at
	ON auth_captcha_challenges(expires_at);
CREATE INDEX idx_auth_captcha_challenges_consumed_at
	ON auth_captcha_challenges(consumed_at);
