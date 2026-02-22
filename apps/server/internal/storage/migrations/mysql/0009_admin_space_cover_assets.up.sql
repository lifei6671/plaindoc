ALTER TABLE spaces
	ADD COLUMN description VARCHAR(1024) NOT NULL DEFAULT '',
	ADD COLUMN cover_asset_id VARCHAR(26) NULL,
	ADD COLUMN cover_key VARCHAR(512) NOT NULL DEFAULT '',
	ADD COLUMN cover_url TEXT NOT NULL,
	ADD COLUMN cover_width INT NOT NULL DEFAULT 0,
	ADD COLUMN cover_height INT NOT NULL DEFAULT 0,
	ADD COLUMN cover_source VARCHAR(32) NOT NULL DEFAULT '',
	ADD CONSTRAINT ck_spaces_cover_source CHECK (cover_source IN ('', 'user_upload', 'system_generated'));

CREATE TABLE IF NOT EXISTS space_cover_assets (
	id BIGINT AUTO_INCREMENT PRIMARY KEY,
	asset_id VARCHAR(26) NOT NULL UNIQUE,
	source VARCHAR(32) NOT NULL,
	object_key VARCHAR(512) NOT NULL,
	object_url TEXT NOT NULL,
	mime_type VARCHAR(64) NOT NULL,
	width INT NOT NULL,
	height INT NOT NULL,
	size_bytes BIGINT NOT NULL,
	normalized TINYINT(1) NOT NULL DEFAULT 1,
	created_by_user_id VARCHAR(26) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	CONSTRAINT ck_space_cover_assets_source CHECK (source IN ('user_upload', 'system_generated')),
	CONSTRAINT ck_space_cover_assets_normalized CHECK (normalized IN (0, 1)),
	CONSTRAINT fk_space_cover_assets_created_by_user_id FOREIGN KEY (created_by_user_id) REFERENCES users(user_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE spaces
	ADD CONSTRAINT fk_spaces_cover_asset_id FOREIGN KEY (cover_asset_id) REFERENCES space_cover_assets(asset_id) ON DELETE SET NULL;

CREATE INDEX idx_spaces_cover_asset_id ON spaces(cover_asset_id);
CREATE INDEX idx_space_cover_assets_source ON space_cover_assets(source);
