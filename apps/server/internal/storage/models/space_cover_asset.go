package models

import "time"

// SpaceCoverAsset 对应 space_cover_assets 表。
type SpaceCoverAsset struct {
	ID              int64     `gorm:"column:id;primaryKey;autoIncrement"`
	AssetID         string    `gorm:"column:asset_id"`
	Source          string    `gorm:"column:source"`
	ObjectKey       string    `gorm:"column:object_key"`
	ObjectURL       string    `gorm:"column:object_url"`
	MimeType        string    `gorm:"column:mime_type"`
	Width           int       `gorm:"column:width"`
	Height          int       `gorm:"column:height"`
	SizeBytes       int64     `gorm:"column:size_bytes"`
	Normalized      bool      `gorm:"column:normalized"`
	CreatedByUserID string    `gorm:"column:created_by_user_id"`
	CreatedAt       time.Time `gorm:"column:created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at"`
}

func (SpaceCoverAsset) TableName() string {
	return "space_cover_assets"
}
