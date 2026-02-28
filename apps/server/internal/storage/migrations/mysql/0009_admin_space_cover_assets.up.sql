ALTER TABLE spaces
	ADD COLUMN description VARCHAR(1024) NOT NULL DEFAULT '',
	ADD COLUMN cover_asset_id VARCHAR(26) NULL,
	ADD COLUMN cover_key VARCHAR(512) NOT NULL DEFAULT '',
	ADD COLUMN cover_url TEXT NOT NULL,
	ADD COLUMN cover_width INT NOT NULL DEFAULT 0,
	ADD COLUMN cover_height INT NOT NULL DEFAULT 0,
	ADD COLUMN cover_source VARCHAR(32) NOT NULL DEFAULT '',
	ADD CONSTRAINT ck_spaces_cover_source CHECK (cover_source IN ('', 'user_upload', 'system_generated'));

CREATE TABLE IF NOT EXISTS space_cover_assets (
	id BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '主键ID',
	asset_id VARCHAR(26) NOT NULL UNIQUE COMMENT '素材业务ID（ULID）',
	source VARCHAR(32) NOT NULL COMMENT '素材来源：user_upload/system_generated',
	object_key VARCHAR(512) NOT NULL COMMENT '对象存储键',
	object_url TEXT NOT NULL COMMENT '对象访问URL',
	mime_type VARCHAR(64) NOT NULL COMMENT '文件MIME类型',
	width INT NOT NULL COMMENT '图片宽度（像素）',
	height INT NOT NULL COMMENT '图片高度（像素）',
	size_bytes BIGINT NOT NULL COMMENT '文件字节大小',
	normalized TINYINT(1) NOT NULL DEFAULT 1 COMMENT '是否已完成尺寸与格式归一化',
	created_by_user_id VARCHAR(26) NOT NULL COMMENT '创建人用户ID',
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
	CONSTRAINT ck_space_cover_assets_source CHECK (source IN ('user_upload', 'system_generated')),
	CONSTRAINT ck_space_cover_assets_normalized CHECK (normalized IN (0, 1)),
	CONSTRAINT fk_space_cover_assets_created_by_user_id FOREIGN KEY (created_by_user_id) REFERENCES users(user_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='空间封面素材表：记录封面源文件元数据';

ALTER TABLE spaces
	ADD CONSTRAINT fk_spaces_cover_asset_id FOREIGN KEY (cover_asset_id) REFERENCES space_cover_assets(asset_id) ON DELETE SET NULL;

CREATE INDEX idx_spaces_cover_asset_id ON spaces(cover_asset_id);
CREATE INDEX idx_space_cover_assets_source ON space_cover_assets(source);
