package models

import "time"

// DocumentShare 对应 document_shares 表。
type DocumentShare struct {
	ID              int64             `gorm:"column:id;primaryKey;autoIncrement"`
	ShareID         string            `gorm:"column:share_id"`
	DocumentID      string            `gorm:"column:document_id"`
	SpaceID         string            `gorm:"column:space_id"`
	Mode            DocumentShareMode `gorm:"column:mode"`
	PasswordHash    *string           `gorm:"column:password_hash"`
	PasswordHint    string            `gorm:"column:password_hint"`
	ExpiresAt       *time.Time        `gorm:"column:expires_at"`
	DisabledAt      *time.Time        `gorm:"column:disabled_at"`
	AccessVersion   int               `gorm:"column:access_version"`
	CreatedByUserID *string           `gorm:"column:created_by_user_id"`
	UpdatedByUserID *string           `gorm:"column:updated_by_user_id"`
	CreatedAt       time.Time         `gorm:"column:created_at"`
	UpdatedAt       time.Time         `gorm:"column:updated_at"`
}

func (DocumentShare) TableName() string {
	return "document_shares"
}
