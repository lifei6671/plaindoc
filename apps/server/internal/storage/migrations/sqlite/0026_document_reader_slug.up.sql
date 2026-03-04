ALTER TABLE nodes ADD COLUMN reader_slug TEXT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uk_nodes_space_reader_slug ON nodes(space_id, reader_slug) WHERE reader_slug IS NOT NULL;
