DROP TABLE IF EXISTS system_configs;
DROP INDEX IF EXISTS idx_themes_is_enabled;
ALTER TABLE themes DROP COLUMN IF EXISTS is_enabled;
