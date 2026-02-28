package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

type gormAuthCaptchaChallengeRepository struct {
	db *gorm.DB
}

type authCaptchaChallengeRow struct {
	ID                int64   `gorm:"column:id"`
	CaptchaID         string  `gorm:"column:captcha_id"`
	Scene             string  `gorm:"column:scene"`
	SubjectHash       string  `gorm:"column:subject_hash"`
	Level             int     `gorm:"column:level"`
	AnswerHash        string  `gorm:"column:answer_hash"`
	AnswerSalt        string  `gorm:"column:answer_salt"`
	IssuedIPHash      string  `gorm:"column:issued_ip_hash"`
	ExpiresAtRaw      string  `gorm:"column:expires_at"`
	ConsumedAtRaw     *string `gorm:"column:consumed_at"`
	FailedVerifyCount int     `gorm:"column:failed_verify_count"`
	CreatedAtRaw      string  `gorm:"column:created_at"`
	UpdatedAtRaw      string  `gorm:"column:updated_at"`
}

// NewGormAuthCaptchaChallengeRepository 创建基于 GORM 的验证码会话仓储实现。
func NewGormAuthCaptchaChallengeRepository(db *gorm.DB) AuthCaptchaChallengeRepository {
	return &gormAuthCaptchaChallengeRepository{db: db}
}

func (r *gormAuthCaptchaChallengeRepository) GetByCaptchaID(
	ctx context.Context,
	captchaID string,
) (*models.AuthCaptchaChallenge, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("auth captcha challenge repository db is nil")
	}

	normalizedCaptchaID := strings.TrimSpace(captchaID)
	if normalizedCaptchaID == "" {
		return nil, gorm.ErrRecordNotFound
	}

	var row authCaptchaChallengeRow
	if err := r.db.WithContext(ctx).
		Table("auth_captcha_challenges").
		Select(
			"id",
			"captcha_id",
			"scene",
			"subject_hash",
			"level",
			"answer_hash",
			"answer_salt",
			"issued_ip_hash",
			"expires_at",
			"consumed_at",
			"failed_verify_count",
			"created_at",
			"updated_at",
		).
		Where("captcha_id = ?", normalizedCaptchaID).
		Take(&row).Error; err != nil {
		return nil, err
	}
	challenge := mapAuthCaptchaChallengeRow(row)
	return &challenge, nil
}

func (r *gormAuthCaptchaChallengeRepository) Create(
	ctx context.Context,
	challenge *models.AuthCaptchaChallenge,
) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("auth captcha challenge repository db is nil")
	}
	if challenge == nil {
		return fmt.Errorf("auth captcha challenge is nil")
	}
	return r.db.WithContext(ctx).Create(challenge).Error
}

func (r *gormAuthCaptchaChallengeRepository) Update(
	ctx context.Context,
	challenge *models.AuthCaptchaChallenge,
) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("auth captcha challenge repository db is nil")
	}
	if challenge == nil {
		return fmt.Errorf("auth captcha challenge is nil")
	}
	if challenge.ID <= 0 {
		return fmt.Errorf("auth captcha challenge id must be greater than 0")
	}

	now := challenge.UpdatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}

	return r.db.WithContext(ctx).
		Model(&models.AuthCaptchaChallenge{}).
		Where("id = ?", challenge.ID).
		Updates(map[string]any{
			"consumed_at":         challenge.ConsumedAt,
			"failed_verify_count": challenge.FailedVerifyCount,
			"updated_at":          now,
		}).Error
}

func mapAuthCaptchaChallengeRow(row authCaptchaChallengeRow) models.AuthCaptchaChallenge {
	return models.AuthCaptchaChallenge{
		ID:                row.ID,
		CaptchaID:         row.CaptchaID,
		Scene:             row.Scene,
		SubjectHash:       row.SubjectHash,
		Level:             row.Level,
		AnswerHash:        row.AnswerHash,
		AnswerSalt:        row.AnswerSalt,
		IssuedIPHash:      row.IssuedIPHash,
		ExpiresAt:         parseRecordTime(row.ExpiresAtRaw),
		ConsumedAt:        parseNullableRecordTime(row.ConsumedAtRaw),
		FailedVerifyCount: row.FailedVerifyCount,
		CreatedAt:         parseRecordTime(row.CreatedAtRaw),
		UpdatedAt:         parseRecordTime(row.UpdatedAtRaw),
	}
}
