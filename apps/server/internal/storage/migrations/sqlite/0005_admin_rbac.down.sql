DROP INDEX IF EXISTS idx_documents_status;
DROP INDEX IF EXISTS idx_spaces_status;
DROP INDEX IF EXISTS idx_users_status;

DROP INDEX IF EXISTS idx_space_admin_scopes_space_id;
DROP INDEX IF EXISTS idx_space_admin_scopes_user_id;
DROP INDEX IF EXISTS idx_user_admin_roles_role;
DROP INDEX IF EXISTS idx_user_admin_roles_user_id;

DROP TABLE IF EXISTS space_admin_scopes;
DROP TABLE IF EXISTS user_admin_roles;

ALTER TABLE documents DROP COLUMN deleted_at;
ALTER TABLE documents DROP COLUMN banned_at;
ALTER TABLE documents DROP COLUMN banned_reason;
ALTER TABLE documents DROP COLUMN status;

ALTER TABLE spaces DROP COLUMN deleted_at;
ALTER TABLE spaces DROP COLUMN banned_at;
ALTER TABLE spaces DROP COLUMN banned_reason;
ALTER TABLE spaces DROP COLUMN status;

ALTER TABLE users DROP COLUMN deleted_at;
ALTER TABLE users DROP COLUMN banned_at;
ALTER TABLE users DROP COLUMN banned_reason;
ALTER TABLE users DROP COLUMN status;
