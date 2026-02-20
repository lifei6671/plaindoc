DROP INDEX IF EXISTS idx_documents_status;
DROP INDEX IF EXISTS idx_spaces_status;
DROP INDEX IF EXISTS idx_users_status;

DROP INDEX IF EXISTS idx_space_admin_scopes_space_id;
DROP INDEX IF EXISTS idx_space_admin_scopes_user_id;
DROP INDEX IF EXISTS idx_user_admin_roles_role;
DROP INDEX IF EXISTS idx_user_admin_roles_user_id;

ALTER TABLE documents DROP CONSTRAINT IF EXISTS ck_documents_status;
ALTER TABLE spaces DROP CONSTRAINT IF EXISTS ck_spaces_status;
ALTER TABLE users DROP CONSTRAINT IF EXISTS ck_users_status;

ALTER TABLE documents
	DROP COLUMN IF EXISTS deleted_at,
	DROP COLUMN IF EXISTS banned_at,
	DROP COLUMN IF EXISTS banned_reason,
	DROP COLUMN IF EXISTS status;

ALTER TABLE spaces
	DROP COLUMN IF EXISTS deleted_at,
	DROP COLUMN IF EXISTS banned_at,
	DROP COLUMN IF EXISTS banned_reason,
	DROP COLUMN IF EXISTS status;

ALTER TABLE users
	DROP COLUMN IF EXISTS deleted_at,
	DROP COLUMN IF EXISTS banned_at,
	DROP COLUMN IF EXISTS banned_reason,
	DROP COLUMN IF EXISTS status;

DROP TABLE IF EXISTS space_admin_scopes;
DROP TABLE IF EXISTS user_admin_roles;
