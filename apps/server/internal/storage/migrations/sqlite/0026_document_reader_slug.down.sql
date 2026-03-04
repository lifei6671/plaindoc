DROP INDEX IF EXISTS uk_nodes_space_reader_slug;

-- SQLite 不支持直接删除列；reader_slug 列在 down 迁移中保留。
