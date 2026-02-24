ALTER TABLE nodes
	ADD COLUMN created_by_user_id VARCHAR(26) NULL AFTER sort,
	ADD COLUMN updated_by_user_id VARCHAR(26) NULL AFTER created_by_user_id,
	ADD CONSTRAINT fk_nodes_created_by_user_id FOREIGN KEY (created_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL,
	ADD CONSTRAINT fk_nodes_updated_by_user_id FOREIGN KEY (updated_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL;

ALTER TABLE documents
	ADD COLUMN created_by_user_id VARCHAR(26) NULL AFTER version,
	ADD CONSTRAINT fk_documents_created_by_user_id FOREIGN KEY (created_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL;

CREATE INDEX idx_nodes_created_by_user_id ON nodes(created_by_user_id);
CREATE INDEX idx_nodes_updated_by_user_id ON nodes(updated_by_user_id);
CREATE INDEX idx_documents_created_by_user_id ON documents(created_by_user_id);

-- 文档创建人优先从首个修订写入，缺失时回退到当前更新人。
UPDATE documents AS d
LEFT JOIN document_revisions AS dr
	ON dr.document_id = d.document_id
	AND dr.version = 1
SET d.created_by_user_id = COALESCE(d.created_by_user_id, dr.editor_user_id, d.updated_by_user_id)
WHERE d.created_by_user_id IS NULL;

-- 目录节点同步冗余创建/更新人，文档节点后续查询可直接命中 nodes/documents 字段。
UPDATE nodes AS n
LEFT JOIN documents AS d
	ON d.node_id = n.node_id
SET
	n.created_by_user_id = COALESCE(n.created_by_user_id, d.created_by_user_id),
	n.updated_by_user_id = COALESCE(n.updated_by_user_id, d.updated_by_user_id, d.created_by_user_id)
WHERE d.node_id IS NOT NULL;
