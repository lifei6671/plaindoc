DROP TABLE IF EXISTS system_configs;
DROP INDEX idx_themes_is_enabled ON themes;
ALTER TABLE themes DROP COLUMN is_enabled;
