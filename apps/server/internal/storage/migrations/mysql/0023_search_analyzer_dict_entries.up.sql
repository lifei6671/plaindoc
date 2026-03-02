CREATE TABLE IF NOT EXISTS search_analyzer_dict_entries (
	id BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '主键ID',
	analyzer VARCHAR(32) NOT NULL COMMENT '分词器名称：jieba/simple',
	term VARCHAR(191) NOT NULL COMMENT '词条内容',
	weight INT NULL COMMENT '分词权重（可选）',
	tag VARCHAR(64) NOT NULL DEFAULT '' COMMENT '词性标签（可选）',
	status VARCHAR(16) NOT NULL DEFAULT 'active' COMMENT '词条状态：active/deleted',
	created_by_user_id VARCHAR(26) NULL COMMENT '创建人用户ID',
	updated_by_user_id VARCHAR(26) NULL COMMENT '更新人用户ID',
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
	UNIQUE KEY uk_search_analyzer_dict_entries_analyzer_term (analyzer, term),
	KEY idx_search_analyzer_dict_entries_active_lookup (analyzer, status, updated_at),
	CONSTRAINT fk_search_analyzer_dict_entries_created_by_user_id FOREIGN KEY (created_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL,
	CONSTRAINT fk_search_analyzer_dict_entries_updated_by_user_id FOREIGN KEY (updated_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='全文检索分词词典词条表';
