package models

import "time"

// UserIdentity 对应 user_identities 表，记录本地用户与外部身份源的绑定关系。
type UserIdentity struct {
	ID           int64      `gorm:"column:id;primaryKey;autoIncrement"`
	UserID       string     `gorm:"column:user_id"`
	ProviderType string     `gorm:"column:provider_type"`
	ProviderID   string     `gorm:"column:provider_id"`
	ExternalID   string     `gorm:"column:external_id"`
	LoginName    string     `gorm:"column:login_name"`
	LastLoginAt  *time.Time `gorm:"column:last_login_at"`
	CreatedAt    time.Time  `gorm:"column:created_at"`
	UpdatedAt    time.Time  `gorm:"column:updated_at"`
}

func (UserIdentity) TableName() string {
	return "user_identities"
}

var UserIdentityColumns = struct {
	ID           string
	UserID       string
	ProviderType string
	ProviderID   string
	ExternalID   string
	LoginName    string
	LastLoginAt  string
	CreatedAt    string
	UpdatedAt    string
}{
	ID:           "id",
	UserID:       "user_id",
	ProviderType: "provider_type",
	ProviderID:   "provider_id",
	ExternalID:   "external_id",
	LoginName:    "login_name",
	LastLoginAt:  "last_login_at",
	CreatedAt:    "created_at",
	UpdatedAt:    "updated_at",
}
