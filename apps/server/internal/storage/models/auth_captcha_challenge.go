package models

import "time"

// AuthCaptchaChallenge 对应 auth_captcha_challenges 表，记录验证码生成基本信息与验证状态。
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

var AuthCaptchaChallengeColumns = struct {
	ID                string
	CaptchaID         string
	Scene             string
	SubjectHash       string
	Level             string
	AnswerHash        string
	AnswerSalt        string
	IssuedIPHash      string
	ExpiresAt         string
	ConsumedAt        string
	FailedVerifyCount string
	CreatedAt         string
	UpdatedAt         string
}{
	ID:                "id",
	CaptchaID:         "captcha_id",
	Scene:             "scene",
	SubjectHash:       "subject_hash",
	Level:             "level",
	AnswerHash:        "answer_hash",
	AnswerSalt:        "answer_salt",
	IssuedIPHash:      "issued_ip_hash",
	ExpiresAt:         "expires_at",
	ConsumedAt:        "consumed_at",
	FailedVerifyCount: "failed_verify_count",
	CreatedAt:         "created_at",
	UpdatedAt:         "updated_at",
}
