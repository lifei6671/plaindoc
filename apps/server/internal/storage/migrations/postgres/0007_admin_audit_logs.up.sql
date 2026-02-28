CREATE TABLE IF NOT EXISTS audit_logs (
	id BIGSERIAL PRIMARY KEY,
	actor_user_id VARCHAR(26) NULL REFERENCES users(user_id) ON DELETE SET NULL,
	module VARCHAR(64) NOT NULL,
	action VARCHAR(64) NOT NULL,
	target_type VARCHAR(64) NOT NULL,
	target_id VARCHAR(191) NOT NULL,
	summary TEXT NOT NULL,
	detail_json TEXT NOT NULL DEFAULT '{}',
	request_id VARCHAR(64) NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

COMMENT ON TABLE audit_logs IS '后台审计日志表：记录管理端关键操作轨迹';
COMMENT ON COLUMN audit_logs.id IS '主键ID';
COMMENT ON COLUMN audit_logs.actor_user_id IS '操作者用户ID';
COMMENT ON COLUMN audit_logs.module IS '操作模块';
COMMENT ON COLUMN audit_logs.action IS '操作动作';
COMMENT ON COLUMN audit_logs.target_type IS '目标资源类型';
COMMENT ON COLUMN audit_logs.target_id IS '目标资源ID';
COMMENT ON COLUMN audit_logs.summary IS '操作摘要';
COMMENT ON COLUMN audit_logs.detail_json IS '操作详情JSON';
COMMENT ON COLUMN audit_logs.request_id IS '请求链路ID';
COMMENT ON COLUMN audit_logs.created_at IS '创建时间';

CREATE INDEX IF NOT EXISTS idx_audit_logs_actor_user_id ON audit_logs(actor_user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_module_action ON audit_logs(module, action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_target ON audit_logs(target_type, target_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at);
