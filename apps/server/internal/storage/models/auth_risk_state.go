package models

import "time"

// AuthRiskState 对应 auth_risk_states 表，记录认证风控计数与封禁状态。
type AuthRiskState struct {
	ID               int64      `gorm:"column:id;primaryKey;autoIncrement"`
	Scene            string     `gorm:"column:scene"`
	SubjectType      string     `gorm:"column:subject_type"`
	SubjectHash      string     `gorm:"column:subject_hash"`
	WindowStartedAt  time.Time  `gorm:"column:window_started_at"`
	AttemptCount     int        `gorm:"column:attempt_count"`
	FailedCount      int        `gorm:"column:failed_count"`
	CaptchaFailCount int        `gorm:"column:captcha_failed_count"`
	LockUntil        *time.Time `gorm:"column:lock_until"`
	CreatedAt        time.Time  `gorm:"column:created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at"`
}

func (AuthRiskState) TableName() string {
	return "auth_risk_states"
}
