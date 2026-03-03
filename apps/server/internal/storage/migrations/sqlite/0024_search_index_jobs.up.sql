CREATE TABLE IF NOT EXISTS search_index_jobs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	job_id TEXT NOT NULL UNIQUE,
	provider TEXT NOT NULL DEFAULT '',
	job_type TEXT NOT NULL,
	dedupe_key TEXT NOT NULL,
	payload_json TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'success', 'failed')),
	priority INTEGER NOT NULL DEFAULT 100,
	retry_count INTEGER NOT NULL DEFAULT 0,
	next_run_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	started_at TEXT NULL,
	last_error TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_search_index_jobs_status_next_priority
	ON search_index_jobs(status, next_run_at, priority, id);

CREATE INDEX IF NOT EXISTS idx_search_index_jobs_dedupe_status
	ON search_index_jobs(dedupe_key, status, id);

CREATE INDEX IF NOT EXISTS idx_search_index_jobs_created_at
	ON search_index_jobs(created_at);
