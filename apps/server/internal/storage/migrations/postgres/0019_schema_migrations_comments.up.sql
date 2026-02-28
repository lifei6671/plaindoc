COMMENT ON TABLE schema_migrations IS '数据库迁移记录表：记录已执行的迁移版本';
COMMENT ON COLUMN schema_migrations.version IS '迁移版本号（主键）';
COMMENT ON COLUMN schema_migrations.name IS '迁移名称';
COMMENT ON COLUMN schema_migrations.applied_at IS '迁移执行时间';
