ALTER TABLE auth_captcha_challenges RENAME TO auth_captcha_challenges_new;

CREATE TABLE auth_captcha_challenges (
	id BIGINT PRIMARY KEY AUTO_INCREMENT,
	captcha_id VARCHAR(32) NOT NULL UNIQUE,
	scene VARCHAR(32) NOT NULL,
	subject_hash VARCHAR(128) NOT NULL,
	level INT NOT NULL DEFAULT 1,
	answer_hash VARCHAR(128) NOT NULL,
	answer_salt VARCHAR(64) NOT NULL,
	issued_ip_hash VARCHAR(128) NOT NULL,
	expires_at DATETIME(6) NOT NULL,
	consumed_at DATETIME(6) NULL,
	failed_verify_count INT NOT NULL DEFAULT 0,
	created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
	updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
	CONSTRAINT ck_auth_captcha_challenges_level CHECK (level BETWEEN 1 AND 3),
	CONSTRAINT ck_auth_captcha_challenges_failed_verify_count CHECK (failed_verify_count >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

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

CREATE INDEX idx_auth_captcha_challenges_scene_subject
	ON auth_captcha_challenges(scene, subject_hash);
CREATE INDEX idx_auth_captcha_challenges_expires_at
	ON auth_captcha_challenges(expires_at);
CREATE INDEX idx_auth_captcha_challenges_consumed_at
	ON auth_captcha_challenges(consumed_at);
