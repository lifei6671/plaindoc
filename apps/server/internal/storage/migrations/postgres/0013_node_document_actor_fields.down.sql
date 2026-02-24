DROP INDEX IF EXISTS idx_documents_created_by_user_id;
DROP INDEX IF EXISTS idx_nodes_updated_by_user_id;
DROP INDEX IF EXISTS idx_nodes_created_by_user_id;

ALTER TABLE documents
	DROP COLUMN IF EXISTS created_by_user_id;

ALTER TABLE nodes
	DROP COLUMN IF EXISTS updated_by_user_id;

ALTER TABLE nodes
	DROP COLUMN IF EXISTS created_by_user_id;
