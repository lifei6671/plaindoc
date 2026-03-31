CREATE TABLE IF NOT EXISTS audit_logs (
	id BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '主键ID',
	actor_user_id VARCHAR(26) NULL COMMENT '操作者用户ID',
	module VARCHAR(64) NOT NULL COMMENT '操作模块',
	action VARCHAR(64) NOT NULL COMMENT '操作动作',
	target_type VARCHAR(64) NOT NULL COMMENT '目标资源类型',
	target_id VARCHAR(191) NOT NULL COMMENT '目标资源ID',
	summary TEXT NOT NULL COMMENT '操作摘要',
	detail_json LONGTEXT NOT NULL COMMENT '操作详情JSON',
	request_id VARCHAR(64) NOT NULL DEFAULT '' COMMENT '请求链路ID',
	created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间',
	CONSTRAINT fk_audit_logs_actor_user_id FOREIGN KEY (actor_user_id) REFERENCES users(user_id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='后台审计日志表：记录管理端关键操作轨迹';

CREATE INDEX idx_audit_logs_actor_user_id ON audit_logs(actor_user_id);
CREATE INDEX idx_audit_logs_module_action ON audit_logs(module, action);
CREATE INDEX idx_audit_logs_target ON audit_logs(target_type, target_id);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at);
