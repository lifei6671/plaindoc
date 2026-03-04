ALTER TABLE nodes
	ADD COLUMN reader_slug VARCHAR(120) NULL COMMENT '阅读页自定义文档标识（同空间唯一）';

CREATE UNIQUE INDEX uk_nodes_space_reader_slug ON nodes(space_id, reader_slug);
