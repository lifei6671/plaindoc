package models

import "time"

// SpaceAdminScope 对应 space_admin_scopes 表。
type SpaceAdminScope struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	UserID    string    `gorm:"column:user_id"`
	SpaceID   string    `gorm:"column:space_id"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (SpaceAdminScope) TableName() string {
	return "space_admin_scopes"
}
