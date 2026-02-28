package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/captchastore"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	captchaStoreScene             = "store"
	captchaStoreSubjectHash       = "store"
	captchaStoreAnswerSalt        = "-"
	captchaStoreIssuedIPHash      = "-"
	captchaStoreDefaultLevel      = 4
	captchaStoreIDPrefix          = "s:"
	captchaStoreIDMaxLength       = 32
	captchaStoreIDHashHexLength   = 30
	captchaStoreValueMaxLength    = 128
	captchaStoreFallbackKeepYears = 100
)

var _ captchastore.DatabaseRepository = (*gormCaptchaStoreRepository)(nil)

type gormCaptchaStoreRepository struct {
	db    *gorm.DB
	nowFn func() time.Time
}

type captchaStoreRow struct {
	CaptchaID    string `gorm:"column:captcha_id"`
	AnswerHash   string `gorm:"column:answer_hash"`
	ExpiresAtRaw string `gorm:"column:expires_at"`
	CreatedAtRaw string `gorm:"column:created_at"`
	UpdatedAtRaw string `gorm:"column:updated_at"`
}

// NewGormCaptchaStoreRepository 创建基于 auth_captcha_challenges 表的验证码存储仓储。
func NewGormCaptchaStoreRepository(db *gorm.DB) captchastore.DatabaseRepository {
	return &gormCaptchaStoreRepository{
		db: db,
		nowFn: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (r *gormCaptchaStoreRepository) UpsertCaptchaRecord(
	ctx context.Context,
	record captchastore.Record,
) error {
	if r == nil || r.db == nil {
		return errors.New("captcha store repository db is nil")
	}

	normalizedID := strings.TrimSpace(record.ID)
	if normalizedID == "" {
		return errors.New("captcha store id is required")
	}
	value := strings.TrimSpace(record.Value)
	if value == "" {
		return errors.New("captcha store value is required")
	}
	if len(value) > captchaStoreValueMaxLength {
		return fmt.Errorf("captcha store value length must be <= %d", captchaStoreValueMaxLength)
	}

	now := r.currentTime()
	createdAt := record.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = now
	}
	updatedAt := record.UpdatedAt.UTC()
	if updatedAt.IsZero() {
		updatedAt = now
	}
	expiresAt := record.ExpiresAt.UTC()
	if expiresAt.IsZero() {
		expiresAt = now.AddDate(captchaStoreFallbackKeepYears, 0, 0)
	}

	storedCaptchaID := mapCaptchaStoreID(normalizedID)
	row := &models.AuthCaptchaChallenge{
		CaptchaID:         storedCaptchaID,
		Scene:             captchaStoreScene,
		SubjectHash:       captchaStoreSubjectHash,
		Level:             captchaStoreDefaultLevel,
		AnswerHash:        value,
		AnswerSalt:        captchaStoreAnswerSalt,
		IssuedIPHash:      captchaStoreIssuedIPHash,
		ExpiresAt:         expiresAt,
		ConsumedAt:        nil,
		FailedVerifyCount: 0,
		CreatedAt:         createdAt,
		UpdatedAt:         updatedAt,
	}

	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "captcha_id"},
			},
			DoUpdates: clause.Assignments(map[string]any{
				"scene":               captchaStoreScene,
				"subject_hash":        captchaStoreSubjectHash,
				"level":               captchaStoreDefaultLevel,
				"answer_hash":         value,
				"answer_salt":         captchaStoreAnswerSalt,
				"issued_ip_hash":      captchaStoreIssuedIPHash,
				"expires_at":          expiresAt,
				"consumed_at":         nil,
				"failed_verify_count": 0,
				"updated_at":          updatedAt,
			}),
		}).
		Create(row).Error
}

func (r *gormCaptchaStoreRepository) GetCaptchaRecordByID(
	ctx context.Context,
	id string,
) (captchastore.Record, bool, error) {
	if r == nil || r.db == nil {
		return captchastore.Record{}, false, errors.New("captcha store repository db is nil")
	}
	normalizedID := strings.TrimSpace(id)
	if normalizedID == "" {
		return captchastore.Record{}, false, nil
	}

	var row captchaStoreRow
	err := r.db.WithContext(ctx).
		Table("auth_captcha_challenges").
		Select("captcha_id", "answer_hash", "expires_at", "created_at", "updated_at").
		Where("captcha_id = ? AND scene = ?", mapCaptchaStoreID(normalizedID), captchaStoreScene).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return captchastore.Record{}, false, nil
		}
		return captchastore.Record{}, false, err
	}
	return captchastore.Record{
		ID:        normalizedID,
		Value:     row.AnswerHash,
		ExpiresAt: parseRecordTime(row.ExpiresAtRaw),
		CreatedAt: parseRecordTime(row.CreatedAtRaw),
		UpdatedAt: parseRecordTime(row.UpdatedAtRaw),
	}, true, nil
}

func (r *gormCaptchaStoreRepository) DeleteCaptchaRecordByID(
	ctx context.Context,
	id string,
) error {
	if r == nil || r.db == nil {
		return errors.New("captcha store repository db is nil")
	}
	normalizedID := strings.TrimSpace(id)
	if normalizedID == "" {
		return nil
	}
	return r.db.WithContext(ctx).
		Where("captcha_id = ? AND scene = ?", mapCaptchaStoreID(normalizedID), captchaStoreScene).
		Delete(&models.AuthCaptchaChallenge{}).Error
}

func (r *gormCaptchaStoreRepository) currentTime() time.Time {
	if r == nil || r.nowFn == nil {
		return time.Now().UTC()
	}
	return r.nowFn().UTC()
}

func mapCaptchaStoreID(rawID string) string {
	normalizedID := strings.TrimSpace(rawID)
	if normalizedID == "" {
		return ""
	}

	candidate := captchaStoreIDPrefix + normalizedID
	if len(candidate) <= captchaStoreIDMaxLength {
		return candidate
	}

	sum := sha256.Sum256([]byte(normalizedID))
	hexEncoded := hex.EncodeToString(sum[:])
	if len(hexEncoded) > captchaStoreIDHashHexLength {
		hexEncoded = hexEncoded[:captchaStoreIDHashHexLength]
	}
	return captchaStoreIDPrefix + hexEncoded
}
