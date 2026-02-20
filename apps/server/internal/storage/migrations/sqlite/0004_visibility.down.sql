DROP INDEX IF EXISTS idx_documents_visibility;
DROP INDEX IF EXISTS idx_spaces_visibility;

ALTER TABLE documents DROP COLUMN visibility;
ALTER TABLE spaces DROP COLUMN visibility;
