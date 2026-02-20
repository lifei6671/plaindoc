DROP INDEX IF EXISTS idx_documents_visibility;
DROP INDEX IF EXISTS idx_spaces_visibility;

ALTER TABLE documents DROP CONSTRAINT IF EXISTS ck_documents_visibility;
ALTER TABLE spaces DROP CONSTRAINT IF EXISTS ck_spaces_visibility;

ALTER TABLE documents DROP COLUMN IF EXISTS visibility;
ALTER TABLE spaces DROP COLUMN IF EXISTS visibility;
