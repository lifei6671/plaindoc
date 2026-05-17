CREATE TABLE IF NOT EXISTS admin_space_transfer_jobs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	job_id TEXT NOT NULL UNIQUE,
	kind TEXT NOT NULL CHECK (kind IN ('space_export', 'space_import')),
	actor_user_id TEXT NOT NULL,
	space_id TEXT NOT NULL DEFAULT '',
	space_name TEXT NOT NULL DEFAULT '',
	format TEXT NOT NULL DEFAULT '',
	import_id TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'completed', 'failed')),
	stage TEXT NOT NULL DEFAULT '',
	progress INTEGER NOT NULL DEFAULT 0 CHECK (progress >= 0 AND progress <= 100),
	message TEXT NOT NULL DEFAULT '',
	file_path TEXT NOT NULL DEFAULT '',
	file_name TEXT NOT NULL DEFAULT '',
	size_bytes INTEGER NOT NULL DEFAULT 0,
	new_space_id TEXT NOT NULL DEFAULT '',
	error_message TEXT NOT NULL DEFAULT '',
	started_at TIMESTAMP NULL,
	completed_at TIMESTAMP NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	expires_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_admin_space_transfer_jobs_actor_status_updated
	ON admin_space_transfer_jobs(actor_user_id, status, updated_at, id);

CREATE INDEX IF NOT EXISTS idx_admin_space_transfer_jobs_status_expires
	ON admin_space_transfer_jobs(status, expires_at, id);

CREATE INDEX IF NOT EXISTS idx_admin_space_transfer_jobs_kind_job
	ON admin_space_transfer_jobs(kind, job_id);
