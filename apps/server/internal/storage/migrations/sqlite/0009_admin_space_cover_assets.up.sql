ALTER TABLE spaces
	ADD COLUMN description TEXT NOT NULL DEFAULT '';
ALTER TABLE spaces
	ADD COLUMN cover_asset_id TEXT NULL;
ALTER TABLE spaces
	ADD COLUMN cover_key TEXT NOT NULL DEFAULT '';
ALTER TABLE spaces
	ADD COLUMN cover_url TEXT NOT NULL DEFAULT '';
ALTER TABLE spaces
	ADD COLUMN cover_width INTEGER NOT NULL DEFAULT 0;
ALTER TABLE spaces
	ADD COLUMN cover_height INTEGER NOT NULL DEFAULT 0;
ALTER TABLE spaces
	ADD COLUMN cover_source TEXT NOT NULL DEFAULT '' CHECK (cover_source IN ('', 'user_upload', 'system_generated'));

CREATE TABLE IF NOT EXISTS space_cover_assets (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	asset_id TEXT NOT NULL UNIQUE,
	source TEXT NOT NULL CHECK (source IN ('user_upload', 'system_generated')),
	object_key TEXT NOT NULL,
	object_url TEXT NOT NULL,
	mime_type TEXT NOT NULL,
	width INTEGER NOT NULL,
	height INTEGER NOT NULL,
	size_bytes INTEGER NOT NULL,
	normalized INTEGER NOT NULL DEFAULT 1 CHECK (normalized IN (0, 1)),
	created_by_user_id TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (created_by_user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_spaces_cover_asset_id ON spaces(cover_asset_id);
CREATE INDEX IF NOT EXISTS idx_space_cover_assets_source ON space_cover_assets(source);
