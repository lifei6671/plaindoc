ALTER TABLE documents
	DROP KEY idx_documents_render_status,
	DROP CHECK ck_documents_render_status,
	DROP COLUMN rendered_at,
	DROP COLUMN render_error,
	DROP COLUMN render_status;
