package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/recordtime"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

type gormAuthCaptchaChallengeRepository struct {
	db *gorm.DB
}

type authCaptchaChallengeRow = authCaptchaChallengeRowDB

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
		Model(&models.AuthCaptchaChallenge{}).
		Select(
			models.AuthCaptchaChallengeColumns.ID,
			models.AuthCaptchaChallengeColumns.CaptchaID,
			models.AuthCaptchaChallengeColumns.Scene,
			models.AuthCaptchaChallengeColumns.SubjectHash,
			models.AuthCaptchaChallengeColumns.Level,
			models.AuthCaptchaChallengeColumns.AnswerHash,
			models.AuthCaptchaChallengeColumns.AnswerSalt,
			models.AuthCaptchaChallengeColumns.IssuedIPHash,
			models.AuthCaptchaChallengeColumns.ExpiresAt+" AS expires_at_raw",
			models.AuthCaptchaChallengeColumns.ConsumedAt+" AS consumed_at_raw",
			models.AuthCaptchaChallengeColumns.FailedVerifyCount,
			models.AuthCaptchaChallengeColumns.CreatedAt+" AS created_at_raw",
			models.AuthCaptchaChallengeColumns.UpdatedAt+" AS updated_at_raw",
		).
		Where(models.AuthCaptchaChallengeColumns.CaptchaID+" = ?", normalizedCaptchaID).
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
		Where(models.AuthCaptchaChallengeColumns.ID+" = ?", challenge.ID).
		Updates(map[string]any{
			models.AuthCaptchaChallengeColumns.ConsumedAt:        challenge.ConsumedAt,
			models.AuthCaptchaChallengeColumns.FailedVerifyCount: challenge.FailedVerifyCount,
			models.AuthCaptchaChallengeColumns.UpdatedAt:         now,
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
		ExpiresAt:         recordtime.Parse(row.ExpiresAtRaw),
		ConsumedAt:        recordtime.ParseNullable(row.ConsumedAtRaw),
		FailedVerifyCount: row.FailedVerifyCount,
		CreatedAt:         recordtime.Parse(row.CreatedAtRaw),
		UpdatedAt:         recordtime.Parse(row.UpdatedAtRaw),
	}
}
