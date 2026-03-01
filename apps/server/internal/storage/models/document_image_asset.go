package models

import "time"

// DocumentImageAsset 对应 document_image_assets 表。
// 该表用于追踪文档中引用的图片资源，支持“待清理”标记与定时清理。
type DocumentImageAsset struct {
	ID               int64      `gorm:"column:id;primaryKey;autoIncrement"`
	ImageAssetID     string     `gorm:"column:image_asset_id"`
	DocumentID       string     `gorm:"column:document_id"`
	SpaceID          string     `gorm:"column:space_id"`
	StorageProvider  string     `gorm:"column:storage_provider"`
	ObjectKey        string     `gorm:"column:object_key"`
	ObjectURL        string     `gorm:"column:object_url"`
	Status           string     `gorm:"column:status"`
	PendingCleanupAt *time.Time `gorm:"column:pending_cleanup_at"`
	DeletedAt        *time.Time `gorm:"column:deleted_at"`
	LastReferencedAt time.Time  `gorm:"column:last_referenced_at"`
	CreatedAt        time.Time  `gorm:"column:created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at"`
}

func (DocumentImageAsset) TableName() string {
	return "document_image_assets"
}
