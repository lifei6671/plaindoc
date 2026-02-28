CREATE TABLE IF NOT EXISTS auth_risk_states (
	id BIGINT PRIMARY KEY AUTO_INCREMENT,
	scene VARCHAR(32) NOT NULL,
	subject_type VARCHAR(32) NOT NULL,
	subject_hash VARCHAR(128) NOT NULL,
	window_started_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
	attempt_count INT NOT NULL DEFAULT 0,
	failed_count INT NOT NULL DEFAULT 0,
	captcha_failed_count INT NOT NULL DEFAULT 0,
	lock_until DATETIME(6) NULL,
	created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
	updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
	UNIQUE KEY uk_auth_risk_states_scene_subject (scene, subject_type, subject_hash),
	CONSTRAINT ck_auth_risk_states_attempt_count CHECK (attempt_count >= 0),
	CONSTRAINT ck_auth_risk_states_failed_count CHECK (failed_count >= 0),
	CONSTRAINT ck_auth_risk_states_captcha_failed_count CHECK (captcha_failed_count >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE INDEX idx_auth_risk_states_lock_until ON auth_risk_states(lock_until);
CREATE INDEX idx_auth_risk_states_updated_at ON auth_risk_states(updated_at);

CREATE TABLE IF NOT EXISTS auth_captcha_challenges (
	id BIGINT PRIMARY KEY AUTO_INCREMENT,
	captcha_id VARCHAR(32) NOT NULL UNIQUE,
	scene VARCHAR(32) NOT NULL,
	subject_hash VARCHAR(128) NOT NULL,
	level INT NOT NULL DEFAULT 4,
	answer_hash VARCHAR(128) NOT NULL,
	answer_salt VARCHAR(64) NOT NULL,
	issued_ip_hash VARCHAR(128) NOT NULL,
	expires_at DATETIME(6) NOT NULL,
	consumed_at DATETIME(6) NULL,
	failed_verify_count INT NOT NULL DEFAULT 0,
	created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
	updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
	CONSTRAINT ck_auth_captcha_challenges_level CHECK (level BETWEEN 1 AND 32),
	CONSTRAINT ck_auth_captcha_challenges_failed_verify_count CHECK (failed_verify_count >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE INDEX idx_auth_captcha_challenges_scene_subject
	ON auth_captcha_challenges(scene, subject_hash);
CREATE INDEX idx_auth_captcha_challenges_expires_at
	ON auth_captcha_challenges(expires_at);
CREATE INDEX idx_auth_captcha_challenges_consumed_at
	ON auth_captcha_challenges(consumed_at);
