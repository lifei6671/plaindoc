CREATE TABLE IF NOT EXISTS auth_captcha_challenges_v16_stage (
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
	CONSTRAINT ck_auth_captcha_challenges_v16_stage_level CHECK (level BETWEEN 1 AND 32),
	CONSTRAINT ck_auth_captcha_challenges_v16_stage_failed_verify_count CHECK (failed_verify_count >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='认证验证码迁移中转表：用于 0016 平滑重建验证码会话表';

SET @captcha_old_table_exists = (
	SELECT COUNT(*)
	FROM information_schema.tables
	WHERE table_schema = DATABASE()
		AND table_name = 'auth_captcha_challenges_old'
);
SET @sql = IF(
	@captcha_old_table_exists = 1,
	'INSERT INTO auth_captcha_challenges_v16_stage (
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
		old.id,
		old.captcha_id,
		old.scene,
		old.subject_hash,
		CASE
			WHEN old.level < 1 THEN 1
			WHEN old.level > 32 THEN 32
			ELSE old.level
		END AS level,
		old.answer_hash,
		old.answer_salt,
		old.issued_ip_hash,
		old.expires_at,
		old.consumed_at,
		old.failed_verify_count,
		old.created_at,
		old.updated_at
	FROM auth_captcha_challenges_old AS old
	LEFT JOIN auth_captcha_challenges_v16_stage AS stage
		ON stage.id = old.id
	WHERE stage.id IS NULL',
	'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @captcha_table_exists = (
	SELECT COUNT(*)
	FROM information_schema.tables
	WHERE table_schema = DATABASE()
		AND table_name = 'auth_captcha_challenges'
);
SET @sql = IF(
	@captcha_table_exists = 1,
	'INSERT INTO auth_captcha_challenges_v16_stage (
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
		current_table.id,
		current_table.captcha_id,
		current_table.scene,
		current_table.subject_hash,
		CASE
			WHEN current_table.level < 1 THEN 1
			WHEN current_table.level > 32 THEN 32
			ELSE current_table.level
		END AS level,
		current_table.answer_hash,
		current_table.answer_salt,
		current_table.issued_ip_hash,
		current_table.expires_at,
		current_table.consumed_at,
		current_table.failed_verify_count,
		current_table.created_at,
		current_table.updated_at
	FROM auth_captcha_challenges AS current_table
	LEFT JOIN auth_captcha_challenges_v16_stage AS stage
		ON stage.id = current_table.id
	WHERE stage.id IS NULL',
	'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

DROP TABLE IF EXISTS auth_captcha_challenges_old;
DROP TABLE IF EXISTS auth_captcha_challenges;

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
	stage.id,
	stage.captcha_id,
	stage.scene,
	stage.subject_hash,
	stage.level,
	stage.answer_hash,
	stage.answer_salt,
	stage.issued_ip_hash,
	stage.expires_at,
	stage.consumed_at,
	stage.failed_verify_count,
	stage.created_at,
	stage.updated_at
FROM auth_captcha_challenges_v16_stage AS stage;

CREATE INDEX idx_auth_captcha_challenges_scene_subject
	ON auth_captcha_challenges(scene, subject_hash);
CREATE INDEX idx_auth_captcha_challenges_expires_at
	ON auth_captcha_challenges(expires_at);
CREATE INDEX idx_auth_captcha_challenges_consumed_at
	ON auth_captcha_challenges(consumed_at);

DROP TABLE IF EXISTS auth_captcha_challenges_v16_stage;
