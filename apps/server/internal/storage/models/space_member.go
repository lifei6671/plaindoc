package models

import "time"

// SpaceMember 对应 space_members 表。
type SpaceMember struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	SpaceID   string    `gorm:"column:space_id"`
	UserID    string    `gorm:"column:user_id"`
	Role      Role      `gorm:"column:role"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (SpaceMember) TableName() string {
	return "space_members"
}

var SpaceMemberColumns = struct {
	ID        string
	SpaceID   string
	UserID    string
	Role      string
	CreatedAt string
	UpdatedAt string
}{
	ID:        "id",
	SpaceID:   "space_id",
	UserID:    "user_id",
	Role:      "role",
	CreatedAt: "created_at",
	UpdatedAt: "updated_at",
}
