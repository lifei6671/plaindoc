CREATE TABLE IF NOT EXISTS search_index_jobs (
	id BIGSERIAL PRIMARY KEY,
	job_id VARCHAR(26) NOT NULL,
	provider VARCHAR(32) NOT NULL DEFAULT '',
	job_type VARCHAR(32) NOT NULL,
	dedupe_key VARCHAR(255) NOT NULL,
	payload_json TEXT NOT NULL,
	status VARCHAR(16) NOT NULL DEFAULT 'pending',
	priority INTEGER NOT NULL DEFAULT 100,
	retry_count INTEGER NOT NULL DEFAULT 0,
	next_run_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	started_at TIMESTAMPTZ NULL,
	last_error TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	CONSTRAINT uk_search_index_jobs_job_id UNIQUE (job_id),
	CONSTRAINT chk_search_index_jobs_status CHECK (status IN ('pending', 'running', 'success', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_search_index_jobs_status_next_priority
	ON search_index_jobs(status, next_run_at, priority, id);

CREATE INDEX IF NOT EXISTS idx_search_index_jobs_dedupe_status
	ON search_index_jobs(dedupe_key, status, id);

CREATE INDEX IF NOT EXISTS idx_search_index_jobs_created_at
	ON search_index_jobs(created_at);
