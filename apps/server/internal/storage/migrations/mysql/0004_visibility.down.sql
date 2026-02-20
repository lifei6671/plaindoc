DROP INDEX idx_documents_visibility ON documents;
DROP INDEX idx_spaces_visibility ON spaces;

ALTER TABLE documents DROP COLUMN visibility;
ALTER TABLE spaces DROP COLUMN visibility;
