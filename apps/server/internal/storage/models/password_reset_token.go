package models

import "time"

// PasswordResetToken 对应 password_reset_tokens 表，记录密码重置令牌生命周期。
type PasswordResetToken struct {
	ID                int64      `gorm:"column:id;primaryKey;autoIncrement"`
	TokenID           string     `gorm:"column:token_id"`
	TokenSecretHash   string     `gorm:"column:token_secret_hash"`
	UserID            string     `gorm:"column:user_id"`
	Source            string     `gorm:"column:source"`
	RequestedByUserID *string    `gorm:"column:requested_by_user_id"`
	RequestIPHash     string     `gorm:"column:request_ip_hash"`
	ExpiresAt         time.Time  `gorm:"column:expires_at"`
	ConsumedAt        *time.Time `gorm:"column:consumed_at"`
	InvalidatedAt     *time.Time `gorm:"column:invalidated_at"`
	CreatedAt         time.Time  `gorm:"column:created_at"`
	UpdatedAt         time.Time  `gorm:"column:updated_at"`
}

func (PasswordResetToken) TableName() string {
	return "password_reset_tokens"
}

var PasswordResetTokenColumns = struct {
	ID                string
	TokenID           string
	TokenSecretHash   string
	UserID            string
	Source            string
	RequestedByUserID string
	RequestIPHash     string
	ExpiresAt         string
	ConsumedAt        string
	InvalidatedAt     string
	CreatedAt         string
	UpdatedAt         string
}{
	ID:                "id",
	TokenID:           "token_id",
	TokenSecretHash:   "token_secret_hash",
	UserID:            "user_id",
	Source:            "source",
	RequestedByUserID: "requested_by_user_id",
	RequestIPHash:     "request_ip_hash",
	ExpiresAt:         "expires_at",
	ConsumedAt:        "consumed_at",
	InvalidatedAt:     "invalidated_at",
	CreatedAt:         "created_at",
	UpdatedAt:         "updated_at",
}
