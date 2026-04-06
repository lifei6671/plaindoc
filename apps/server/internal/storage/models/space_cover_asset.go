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

var SpaceCoverAssetColumns = struct {
	ID              string
	AssetID         string
	Source          string
	ObjectKey       string
	ObjectURL       string
	MimeType        string
	Width           string
	Height          string
	SizeBytes       string
	Normalized      string
	CreatedByUserID string
	CreatedAt       string
	UpdatedAt       string
}{
	ID:              "id",
	AssetID:         "asset_id",
	Source:          "source",
	ObjectKey:       "object_key",
	ObjectURL:       "object_url",
	MimeType:        "mime_type",
	Width:           "width",
	Height:          "height",
	SizeBytes:       "size_bytes",
	Normalized:      "normalized",
	CreatedByUserID: "created_by_user_id",
	CreatedAt:       "created_at",
	UpdatedAt:       "updated_at",
}
