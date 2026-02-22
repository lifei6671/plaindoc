DROP INDEX IF EXISTS idx_space_cover_assets_source;
DROP INDEX IF EXISTS idx_spaces_cover_asset_id;

ALTER TABLE spaces DROP CONSTRAINT IF EXISTS fk_spaces_cover_asset_id;
ALTER TABLE spaces DROP CONSTRAINT IF EXISTS ck_spaces_cover_source;

DROP TABLE IF EXISTS space_cover_assets;

ALTER TABLE spaces
	DROP COLUMN IF EXISTS cover_source,
	DROP COLUMN IF EXISTS cover_height,
	DROP COLUMN IF EXISTS cover_width,
	DROP COLUMN IF EXISTS cover_url,
	DROP COLUMN IF EXISTS cover_key,
	DROP COLUMN IF EXISTS cover_asset_id,
	DROP COLUMN IF EXISTS description;
