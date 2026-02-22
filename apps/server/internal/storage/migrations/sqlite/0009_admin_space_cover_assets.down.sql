DROP INDEX IF EXISTS idx_space_cover_assets_source;
DROP INDEX IF EXISTS idx_spaces_cover_asset_id;

DROP TABLE IF EXISTS space_cover_assets;

ALTER TABLE spaces DROP COLUMN cover_source;
ALTER TABLE spaces DROP COLUMN cover_height;
ALTER TABLE spaces DROP COLUMN cover_width;
ALTER TABLE spaces DROP COLUMN cover_url;
ALTER TABLE spaces DROP COLUMN cover_key;
ALTER TABLE spaces DROP COLUMN cover_asset_id;
ALTER TABLE spaces DROP COLUMN description;
