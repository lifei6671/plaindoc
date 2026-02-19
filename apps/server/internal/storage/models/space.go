package models

import "time"

// Space 对应 spaces 表。
type Space struct {
	ID          int64     `gorm:"column:id;primaryKey;autoIncrement"`
	SpaceID     string    `gorm:"column:space_id"`
	Name        string    `gorm:"column:name"`
	OwnerUserID string    `gorm:"column:owner_user_id"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (Space) TableName() string {
	return "spaces"
}
