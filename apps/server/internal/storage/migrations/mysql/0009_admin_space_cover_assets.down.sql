DROP INDEX idx_space_cover_assets_source ON space_cover_assets;
DROP INDEX idx_spaces_cover_asset_id ON spaces;

ALTER TABLE spaces DROP FOREIGN KEY fk_spaces_cover_asset_id;

DROP TABLE IF EXISTS space_cover_assets;

ALTER TABLE spaces
	DROP CHECK ck_spaces_cover_source,
	DROP COLUMN cover_source,
	DROP COLUMN cover_height,
	DROP COLUMN cover_width,
	DROP COLUMN cover_url,
	DROP COLUMN cover_key,
	DROP COLUMN cover_asset_id,
	DROP COLUMN description;
