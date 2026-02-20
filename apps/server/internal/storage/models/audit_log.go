package models

import "time"

// AuditLog 对应 audit_logs 表。
type AuditLog struct {
	ID          int64     `gorm:"column:id;primaryKey;autoIncrement"`
	ActorUserID *string   `gorm:"column:actor_user_id"`
	Module      string    `gorm:"column:module"`
	Action      string    `gorm:"column:action"`
	TargetType  string    `gorm:"column:target_type"`
	TargetID    string    `gorm:"column:target_id"`
	Summary     string    `gorm:"column:summary"`
	DetailJSON  string    `gorm:"column:detail_json"`
	RequestID   string    `gorm:"column:request_id"`
	CreatedAt   time.Time `gorm:"column:created_at"`
}

func (AuditLog) TableName() string {
	return "audit_logs"
}
