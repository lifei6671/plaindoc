CREATE TABLE IF NOT EXISTS search_analyzer_dict_entries (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	analyzer TEXT NOT NULL,
	term TEXT NOT NULL,
	weight INTEGER NULL,
	tag TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'deleted')),
	created_by_user_id TEXT NULL,
	updated_by_user_id TEXT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (created_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL,
	FOREIGN KEY (updated_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_search_analyzer_dict_entries_analyzer_term
	ON search_analyzer_dict_entries(analyzer, term);

CREATE INDEX IF NOT EXISTS idx_search_analyzer_dict_entries_active_lookup
	ON search_analyzer_dict_entries(analyzer, status, updated_at);
