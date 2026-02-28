ALTER TABLE auth_captcha_challenges RENAME TO auth_captcha_challenges_old;

CREATE TABLE auth_captcha_challenges (
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
