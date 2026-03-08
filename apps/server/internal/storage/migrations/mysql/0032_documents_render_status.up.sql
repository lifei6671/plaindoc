ALTER TABLE documents
	ADD COLUMN render_status VARCHAR(16) NOT NULL DEFAULT 'idle' COMMENT 'Office 阅读渲染状态：idle/pending/success/failed' AFTER content_md,
	ADD COLUMN render_error TEXT NULL COMMENT 'Office 阅读渲染错误信息' AFTER render_status,
	ADD COLUMN rendered_at DATETIME(3) NULL COMMENT 'Office 阅读渲染完成时间' AFTER render_error,
	ADD CONSTRAINT ck_documents_render_status CHECK (render_status IN ('idle', 'pending', 'success', 'failed')),
	ADD KEY idx_documents_render_status (render_status);
