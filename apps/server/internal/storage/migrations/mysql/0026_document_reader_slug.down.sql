DROP INDEX uk_nodes_space_reader_slug ON nodes;

ALTER TABLE nodes
	DROP COLUMN reader_slug;
