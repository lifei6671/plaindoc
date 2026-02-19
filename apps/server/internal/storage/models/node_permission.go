package models

import "time"

// NodePermission 对应 node_permissions 表。
type NodePermission struct {
	ID              int64     `gorm:"column:id;primaryKey;autoIncrement"`
	NodeID          string    `gorm:"column:node_id"`
	UserID          string    `gorm:"column:user_id"`
	Role            Role      `gorm:"column:role"`
	GrantedByUserID string    `gorm:"column:granted_by_user_id"`
	CreatedAt       time.Time `gorm:"column:created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at"`
}

func (NodePermission) TableName() string {
	return "node_permissions"
}
