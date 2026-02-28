ALTER TABLE spaces
	ADD COLUMN description TEXT NOT NULL DEFAULT '',
	ADD COLUMN cover_asset_id VARCHAR(26) NULL,
	ADD COLUMN cover_key VARCHAR(512) NOT NULL DEFAULT '',
	ADD COLUMN cover_url TEXT NOT NULL DEFAULT '',
	ADD COLUMN cover_width INTEGER NOT NULL DEFAULT 0,
	ADD COLUMN cover_height INTEGER NOT NULL DEFAULT 0,
	ADD COLUMN cover_source VARCHAR(32) NOT NULL DEFAULT '';
COMMENT ON COLUMN spaces.description IS '空间描述';
COMMENT ON COLUMN spaces.cover_asset_id IS '封面素材ID（关联 space_cover_assets.asset_id）';
COMMENT ON COLUMN spaces.cover_key IS '封面对象存储键';
COMMENT ON COLUMN spaces.cover_url IS '封面访问URL';
COMMENT ON COLUMN spaces.cover_width IS '封面宽度（像素）';
COMMENT ON COLUMN spaces.cover_height IS '封面高度（像素）';
COMMENT ON COLUMN spaces.cover_source IS '封面来源：空/用户上传/系统生成';

ALTER TABLE spaces
	ADD CONSTRAINT ck_spaces_cover_source
	CHECK (cover_source IN ('', 'user_upload', 'system_generated'));

CREATE TABLE IF NOT EXISTS space_cover_assets (
	id BIGSERIAL PRIMARY KEY,
	asset_id VARCHAR(26) NOT NULL UNIQUE,
	source VARCHAR(32) NOT NULL CHECK (source IN ('user_upload', 'system_generated')),
	object_key VARCHAR(512) NOT NULL,
	object_url TEXT NOT NULL,
	mime_type VARCHAR(64) NOT NULL,
	width INTEGER NOT NULL,
	height INTEGER NOT NULL,
	size_bytes BIGINT NOT NULL,
	normalized BOOLEAN NOT NULL DEFAULT TRUE,
	created_by_user_id VARCHAR(26) NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

COMMENT ON TABLE space_cover_assets IS '空间封面素材表：记录封面源文件元数据';
COMMENT ON COLUMN space_cover_assets.id IS '主键ID';
COMMENT ON COLUMN space_cover_assets.asset_id IS '素材业务ID（ULID）';
COMMENT ON COLUMN space_cover_assets.source IS '素材来源：user_upload/system_generated';
COMMENT ON COLUMN space_cover_assets.object_key IS '对象存储键';
COMMENT ON COLUMN space_cover_assets.object_url IS '对象访问URL';
COMMENT ON COLUMN space_cover_assets.mime_type IS '文件MIME类型';
COMMENT ON COLUMN space_cover_assets.width IS '图片宽度（像素）';
COMMENT ON COLUMN space_cover_assets.height IS '图片高度（像素）';
COMMENT ON COLUMN space_cover_assets.size_bytes IS '文件字节大小';
COMMENT ON COLUMN space_cover_assets.normalized IS '是否已完成尺寸与格式归一化';
COMMENT ON COLUMN space_cover_assets.created_by_user_id IS '创建人用户ID';
COMMENT ON COLUMN space_cover_assets.created_at IS '创建时间';
COMMENT ON COLUMN space_cover_assets.updated_at IS '更新时间';

ALTER TABLE spaces
	ADD CONSTRAINT fk_spaces_cover_asset_id
	FOREIGN KEY (cover_asset_id) REFERENCES space_cover_assets(asset_id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_spaces_cover_asset_id ON spaces(cover_asset_id);
CREATE INDEX IF NOT EXISTS idx_space_cover_assets_source ON space_cover_assets(source);
