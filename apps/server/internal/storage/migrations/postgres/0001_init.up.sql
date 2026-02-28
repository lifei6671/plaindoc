CREATE TABLE IF NOT EXISTS users (
	id BIGSERIAL PRIMARY KEY,
	user_id VARCHAR(26) NOT NULL UNIQUE,
	email VARCHAR(320) NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	name VARCHAR(128) NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS spaces (
	id BIGSERIAL PRIMARY KEY,
	space_id VARCHAR(26) NOT NULL UNIQUE,
	name VARCHAR(255) NOT NULL,
	owner_user_id VARCHAR(26) NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS space_members (
	id BIGSERIAL PRIMARY KEY,
	space_id VARCHAR(26) NOT NULL REFERENCES spaces(space_id) ON DELETE CASCADE,
	user_id VARCHAR(26) NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
	role VARCHAR(16) NOT NULL CHECK (role IN ('owner', 'collaborator', 'reader')),
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(space_id, user_id)
);

CREATE TABLE IF NOT EXISTS nodes (
	id BIGSERIAL PRIMARY KEY,
	node_id VARCHAR(26) NOT NULL UNIQUE,
	space_id VARCHAR(26) NOT NULL REFERENCES spaces(space_id) ON DELETE CASCADE,
	parent_node_id VARCHAR(26) NULL REFERENCES nodes(node_id) ON DELETE CASCADE,
	type VARCHAR(16) NOT NULL CHECK (type IN ('folder', 'doc')),
	title VARCHAR(255) NOT NULL,
	sort INTEGER NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS themes (
	id BIGSERIAL PRIMARY KEY,
	theme_id VARCHAR(64) NOT NULL UNIQUE,
	name VARCHAR(128) NOT NULL,
	description TEXT NOT NULL,
	variables_json TEXT NOT NULL,
	syntax_theme VARCHAR(32) NOT NULL CHECK (syntax_theme IN ('one-light', 'one-dark')),
	code_block_style_json TEXT NOT NULL,
	code_block_code_style_json TEXT NOT NULL,
	inline_code_style_json TEXT NOT NULL,
	custom_css TEXT NOT NULL DEFAULT '',
	is_builtin BOOLEAN NOT NULL DEFAULT FALSE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

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
	TRUE
)
ON CONFLICT (theme_id) DO NOTHING;

CREATE TABLE IF NOT EXISTS documents (
	id BIGSERIAL PRIMARY KEY,
	document_id VARCHAR(26) NOT NULL UNIQUE,
	node_id VARCHAR(26) NOT NULL UNIQUE REFERENCES nodes(node_id) ON DELETE CASCADE,
	theme_id VARCHAR(64) NOT NULL DEFAULT 'default' REFERENCES themes(theme_id) ON DELETE RESTRICT,
	title VARCHAR(255) NOT NULL,
	content_md TEXT NOT NULL DEFAULT '',
	version INTEGER NOT NULL DEFAULT 1,
	updated_by_user_id VARCHAR(26) NULL REFERENCES users(user_id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS document_revisions (
	id BIGSERIAL PRIMARY KEY,
	document_revision_id VARCHAR(26) NOT NULL UNIQUE,
	document_id VARCHAR(26) NOT NULL REFERENCES documents(document_id) ON DELETE CASCADE,
	version INTEGER NOT NULL,
	content_md TEXT NOT NULL,
	base_version INTEGER NOT NULL,
	editor_user_id VARCHAR(26) NULL REFERENCES users(user_id) ON DELETE SET NULL,
	source VARCHAR(16) NOT NULL CHECK (source IN ('local', 'remote')),
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(document_id, version)
);

CREATE TABLE IF NOT EXISTS node_permissions (
	id BIGSERIAL PRIMARY KEY,
	node_id VARCHAR(26) NOT NULL REFERENCES nodes(node_id) ON DELETE CASCADE,
	user_id VARCHAR(26) NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
	role VARCHAR(16) NOT NULL CHECK (role IN ('owner', 'collaborator', 'reader')),
	granted_by_user_id VARCHAR(26) NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(node_id, user_id)
);

CREATE TABLE IF NOT EXISTS document_permissions (
	id BIGSERIAL PRIMARY KEY,
	document_id VARCHAR(26) NOT NULL REFERENCES documents(document_id) ON DELETE CASCADE,
	user_id VARCHAR(26) NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
	role VARCHAR(16) NOT NULL CHECK (role IN ('owner', 'collaborator', 'reader')),
	granted_by_user_id VARCHAR(26) NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(document_id, user_id)
);

COMMENT ON TABLE users IS '用户主表：存储账号基础信息';
COMMENT ON COLUMN users.id IS '主键ID';
COMMENT ON COLUMN users.user_id IS '用户业务ID（ULID）';
COMMENT ON COLUMN users.email IS '用户邮箱（全局唯一）';
COMMENT ON COLUMN users.password_hash IS '密码哈希（bcrypt）';
COMMENT ON COLUMN users.name IS '用户显示名称';
COMMENT ON COLUMN users.created_at IS '创建时间';
COMMENT ON COLUMN users.updated_at IS '更新时间';

COMMENT ON TABLE spaces IS '知识空间表：协作内容顶层容器';
COMMENT ON COLUMN spaces.id IS '主键ID';
COMMENT ON COLUMN spaces.space_id IS '空间业务ID（ULID）';
COMMENT ON COLUMN spaces.name IS '空间名称';
COMMENT ON COLUMN spaces.owner_user_id IS '空间所有者用户ID';
COMMENT ON COLUMN spaces.created_at IS '创建时间';
COMMENT ON COLUMN spaces.updated_at IS '更新时间';

COMMENT ON TABLE space_members IS '空间成员关系表：定义用户在空间中的协作角色';
COMMENT ON COLUMN space_members.id IS '主键ID';
COMMENT ON COLUMN space_members.space_id IS '空间业务ID';
COMMENT ON COLUMN space_members.user_id IS '用户业务ID';
COMMENT ON COLUMN space_members.role IS '空间角色：owner/collaborator/reader';
COMMENT ON COLUMN space_members.created_at IS '创建时间';
COMMENT ON COLUMN space_members.updated_at IS '更新时间';

COMMENT ON TABLE nodes IS '目录树节点表：空间内目录与文档节点';
COMMENT ON COLUMN nodes.id IS '主键ID';
COMMENT ON COLUMN nodes.node_id IS '节点业务ID（ULID）';
COMMENT ON COLUMN nodes.space_id IS '所属空间ID';
COMMENT ON COLUMN nodes.parent_node_id IS '父节点ID，空表示根节点';
COMMENT ON COLUMN nodes.type IS '节点类型：folder/doc';
COMMENT ON COLUMN nodes.title IS '节点标题';
COMMENT ON COLUMN nodes.sort IS '同级排序值';
COMMENT ON COLUMN nodes.created_at IS '创建时间';
COMMENT ON COLUMN nodes.updated_at IS '更新时间';

COMMENT ON TABLE themes IS '文档主题表：存储主题样式与代码高亮配置';
COMMENT ON COLUMN themes.id IS '主键ID';
COMMENT ON COLUMN themes.theme_id IS '主题业务ID';
COMMENT ON COLUMN themes.name IS '主题名称';
COMMENT ON COLUMN themes.description IS '主题描述';
COMMENT ON COLUMN themes.variables_json IS 'CSS 变量JSON';
COMMENT ON COLUMN themes.syntax_theme IS '代码高亮主题：one-light/one-dark';
COMMENT ON COLUMN themes.code_block_style_json IS '代码块容器样式JSON';
COMMENT ON COLUMN themes.code_block_code_style_json IS '代码块代码样式JSON';
COMMENT ON COLUMN themes.inline_code_style_json IS '行内代码样式JSON';
COMMENT ON COLUMN themes.custom_css IS '自定义CSS';
COMMENT ON COLUMN themes.is_builtin IS '是否内置主题';
COMMENT ON COLUMN themes.created_at IS '创建时间';
COMMENT ON COLUMN themes.updated_at IS '更新时间';

COMMENT ON TABLE documents IS '文档主表：节点对应的正文';
COMMENT ON COLUMN documents.id IS '主键ID';
COMMENT ON COLUMN documents.document_id IS '文档业务ID（ULID）';
COMMENT ON COLUMN documents.node_id IS '关联节点ID（唯一）';
COMMENT ON COLUMN documents.theme_id IS '主题ID';
COMMENT ON COLUMN documents.title IS '文档标题';
COMMENT ON COLUMN documents.content_md IS 'Markdown 正文';
COMMENT ON COLUMN documents.version IS '文档版本号（乐观锁）';
COMMENT ON COLUMN documents.updated_by_user_id IS '最后更新人用户ID';
COMMENT ON COLUMN documents.created_at IS '创建时间';
COMMENT ON COLUMN documents.updated_at IS '更新时间';

COMMENT ON TABLE document_revisions IS '文档修订历史表：保存版本快照与来源';
COMMENT ON COLUMN document_revisions.id IS '主键ID';
COMMENT ON COLUMN document_revisions.document_revision_id IS '修订业务ID（ULID）';
COMMENT ON COLUMN document_revisions.document_id IS '文档业务ID';
COMMENT ON COLUMN document_revisions.version IS '修订版本号';
COMMENT ON COLUMN document_revisions.content_md IS '该版本 Markdown 内容';
COMMENT ON COLUMN document_revisions.base_version IS '保存时基准版本号';
COMMENT ON COLUMN document_revisions.editor_user_id IS '编辑人用户ID';
COMMENT ON COLUMN document_revisions.source IS '修订来源：local/remote';
COMMENT ON COLUMN document_revisions.created_at IS '创建时间';

COMMENT ON TABLE node_permissions IS '节点权限表：节点级 ACL 授权关系';
COMMENT ON COLUMN node_permissions.id IS '主键ID';
COMMENT ON COLUMN node_permissions.node_id IS '节点业务ID';
COMMENT ON COLUMN node_permissions.user_id IS '被授权用户ID';
COMMENT ON COLUMN node_permissions.role IS '授权角色：owner/collaborator/reader';
COMMENT ON COLUMN node_permissions.granted_by_user_id IS '授权人用户ID';
COMMENT ON COLUMN node_permissions.created_at IS '创建时间';
COMMENT ON COLUMN node_permissions.updated_at IS '更新时间';

COMMENT ON TABLE document_permissions IS '文档权限表：文档级 ACL 授权关系';
COMMENT ON COLUMN document_permissions.id IS '主键ID';
COMMENT ON COLUMN document_permissions.document_id IS '文档业务ID';
COMMENT ON COLUMN document_permissions.user_id IS '被授权用户ID';
COMMENT ON COLUMN document_permissions.role IS '授权角色：owner/collaborator/reader';
COMMENT ON COLUMN document_permissions.granted_by_user_id IS '授权人用户ID';
COMMENT ON COLUMN document_permissions.created_at IS '创建时间';
COMMENT ON COLUMN document_permissions.updated_at IS '更新时间';

CREATE INDEX IF NOT EXISTS idx_spaces_owner_user_id ON spaces(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_spaces_updated_at ON spaces(updated_at);

CREATE INDEX IF NOT EXISTS idx_space_members_space_id ON space_members(space_id);
CREATE INDEX IF NOT EXISTS idx_space_members_user_id ON space_members(user_id);

CREATE INDEX IF NOT EXISTS idx_nodes_space_id ON nodes(space_id);
CREATE INDEX IF NOT EXISTS idx_nodes_parent_node_id ON nodes(parent_node_id);
CREATE INDEX IF NOT EXISTS idx_nodes_space_parent_sort ON nodes(space_id, parent_node_id, sort);
CREATE INDEX IF NOT EXISTS idx_nodes_type ON nodes(type);

CREATE INDEX IF NOT EXISTS idx_themes_updated_at ON themes(updated_at);

CREATE INDEX IF NOT EXISTS idx_documents_node_id ON documents(node_id);
CREATE INDEX IF NOT EXISTS idx_documents_theme_id ON documents(theme_id);
CREATE INDEX IF NOT EXISTS idx_documents_updated_at ON documents(updated_at);

CREATE INDEX IF NOT EXISTS idx_revisions_document_id ON document_revisions(document_id);
CREATE INDEX IF NOT EXISTS idx_revisions_document_version ON document_revisions(document_id, version);
CREATE INDEX IF NOT EXISTS idx_revisions_document_created_at ON document_revisions(document_id, created_at);

CREATE INDEX IF NOT EXISTS idx_node_permissions_node_id ON node_permissions(node_id);
CREATE INDEX IF NOT EXISTS idx_node_permissions_user_id ON node_permissions(user_id);

CREATE INDEX IF NOT EXISTS idx_document_permissions_document_id ON document_permissions(document_id);
CREATE INDEX IF NOT EXISTS idx_document_permissions_user_id ON document_permissions(user_id);
