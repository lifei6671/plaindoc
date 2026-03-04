ALTER TABLE nodes
	ADD COLUMN IF NOT EXISTS reader_slug VARCHAR(120);

CREATE UNIQUE INDEX IF NOT EXISTS uk_nodes_space_reader_slug ON nodes(space_id, reader_slug) WHERE reader_slug IS NOT NULL;
