package models

import "time"

// User 对应 users 表。
type User struct {
	ID           int64     `gorm:"column:id;primaryKey;autoIncrement"`
	UserID       string    `gorm:"column:user_id"`
	Email        string    `gorm:"column:email"`
	PasswordHash string    `gorm:"column:password_hash"`
	Name         string    `gorm:"column:name"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

func (User) TableName() string {
	return "users"
}
