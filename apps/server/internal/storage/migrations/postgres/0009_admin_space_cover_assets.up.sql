ALTER TABLE spaces
	ADD COLUMN description TEXT NOT NULL DEFAULT '',
	ADD COLUMN cover_asset_id VARCHAR(26) NULL,
	ADD COLUMN cover_key VARCHAR(512) NOT NULL DEFAULT '',
	ADD COLUMN cover_url TEXT NOT NULL DEFAULT '',
	ADD COLUMN cover_width INTEGER NOT NULL DEFAULT 0,
	ADD COLUMN cover_height INTEGER NOT NULL DEFAULT 0,
	ADD COLUMN cover_source VARCHAR(32) NOT NULL DEFAULT '';

ALTER TABLE spaces
	ADD CONSTRAINT ck_spaces_cover_source
	CHECK (cover_source IN ('', 'user_upload', 'system_generated'));

CREATE TABLE IF NOT EXISTS space_cover_assets (
	id BIGSERIAL PRIMARY KEY,
	asset_id VARCHAR(26) NOT NULL UNIQUE,
	source VARCHAR(32) NOT NULL CHECK (source IN ('user_upload', 'system_generated')),
	object_key VARCHAR(512) NOT NULL,
	object_url TEXT NOT NULL,
	mime_type VARCHAR(64) NOT NULL,
	width INTEGER NOT NULL,
	height INTEGER NOT NULL,
	size_bytes BIGINT NOT NULL,
	normalized BOOLEAN NOT NULL DEFAULT TRUE,
	created_by_user_id VARCHAR(26) NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE spaces
	ADD CONSTRAINT fk_spaces_cover_asset_id
	FOREIGN KEY (cover_asset_id) REFERENCES space_cover_assets(asset_id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_spaces_cover_asset_id ON spaces(cover_asset_id);
CREATE INDEX IF NOT EXISTS idx_space_cover_assets_source ON space_cover_assets(source);
