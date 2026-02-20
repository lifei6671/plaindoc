CREATE TABLE IF NOT EXISTS audit_logs (
	id BIGINT PRIMARY KEY AUTO_INCREMENT,
	actor_user_id VARCHAR(26) NULL,
	module VARCHAR(64) NOT NULL,
	action VARCHAR(64) NOT NULL,
	target_type VARCHAR(64) NOT NULL,
	target_id VARCHAR(191) NOT NULL,
	summary TEXT NOT NULL,
	detail_json LONGTEXT NOT NULL,
	request_id VARCHAR(64) NOT NULL DEFAULT '',
	created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
	CONSTRAINT fk_audit_logs_actor_user_id FOREIGN KEY (actor_user_id) REFERENCES users(user_id) ON DELETE SET NULL
);

CREATE INDEX idx_audit_logs_actor_user_id ON audit_logs(actor_user_id);
CREATE INDEX idx_audit_logs_module_action ON audit_logs(module, action);
CREATE INDEX idx_audit_logs_target ON audit_logs(target_type, target_id);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at);
