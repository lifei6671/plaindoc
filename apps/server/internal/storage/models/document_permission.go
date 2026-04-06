package models

import "time"

// DocumentPermission 对应 document_permissions 表。
type DocumentPermission struct {
	ID              int64     `gorm:"column:id;primaryKey;autoIncrement"`
	DocumentID      string    `gorm:"column:document_id"`
	UserID          string    `gorm:"column:user_id"`
	Role            Role      `gorm:"column:role"`
	GrantedByUserID string    `gorm:"column:granted_by_user_id"`
	CreatedAt       time.Time `gorm:"column:created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at"`
}

func (DocumentPermission) TableName() string {
	return "document_permissions"
}

var DocumentPermissionColumns = struct {
	ID              string
	DocumentID      string
	UserID          string
	Role            string
	GrantedByUserID string
	CreatedAt       string
	UpdatedAt       string
}{
	ID:              "id",
	DocumentID:      "document_id",
	UserID:          "user_id",
	Role:            "role",
	GrantedByUserID: "granted_by_user_id",
	CreatedAt:       "created_at",
	UpdatedAt:       "updated_at",
}
