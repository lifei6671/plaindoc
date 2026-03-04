DROP INDEX IF EXISTS uk_nodes_space_reader_slug;

ALTER TABLE nodes
	DROP COLUMN IF EXISTS reader_slug;
