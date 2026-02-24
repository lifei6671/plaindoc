DROP INDEX idx_documents_created_by_user_id ON documents;
DROP INDEX idx_nodes_updated_by_user_id ON nodes;
DROP INDEX idx_nodes_created_by_user_id ON nodes;

ALTER TABLE documents
	DROP FOREIGN KEY fk_documents_created_by_user_id,
	DROP COLUMN created_by_user_id;

ALTER TABLE nodes
	DROP FOREIGN KEY fk_nodes_updated_by_user_id,
	DROP FOREIGN KEY fk_nodes_created_by_user_id,
	DROP COLUMN updated_by_user_id,
	DROP COLUMN created_by_user_id;
