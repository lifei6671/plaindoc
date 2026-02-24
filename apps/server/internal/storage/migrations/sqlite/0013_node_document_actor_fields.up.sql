ALTER TABLE nodes
	ADD COLUMN created_by_user_id TEXT NULL REFERENCES users(user_id) ON DELETE SET NULL;

ALTER TABLE nodes
	ADD COLUMN updated_by_user_id TEXT NULL REFERENCES users(user_id) ON DELETE SET NULL;

ALTER TABLE documents
	ADD COLUMN created_by_user_id TEXT NULL REFERENCES users(user_id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_nodes_created_by_user_id ON nodes(created_by_user_id);
CREATE INDEX IF NOT EXISTS idx_nodes_updated_by_user_id ON nodes(updated_by_user_id);
CREATE INDEX IF NOT EXISTS idx_documents_created_by_user_id ON documents(created_by_user_id);

-- 文档创建人优先从首个修订写入，缺失时回退到当前更新人。
UPDATE documents
SET created_by_user_id = COALESCE(
	created_by_user_id,
	(
		SELECT dr.editor_user_id
		FROM document_revisions AS dr
		WHERE dr.document_id = documents.document_id
			AND dr.version = 1
		LIMIT 1
	),
	updated_by_user_id
)
WHERE created_by_user_id IS NULL;

-- 目录节点同步冗余创建/更新人，文档节点后续查询可直接命中 nodes/documents 字段。
UPDATE nodes
SET created_by_user_id = COALESCE(
		created_by_user_id,
		(
			SELECT d.created_by_user_id
			FROM documents AS d
			WHERE d.node_id = nodes.node_id
			LIMIT 1
		)
	),
	updated_by_user_id = COALESCE(
		updated_by_user_id,
		(
			SELECT d.updated_by_user_id
			FROM documents AS d
			WHERE d.node_id = nodes.node_id
			LIMIT 1
		),
		(
			SELECT d.created_by_user_id
			FROM documents AS d
			WHERE d.node_id = nodes.node_id
			LIMIT 1
		)
	)
WHERE EXISTS (
	SELECT 1
	FROM documents AS d
	WHERE d.node_id = nodes.node_id
);
