package models

import "time"

// AuthCaptchaChallenge 对应 auth_captcha_challenges 表，记录验证码挑战与消费状态。
type AuthCaptchaChallenge struct {
	ID                int64      `gorm:"column:id;primaryKey;autoIncrement"`
	CaptchaID         string     `gorm:"column:captcha_id"`
	Scene             string     `gorm:"column:scene"`
	SubjectHash       string     `gorm:"column:subject_hash"`
	Level             int        `gorm:"column:level"`
	AnswerHash        string     `gorm:"column:answer_hash"`
	AnswerSalt        string     `gorm:"column:answer_salt"`
	IssuedIPHash      string     `gorm:"column:issued_ip_hash"`
	ExpiresAt         time.Time  `gorm:"column:expires_at"`
	ConsumedAt        *time.Time `gorm:"column:consumed_at"`
	FailedVerifyCount int        `gorm:"column:failed_verify_count"`
	CreatedAt         time.Time  `gorm:"column:created_at"`
	UpdatedAt         time.Time  `gorm:"column:updated_at"`
}

func (AuthCaptchaChallenge) TableName() string {
	return "auth_captcha_challenges"
}
