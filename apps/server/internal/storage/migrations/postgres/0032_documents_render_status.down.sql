DROP INDEX IF EXISTS idx_documents_render_status;

ALTER TABLE documents
	DROP CONSTRAINT IF EXISTS ck_documents_render_status;

ALTER TABLE documents
	DROP COLUMN IF EXISTS rendered_at,
	DROP COLUMN IF EXISTS render_error,
	DROP COLUMN IF EXISTS render_status;
