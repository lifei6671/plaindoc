package models

import "time"

// User 对应 users 表。
type User struct {
	ID           int64        `gorm:"column:id;primaryKey;autoIncrement"`
	UserID       string       `gorm:"column:user_id"`
	Email        string       `gorm:"column:email"`
	PasswordHash string       `gorm:"column:password_hash"`
	Name         string       `gorm:"column:name"`
	AvatarURL    string       `gorm:"column:avatar_url"`
	Status       EntityStatus `gorm:"column:status"`
	BannedReason string       `gorm:"column:banned_reason"`
	BannedAt     *time.Time   `gorm:"column:banned_at"`
	DeletedAt    *time.Time   `gorm:"column:deleted_at"`
	CreatedAt    time.Time    `gorm:"column:created_at"`
	UpdatedAt    time.Time    `gorm:"column:updated_at"`
}

func (User) TableName() string {
	return "users"
}
