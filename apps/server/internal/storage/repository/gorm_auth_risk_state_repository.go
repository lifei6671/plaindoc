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

type gormAuthRiskStateRepository struct {
	db *gorm.DB
}

type authRiskStateRow = authRiskStateRowDB

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
		Model(&models.AuthRiskState{}).
		Select(
			models.AuthRiskStateColumns.ID,
			models.AuthRiskStateColumns.Scene,
			models.AuthRiskStateColumns.SubjectType,
			models.AuthRiskStateColumns.SubjectHash,
			models.AuthRiskStateColumns.WindowStartedAt+" AS window_started_at_raw",
			models.AuthRiskStateColumns.AttemptCount,
			models.AuthRiskStateColumns.FailedCount,
			models.AuthRiskStateColumns.CaptchaFailCount+" AS captcha_failed_count",
			models.AuthRiskStateColumns.LockUntil+" AS lock_until_raw",
			models.AuthRiskStateColumns.CreatedAt+" AS created_at_raw",
			models.AuthRiskStateColumns.UpdatedAt+" AS updated_at_raw",
		).
		Where(models.AuthRiskStateColumns.Scene+" = ?", normalizedScene).
		Where(models.AuthRiskStateColumns.SubjectType+" = ?", normalizedSubjectType).
		Where(models.AuthRiskStateColumns.SubjectHash+" = ?", normalizedSubjectHash).
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
		Where(models.AuthRiskStateColumns.ID+" = ?", state.ID).
		Updates(map[string]any{
			models.AuthRiskStateColumns.WindowStartedAt:  state.WindowStartedAt,
			models.AuthRiskStateColumns.AttemptCount:     state.AttemptCount,
			models.AuthRiskStateColumns.FailedCount:      state.FailedCount,
			models.AuthRiskStateColumns.CaptchaFailCount: state.CaptchaFailCount,
			models.AuthRiskStateColumns.LockUntil:        state.LockUntil,
			models.AuthRiskStateColumns.UpdatedAt:        now,
		}).Error
}

func mapAuthRiskStateRow(row authRiskStateRow) models.AuthRiskState {
	return models.AuthRiskState{
		ID:               row.ID,
		Scene:            row.Scene,
		SubjectType:      row.SubjectType,
		SubjectHash:      row.SubjectHash,
		WindowStartedAt:  recordtime.Parse(row.WindowStartedAtRaw),
		AttemptCount:     row.AttemptCount,
		FailedCount:      row.FailedCount,
		CaptchaFailCount: row.CaptchaFailedCount,
		LockUntil:        recordtime.ParseNullable(row.LockUntilRaw),
		CreatedAt:        recordtime.Parse(row.CreatedAtRaw),
		UpdatedAt:        recordtime.Parse(row.UpdatedAtRaw),
	}
}
