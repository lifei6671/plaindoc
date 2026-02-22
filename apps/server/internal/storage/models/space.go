package models

import "time"

// Space 对应 spaces 表。
type Space struct {
	ID           int64        `gorm:"column:id;primaryKey;autoIncrement"`
	SpaceID      string       `gorm:"column:space_id"`
	Name         string       `gorm:"column:name"`
	Description  string       `gorm:"column:description"`
	CategoryID   string       `gorm:"column:category_id"`
	Category     string       `gorm:"column:category"`
	OwnerUserID  string       `gorm:"column:owner_user_id"`
	Visibility   Visibility   `gorm:"column:visibility"`
	CoverAssetID *string      `gorm:"column:cover_asset_id"`
	CoverKey     string       `gorm:"column:cover_key"`
	CoverURL     string       `gorm:"column:cover_url"`
	CoverWidth   int          `gorm:"column:cover_width"`
	CoverHeight  int          `gorm:"column:cover_height"`
	CoverSource  string       `gorm:"column:cover_source"`
	Status       EntityStatus `gorm:"column:status"`
	BannedReason string       `gorm:"column:banned_reason"`
	BannedAt     *time.Time   `gorm:"column:banned_at"`
	DeletedAt    *time.Time   `gorm:"column:deleted_at"`
	CreatedAt    time.Time    `gorm:"column:created_at"`
	UpdatedAt    time.Time    `gorm:"column:updated_at"`
}

func (Space) TableName() string {
	return "spaces"
}
