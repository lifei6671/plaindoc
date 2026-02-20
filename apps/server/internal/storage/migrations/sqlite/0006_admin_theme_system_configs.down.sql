DROP TABLE IF EXISTS system_configs;
DROP INDEX IF EXISTS idx_themes_is_enabled;
-- SQLite 回滚不执行 DROP COLUMN，以避免旧版本兼容性问题。
