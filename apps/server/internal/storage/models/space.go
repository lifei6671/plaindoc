package models

import "time"

// Space 对应 spaces 表。
type Space struct {
	ID           int64        `gorm:"column:id;primaryKey;autoIncrement"`
	SpaceID      string       `gorm:"column:space_id"`
	Name         string       `gorm:"column:name"`
	OwnerUserID  string       `gorm:"column:owner_user_id"`
	Visibility   Visibility   `gorm:"column:visibility"`
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
