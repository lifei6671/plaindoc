ALTER TABLE auth_captcha_challenges RENAME TO auth_captcha_challenges_new;

CREATE TABLE auth_captcha_challenges (
	id BIGSERIAL PRIMARY KEY,
	captcha_id VARCHAR(32) NOT NULL UNIQUE,
	scene VARCHAR(32) NOT NULL,
	subject_hash VARCHAR(128) NOT NULL,
	level INTEGER NOT NULL DEFAULT 1 CHECK (level BETWEEN 1 AND 3),
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
COMMENT ON COLUMN auth_captcha_challenges.level IS '验证码字符数量（位数），历史回滚语义为 1..3';
COMMENT ON COLUMN auth_captcha_challenges.answer_hash IS '验证码答案哈希值';
COMMENT ON COLUMN auth_captcha_challenges.answer_salt IS '验证码答案哈希盐';
COMMENT ON COLUMN auth_captcha_challenges.issued_ip_hash IS '发放验证码时的请求IP哈希值';
COMMENT ON COLUMN auth_captcha_challenges.expires_at IS '验证码过期时间';
COMMENT ON COLUMN auth_captcha_challenges.consumed_at IS '验证码消费时间；为空表示未消费';
COMMENT ON COLUMN auth_captcha_challenges.failed_verify_count IS '该会话下验证码校验失败次数';
COMMENT ON COLUMN auth_captcha_challenges.created_at IS '创建时间';
COMMENT ON COLUMN auth_captcha_challenges.updated_at IS '更新时间';

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
		WHEN level > 3 THEN 3
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
FROM auth_captcha_challenges_new;

DROP TABLE auth_captcha_challenges_new;

CREATE INDEX IF NOT EXISTS idx_auth_captcha_challenges_scene_subject
	ON auth_captcha_challenges(scene, subject_hash);
CREATE INDEX IF NOT EXISTS idx_auth_captcha_challenges_expires_at
	ON auth_captcha_challenges(expires_at);
CREATE INDEX IF NOT EXISTS idx_auth_captcha_challenges_consumed_at
	ON auth_captcha_challenges(consumed_at);

SELECT setval(
	pg_get_serial_sequence('auth_captcha_challenges', 'id'),
	COALESCE((SELECT MAX(id) FROM auth_captcha_challenges), 1),
	true
);
