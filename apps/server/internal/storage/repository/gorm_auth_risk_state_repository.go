package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

type gormAuthRiskStateRepository struct {
	db *gorm.DB
}

type authRiskStateRow struct {
	ID                 int64   `gorm:"column:id"`
	Scene              string  `gorm:"column:scene"`
	SubjectType        string  `gorm:"column:subject_type"`
	SubjectHash        string  `gorm:"column:subject_hash"`
	WindowStartedAtRaw string  `gorm:"column:window_started_at"`
	AttemptCount       int     `gorm:"column:attempt_count"`
	FailedCount        int     `gorm:"column:failed_count"`
	CaptchaFailedCount int     `gorm:"column:captcha_failed_count"`
	LockUntilRaw       *string `gorm:"column:lock_until"`
	CreatedAtRaw       string  `gorm:"column:created_at"`
	UpdatedAtRaw       string  `gorm:"column:updated_at"`
}

// NewGormAuthRiskStateRepository 创建基于 GORM 的认证风控状态仓储实现。
func NewGormAuthRiskStateRepository(db *gorm.DB) AuthRiskStateRepository {
	return &gormAuthRiskStateRepository{db: db}
}

func (r *gormAuthRiskStateRepository) GetByKey(
	ctx context.Context,
	scene string,
	subjectType string,
	subjectHash string,
) (*models.AuthRiskState, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("auth risk state repository db is nil")
	}

	normalizedScene := strings.TrimSpace(scene)
	normalizedSubjectType := strings.TrimSpace(subjectType)
	normalizedSubjectHash := strings.TrimSpace(subjectHash)
	if normalizedScene == "" || normalizedSubjectType == "" || normalizedSubjectHash == "" {
		return nil, gorm.ErrRecordNotFound
	}

	var row authRiskStateRow
	if err := r.db.WithContext(ctx).
		Table("auth_risk_states").
		Select(
			"id",
			"scene",
			"subject_type",
			"subject_hash",
			"window_started_at",
			"attempt_count",
			"failed_count",
			"captcha_failed_count",
			"lock_until",
			"created_at",
			"updated_at",
		).
		Where(
			"scene = ? AND subject_type = ? AND subject_hash = ?",
			normalizedScene,
			normalizedSubjectType,
			normalizedSubjectHash,
		).
		Take(&row).Error; err != nil {
		return nil, err
	}
	state := mapAuthRiskStateRow(row)
	return &state, nil
}

func (r *gormAuthRiskStateRepository) Create(ctx context.Context, state *models.AuthRiskState) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("auth risk state repository db is nil")
	}
	if state == nil {
		return fmt.Errorf("auth risk state is nil")
	}
	return r.db.WithContext(ctx).Create(state).Error
}

func (r *gormAuthRiskStateRepository) Update(ctx context.Context, state *models.AuthRiskState) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("auth risk state repository db is nil")
	}
	if state == nil {
		return fmt.Errorf("auth risk state is nil")
	}
	if state.ID <= 0 {
		return fmt.Errorf("auth risk state id must be greater than 0")
	}

	now := state.UpdatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}

	return r.db.WithContext(ctx).
		Model(&models.AuthRiskState{}).
		Where("id = ?", state.ID).
		Updates(map[string]any{
			"window_started_at":    state.WindowStartedAt,
			"attempt_count":        state.AttemptCount,
			"failed_count":         state.FailedCount,
			"captcha_failed_count": state.CaptchaFailCount,
			"lock_until":           state.LockUntil,
			"updated_at":           now,
		}).Error
}

func mapAuthRiskStateRow(row authRiskStateRow) models.AuthRiskState {
	return models.AuthRiskState{
		ID:               row.ID,
		Scene:            row.Scene,
		SubjectType:      row.SubjectType,
		SubjectHash:      row.SubjectHash,
		WindowStartedAt:  parseRecordTime(row.WindowStartedAtRaw),
		AttemptCount:     row.AttemptCount,
		FailedCount:      row.FailedCount,
		CaptchaFailCount: row.CaptchaFailedCount,
		LockUntil:        parseNullableRecordTime(row.LockUntilRaw),
		CreatedAt:        parseRecordTime(row.CreatedAtRaw),
		UpdatedAt:        parseRecordTime(row.UpdatedAtRaw),
	}
}
