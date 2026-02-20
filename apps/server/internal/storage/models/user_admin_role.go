package models

import "time"

// UserAdminRole 对应 user_admin_roles 表。
type UserAdminRole struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	UserID    string    `gorm:"column:user_id"`
	Role      AdminRole `gorm:"column:role"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (UserAdminRole) TableName() string {
	return "user_admin_roles"
}
