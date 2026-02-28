CREATE TABLE IF NOT EXISTS auth_risk_states (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	scene TEXT NOT NULL,
	subject_type TEXT NOT NULL,
	subject_hash TEXT NOT NULL,
	window_started_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
	failed_count INTEGER NOT NULL DEFAULT 0 CHECK (failed_count >= 0),
	captcha_failed_count INTEGER NOT NULL DEFAULT 0 CHECK (captcha_failed_count >= 0),
	lock_until TEXT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(scene, subject_type, subject_hash)
);

CREATE INDEX IF NOT EXISTS idx_auth_risk_states_scene_subject
	ON auth_risk_states(scene, subject_type, subject_hash);
CREATE INDEX IF NOT EXISTS idx_auth_risk_states_lock_until
	ON auth_risk_states(lock_until);
CREATE INDEX IF NOT EXISTS idx_auth_risk_states_updated_at
	ON auth_risk_states(updated_at);

CREATE TABLE IF NOT EXISTS auth_captcha_challenges (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	captcha_id TEXT NOT NULL UNIQUE,
	scene TEXT NOT NULL,
	subject_hash TEXT NOT NULL,
	level INTEGER NOT NULL DEFAULT 4 CHECK (level BETWEEN 1 AND 32),
	answer_hash TEXT NOT NULL,
	answer_salt TEXT NOT NULL,
	issued_ip_hash TEXT NOT NULL,
	expires_at TEXT NOT NULL,
	consumed_at TEXT NULL,
	failed_verify_count INTEGER NOT NULL DEFAULT 0 CHECK (failed_verify_count >= 0),
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_auth_captcha_challenges_scene_subject
	ON auth_captcha_challenges(scene, subject_hash);
CREATE INDEX IF NOT EXISTS idx_auth_captcha_challenges_expires_at
	ON auth_captcha_challenges(expires_at);
CREATE INDEX IF NOT EXISTS idx_auth_captcha_challenges_consumed_at
	ON auth_captcha_challenges(consumed_at);
