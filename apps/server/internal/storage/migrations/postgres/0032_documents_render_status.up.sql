ALTER TABLE documents
	ADD COLUMN IF NOT EXISTS render_status VARCHAR(16) NOT NULL DEFAULT 'idle',
	ADD COLUMN IF NOT EXISTS render_error TEXT NOT NULL DEFAULT '',
	ADD COLUMN IF NOT EXISTS rendered_at TIMESTAMPTZ NULL;

DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1
		FROM pg_constraint
		WHERE conname = 'ck_documents_render_status'
	) THEN
		ALTER TABLE documents
			ADD CONSTRAINT ck_documents_render_status
			CHECK (render_status IN ('idle', 'pending', 'success', 'failed'));
	END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_documents_render_status
	ON documents(render_status);
