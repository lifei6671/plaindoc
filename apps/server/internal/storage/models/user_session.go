package models

import "time"

// UserSession 对应 user_sessions 表。
type UserSession struct {
	ID                  int64      `gorm:"column:id;primaryKey;autoIncrement"`
	SessionID           string     `gorm:"column:session_id"`
	UserID              string     `gorm:"column:user_id"`
	RefreshTokenHash    string     `gorm:"column:refresh_token_hash"`
	ExpiresAt           time.Time  `gorm:"column:expires_at"`
	RevokedAt           *time.Time `gorm:"column:revoked_at"`
	ReplacedBySessionID *string    `gorm:"column:replaced_by_session_id"`
	CreatedAt           time.Time  `gorm:"column:created_at"`
	UpdatedAt           time.Time  `gorm:"column:updated_at"`
}

func (UserSession) TableName() string {
	return "user_sessions"
}
