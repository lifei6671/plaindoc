CREATE TABLE IF NOT EXISTS search_analyzer_dict_entries (
	id BIGSERIAL PRIMARY KEY,
	analyzer VARCHAR(32) NOT NULL,
	term VARCHAR(191) NOT NULL,
	weight INTEGER NULL,
	tag VARCHAR(64) NOT NULL DEFAULT '',
	status VARCHAR(16) NOT NULL DEFAULT 'active',
	created_by_user_id VARCHAR(26) NULL,
	updated_by_user_id VARCHAR(26) NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	CONSTRAINT chk_search_analyzer_dict_entries_status CHECK (status IN ('active', 'deleted')),
	CONSTRAINT fk_search_analyzer_dict_entries_created_by_user_id FOREIGN KEY (created_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL,
	CONSTRAINT fk_search_analyzer_dict_entries_updated_by_user_id FOREIGN KEY (updated_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_search_analyzer_dict_entries_analyzer_term
	ON search_analyzer_dict_entries(analyzer, term);

CREATE INDEX IF NOT EXISTS idx_search_analyzer_dict_entries_active_lookup
	ON search_analyzer_dict_entries(analyzer, status, updated_at);
