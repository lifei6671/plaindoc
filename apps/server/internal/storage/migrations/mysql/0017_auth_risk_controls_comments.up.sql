ALTER TABLE auth_risk_states COMMENT = '认证风控状态表：按场景与主体维度累计窗口内尝试/失败计数，并记录临时封禁状态';

ALTER TABLE auth_risk_states
	MODIFY COLUMN id BIGINT NOT NULL AUTO_INCREMENT COMMENT '主键ID',
	MODIFY COLUMN scene VARCHAR(32) NOT NULL COMMENT '风控场景：login/register',
	MODIFY COLUMN subject_type VARCHAR(32) NOT NULL COMMENT '风险主体类型，如 ip、identifier、ip_identifier、email、ip_email',
	MODIFY COLUMN subject_hash VARCHAR(128) NOT NULL COMMENT '风险主体哈希值（HMAC-SHA256），不存储明文',
	MODIFY COLUMN window_started_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '统计窗口起始时间',
	MODIFY COLUMN attempt_count INT NOT NULL DEFAULT 0 COMMENT '窗口内总尝试次数（含成功与失败）',
	MODIFY COLUMN failed_count INT NOT NULL DEFAULT 0 COMMENT '窗口内失败次数',
	MODIFY COLUMN captcha_failed_count INT NOT NULL DEFAULT 0 COMMENT '窗口内验证码校验失败次数',
	MODIFY COLUMN lock_until DATETIME(6) NULL COMMENT '临时封禁截止时间；为空表示未封禁',
	MODIFY COLUMN created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '记录创建时间',
	MODIFY COLUMN updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '记录更新时间';

ALTER TABLE auth_captcha_challenges COMMENT = '认证验证码挑战表：记录验证码题目、有效期和一次性消费状态';

ALTER TABLE auth_captcha_challenges
	MODIFY COLUMN id BIGINT NOT NULL AUTO_INCREMENT COMMENT '主键ID',
	MODIFY COLUMN captcha_id VARCHAR(32) NOT NULL COMMENT '验证码挑战ID（回传前端）',
	MODIFY COLUMN scene VARCHAR(32) NOT NULL COMMENT '验证码适用场景：login/register',
	MODIFY COLUMN subject_hash VARCHAR(128) NOT NULL COMMENT '风险主体哈希值（HMAC-SHA256）',
	MODIFY COLUMN level INT NOT NULL DEFAULT 4 COMMENT '验证码字符数量（位数），例如 4/5/6',
	MODIFY COLUMN answer_hash VARCHAR(128) NOT NULL COMMENT '验证码答案哈希值',
	MODIFY COLUMN answer_salt VARCHAR(64) NOT NULL COMMENT '验证码答案哈希盐',
	MODIFY COLUMN issued_ip_hash VARCHAR(128) NOT NULL COMMENT '发放验证码时的请求IP哈希值',
	MODIFY COLUMN expires_at DATETIME(6) NOT NULL COMMENT '验证码过期时间',
	MODIFY COLUMN consumed_at DATETIME(6) NULL COMMENT '验证码消费时间；为空表示未消费',
	MODIFY COLUMN failed_verify_count INT NOT NULL DEFAULT 0 COMMENT '该挑战下验证码校验失败次数',
	MODIFY COLUMN created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '记录创建时间',
	MODIFY COLUMN updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '记录更新时间';
