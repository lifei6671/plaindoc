ALTER TABLE nodes
	ADD COLUMN created_by_user_id VARCHAR(26) NULL REFERENCES users(user_id) ON DELETE SET NULL;

ALTER TABLE nodes
	ADD COLUMN updated_by_user_id VARCHAR(26) NULL REFERENCES users(user_id) ON DELETE SET NULL;

ALTER TABLE documents
	ADD COLUMN created_by_user_id VARCHAR(26) NULL REFERENCES users(user_id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_nodes_created_by_user_id ON nodes(created_by_user_id);
CREATE INDEX IF NOT EXISTS idx_nodes_updated_by_user_id ON nodes(updated_by_user_id);
CREATE INDEX IF NOT EXISTS idx_documents_created_by_user_id ON documents(created_by_user_id);

-- 文档创建人优先从首个修订写入，缺失时回退到当前更新人。
UPDATE documents AS d
SET created_by_user_id = COALESCE(d.created_by_user_id, dr.editor_user_id, d.updated_by_user_id)
FROM document_revisions AS dr
WHERE dr.document_id = d.document_id
	AND dr.version = 1
	AND d.created_by_user_id IS NULL;

UPDATE documents
SET created_by_user_id = updated_by_user_id
WHERE created_by_user_id IS NULL
	AND updated_by_user_id IS NOT NULL;

-- 目录节点同步冗余创建/更新人，文档节点后续查询可直接命中 nodes/documents 字段。
UPDATE nodes AS n
SET
	created_by_user_id = COALESCE(n.created_by_user_id, d.created_by_user_id),
	updated_by_user_id = COALESCE(n.updated_by_user_id, d.updated_by_user_id, d.created_by_user_id)
FROM documents AS d
WHERE d.node_id = n.node_id;
