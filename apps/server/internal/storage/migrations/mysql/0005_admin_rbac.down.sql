DROP INDEX idx_documents_status ON documents;
DROP INDEX idx_spaces_status ON spaces;
DROP INDEX idx_users_status ON users;

ALTER TABLE documents
	DROP COLUMN deleted_at,
	DROP COLUMN banned_at,
	DROP COLUMN banned_reason,
	DROP COLUMN status;

ALTER TABLE spaces
	DROP COLUMN deleted_at,
	DROP COLUMN banned_at,
	DROP COLUMN banned_reason,
	DROP COLUMN status;

ALTER TABLE users
	DROP COLUMN deleted_at,
	DROP COLUMN banned_at,
	DROP COLUMN banned_reason,
	DROP COLUMN status;

DROP TABLE IF EXISTS space_admin_scopes;
DROP TABLE IF EXISTS user_admin_roles;
