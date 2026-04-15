ALTER TABLE documents
	ADD COLUMN render_status TEXT NOT NULL DEFAULT 'idle' CHECK (render_status IN ('idle', 'pending', 'success', 'failed'));

ALTER TABLE documents
	ADD COLUMN render_error TEXT NOT NULL DEFAULT '';

ALTER TABLE documents
	ADD COLUMN rendered_at TIMESTAMP NULL;

CREATE INDEX IF NOT EXISTS idx_documents_render_status
	ON documents(render_status);
