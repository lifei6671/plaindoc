CREATE TABLE IF NOT EXISTS users (
	id BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '主键ID',
	user_id VARCHAR(26) NOT NULL UNIQUE COMMENT '用户业务ID（ULID）',
	email VARCHAR(320) NOT NULL UNIQUE COMMENT '用户邮箱（全局唯一）',
	password_hash TEXT NOT NULL COMMENT '密码哈希（bcrypt）',
	name VARCHAR(128) NOT NULL COMMENT '用户显示名称',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户主表：存储账号基础信息';

CREATE TABLE IF NOT EXISTS spaces (
	id BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '主键ID',
	space_id VARCHAR(26) NOT NULL UNIQUE COMMENT '空间业务ID（ULID）',
	name VARCHAR(255) NOT NULL COMMENT '空间名称',
	owner_user_id VARCHAR(26) NOT NULL COMMENT '空间所有者用户ID',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
	CONSTRAINT fk_spaces_owner_user_id FOREIGN KEY (owner_user_id) REFERENCES users(user_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='知识空间表：协作内容顶层容器';

CREATE TABLE IF NOT EXISTS space_members (
	id BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '主键ID',
	space_id VARCHAR(26) NOT NULL COMMENT '空间业务ID',
	user_id VARCHAR(26) NOT NULL COMMENT '用户业务ID',
	role VARCHAR(16) NOT NULL COMMENT '空间角色：owner/collaborator/reader',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
	UNIQUE KEY uk_space_members_space_user (space_id, user_id),
	CONSTRAINT ck_space_members_role CHECK (role IN ('owner', 'collaborator', 'reader')),
	CONSTRAINT fk_space_members_space_id FOREIGN KEY (space_id) REFERENCES spaces(space_id) ON DELETE CASCADE,
	CONSTRAINT fk_space_members_user_id FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='空间成员关系表：定义用户在空间中的协作角色';

CREATE TABLE IF NOT EXISTS nodes (
	id BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '主键ID',
	node_id VARCHAR(26) NOT NULL UNIQUE COMMENT '节点业务ID（ULID）',
	space_id VARCHAR(26) NOT NULL COMMENT '所属空间ID',
	parent_node_id VARCHAR(26) NULL COMMENT '父节点ID，空表示根节点',
	type VARCHAR(16) NOT NULL COMMENT '节点类型：folder/doc',
	title VARCHAR(255) NOT NULL COMMENT '节点标题',
	sort INT NOT NULL DEFAULT 0 COMMENT '同级排序值',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
	CONSTRAINT ck_nodes_type CHECK (type IN ('folder', 'doc')),
	CONSTRAINT fk_nodes_space_id FOREIGN KEY (space_id) REFERENCES spaces(space_id) ON DELETE CASCADE,
	CONSTRAINT fk_nodes_parent_node_id FOREIGN KEY (parent_node_id) REFERENCES nodes(node_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='目录树节点表：空间内目录与文档节点';

CREATE TABLE IF NOT EXISTS themes (
	id BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '主键ID',
	theme_id VARCHAR(64) NOT NULL UNIQUE COMMENT '主题业务ID',
	name VARCHAR(128) NOT NULL COMMENT '主题名称',
	description TEXT NOT NULL COMMENT '主题描述',
	variables_json MEDIUMTEXT NOT NULL COMMENT 'CSS 变量JSON',
	syntax_theme VARCHAR(32) NOT NULL COMMENT '代码高亮主题：one-light/one-dark',
	code_block_style_json MEDIUMTEXT NOT NULL COMMENT '代码块容器样式JSON',
	code_block_code_style_json MEDIUMTEXT NOT NULL COMMENT '代码块代码样式JSON',
	inline_code_style_json MEDIUMTEXT NOT NULL COMMENT '行内代码样式JSON',
	custom_css MEDIUMTEXT NOT NULL COMMENT '自定义CSS',
	is_builtin TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否内置主题',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
	CONSTRAINT ck_themes_syntax_theme CHECK (syntax_theme IN ('one-light', 'one-dark'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='文档主题表：存储主题样式与代码高亮配置';

INSERT INTO themes (
	theme_id,
	name,
	description,
	variables_json,
	syntax_theme,
	code_block_style_json,
	code_block_code_style_json,
	inline_code_style_json,
	custom_css,
	is_builtin
) VALUES (
	'default',
	'内置默认',
	'通用文档风格',
	'{"--pd-preview-padding":"30px","--pd-preview-font-family":"Optima-Regular, Optima, PingFangSC-light, PingFangTC-light, PingFang SC, Cambria, Cochin, Georgia, Times, Times New Roman, serif","--pd-preview-text-color":"rgb(89, 89, 89)","--pd-preview-link-color":"rgb(71, 193, 168)","--pd-preview-inline-code-color":"rgb(71, 193, 168)","--pd-preview-font-size":"16px","--pd-preview-line-height":"26px"}',
	'one-light',
	'{"margin":"16px 0","padding":"14px 16px","borderRadius":"10px","border":"1px solid #dbe2ea","boxShadow":"0 1px 2px rgba(15, 23, 42, 0.06)","overflowX":"auto","fontSize":"13px","lineHeight":"1.65","background":"#f8fafc"}',
	'{"fontFamily":"Google Sans Code, Operator Mono, SFMono-Regular, Menlo, Monaco, Consolas, Liberation Mono, Courier New, monospace"}',
	'{"padding":"1px 6px","borderRadius":"5px","border":"1px solid #dbe2ea","background":"#f1f5f9","color":"#0f172a","fontSize":"0.92em","fontFamily":"Google Sans Code, Operator Mono, SFMono-Regular, Menlo, Monaco, Consolas, Liberation Mono, Courier New, monospace"}',
	'',
	1
)
ON DUPLICATE KEY UPDATE theme_id = theme_id;

CREATE TABLE IF NOT EXISTS documents (
	id BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '主键ID',
	document_id VARCHAR(26) NOT NULL UNIQUE COMMENT '文档业务ID（ULID）',
	node_id VARCHAR(26) NOT NULL UNIQUE COMMENT '关联节点ID（唯一）',
	theme_id VARCHAR(64) NOT NULL DEFAULT 'default' COMMENT '主题ID',
	title VARCHAR(255) NOT NULL COMMENT '文档标题',
	content_md MEDIUMTEXT NOT NULL COMMENT 'Markdown 正文',
	version INT NOT NULL DEFAULT 1 COMMENT '文档版本号（乐观锁）',
	updated_by_user_id VARCHAR(26) NULL COMMENT '最后更新人用户ID',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
	CONSTRAINT fk_documents_node_id FOREIGN KEY (node_id) REFERENCES nodes(node_id) ON DELETE CASCADE,
	CONSTRAINT fk_documents_theme_id FOREIGN KEY (theme_id) REFERENCES themes(theme_id) ON DELETE RESTRICT,
	CONSTRAINT fk_documents_updated_by_user_id FOREIGN KEY (updated_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='文档主表：节点对应的正文';

CREATE TABLE IF NOT EXISTS document_revisions (
	id BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '主键ID',
	document_revision_id VARCHAR(26) NOT NULL UNIQUE COMMENT '修订业务ID（ULID）',
	document_id VARCHAR(26) NOT NULL COMMENT '文档业务ID',
	version INT NOT NULL COMMENT '修订版本号',
	content_md MEDIUMTEXT NOT NULL COMMENT '该版本 Markdown 内容',
	base_version INT NOT NULL COMMENT '保存时基准版本号',
	editor_user_id VARCHAR(26) NULL COMMENT '编辑人用户ID',
	source VARCHAR(16) NOT NULL COMMENT '修订来源：local/remote',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
	UNIQUE KEY uk_document_revisions_document_version (document_id, version),
	CONSTRAINT ck_document_revisions_source CHECK (source IN ('local', 'remote')),
	CONSTRAINT fk_document_revisions_document_id FOREIGN KEY (document_id) REFERENCES documents(document_id) ON DELETE CASCADE,
	CONSTRAINT fk_document_revisions_editor_user_id FOREIGN KEY (editor_user_id) REFERENCES users(user_id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='文档修订历史表：保存版本快照与来源';

CREATE TABLE IF NOT EXISTS node_permissions (
	id BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '主键ID',
	node_id VARCHAR(26) NOT NULL COMMENT '节点业务ID',
	user_id VARCHAR(26) NOT NULL COMMENT '被授权用户ID',
	role VARCHAR(16) NOT NULL COMMENT '授权角色：owner/collaborator/reader',
	granted_by_user_id VARCHAR(26) NOT NULL COMMENT '授权人用户ID',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
	UNIQUE KEY uk_node_permissions_node_user (node_id, user_id),
	CONSTRAINT ck_node_permissions_role CHECK (role IN ('owner', 'collaborator', 'reader')),
	CONSTRAINT fk_node_permissions_node_id FOREIGN KEY (node_id) REFERENCES nodes(node_id) ON DELETE CASCADE,
	CONSTRAINT fk_node_permissions_user_id FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
	CONSTRAINT fk_node_permissions_granted_by_user_id FOREIGN KEY (granted_by_user_id) REFERENCES users(user_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='节点权限表：节点级 ACL 授权关系';

CREATE TABLE IF NOT EXISTS document_permissions (
	id BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '主键ID',
	document_id VARCHAR(26) NOT NULL COMMENT '文档业务ID',
	user_id VARCHAR(26) NOT NULL COMMENT '被授权用户ID',
	role VARCHAR(16) NOT NULL COMMENT '授权角色：owner/collaborator/reader',
	granted_by_user_id VARCHAR(26) NOT NULL COMMENT '授权人用户ID',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
	UNIQUE KEY uk_document_permissions_doc_user (document_id, user_id),
	CONSTRAINT ck_document_permissions_role CHECK (role IN ('owner', 'collaborator', 'reader')),
	CONSTRAINT fk_document_permissions_document_id FOREIGN KEY (document_id) REFERENCES documents(document_id) ON DELETE CASCADE,
	CONSTRAINT fk_document_permissions_user_id FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
	CONSTRAINT fk_document_permissions_granted_by_user_id FOREIGN KEY (granted_by_user_id) REFERENCES users(user_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='文档权限表：文档级 ACL 授权关系';

CREATE INDEX idx_spaces_owner_user_id ON spaces(owner_user_id);
CREATE INDEX idx_spaces_updated_at ON spaces(updated_at);

CREATE INDEX idx_space_members_space_id ON space_members(space_id);
CREATE INDEX idx_space_members_user_id ON space_members(user_id);

CREATE INDEX idx_nodes_space_id ON nodes(space_id);
CREATE INDEX idx_nodes_parent_node_id ON nodes(parent_node_id);
CREATE INDEX idx_nodes_space_parent_sort ON nodes(space_id, parent_node_id, sort);
CREATE INDEX idx_nodes_type ON nodes(type);

CREATE INDEX idx_themes_updated_at ON themes(updated_at);

CREATE INDEX idx_documents_node_id ON documents(node_id);
CREATE INDEX idx_documents_theme_id ON documents(theme_id);
CREATE INDEX idx_documents_updated_at ON documents(updated_at);

CREATE INDEX idx_revisions_document_id ON document_revisions(document_id);
CREATE INDEX idx_revisions_document_version ON document_revisions(document_id, version);
CREATE INDEX idx_revisions_document_created_at ON document_revisions(document_id, created_at);

CREATE INDEX idx_node_permissions_node_id ON node_permissions(node_id);
CREATE INDEX idx_node_permissions_user_id ON node_permissions(user_id);

CREATE INDEX idx_document_permissions_document_id ON document_permissions(document_id);
CREATE INDEX idx_document_permissions_user_id ON document_permissions(user_id);
