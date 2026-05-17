CREATE TABLE IF NOT EXISTS admin_space_transfer_jobs (
	id BIGSERIAL PRIMARY KEY,
	job_id VARCHAR(26) NOT NULL,
	kind VARCHAR(32) NOT NULL,
	actor_user_id VARCHAR(64) NOT NULL,
	space_id VARCHAR(64) NOT NULL DEFAULT '',
	space_name VARCHAR(255) NOT NULL DEFAULT '',
	format VARCHAR(32) NOT NULL DEFAULT '',
	import_id VARCHAR(64) NOT NULL DEFAULT '',
	status VARCHAR(16) NOT NULL DEFAULT 'queued',
	stage VARCHAR(64) NOT NULL DEFAULT '',
	progress INTEGER NOT NULL DEFAULT 0,
	message TEXT NOT NULL DEFAULT '',
	file_path TEXT NOT NULL DEFAULT '',
	file_name VARCHAR(255) NOT NULL DEFAULT '',
	size_bytes BIGINT NOT NULL DEFAULT 0,
	new_space_id VARCHAR(64) NOT NULL DEFAULT '',
	error_message TEXT NOT NULL DEFAULT '',
	started_at TIMESTAMPTZ NULL,
	completed_at TIMESTAMPTZ NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	expires_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	CONSTRAINT uk_admin_space_transfer_jobs_job_id UNIQUE (job_id),
	CONSTRAINT chk_admin_space_transfer_jobs_kind CHECK (kind IN ('space_export', 'space_import')),
	CONSTRAINT chk_admin_space_transfer_jobs_status CHECK (status IN ('queued', 'running', 'completed', 'failed')),
	CONSTRAINT chk_admin_space_transfer_jobs_progress CHECK (progress >= 0 AND progress <= 100)
);

CREATE INDEX IF NOT EXISTS idx_admin_space_transfer_jobs_actor_status_updated
	ON admin_space_transfer_jobs(actor_user_id, status, updated_at, id);

CREATE INDEX IF NOT EXISTS idx_admin_space_transfer_jobs_status_expires
	ON admin_space_transfer_jobs(status, expires_at, id);

CREATE INDEX IF NOT EXISTS idx_admin_space_transfer_jobs_kind_job
	ON admin_space_transfer_jobs(kind, job_id);
