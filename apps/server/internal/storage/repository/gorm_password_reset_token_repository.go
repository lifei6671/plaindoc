package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/recordtime"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

type gormPasswordResetTokenRepository struct {
	db *gorm.DB
}

type passwordResetTokenRow = passwordResetTokenRowDB

// NewGormPasswordResetTokenRepository 创建基于 GORM 的密码重置令牌仓储实现。
func NewGormPasswordResetTokenRepository(db *gorm.DB) PasswordResetTokenRepository {
	return &gormPasswordResetTokenRepository{db: db}
}

func (r *gormPasswordResetTokenRepository) Create(
	ctx context.Context,
	token *models.PasswordResetToken,
) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("password reset token repository db is nil")
	}
	if token == nil {
		return fmt.Errorf("password reset token is nil")
	}
	return r.db.WithContext(ctx).Create(token).Error
}

func (r *gormPasswordResetTokenRepository) GetByTokenID(
	ctx context.Context,
	tokenID string,
) (*models.PasswordResetToken, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("password reset token repository db is nil")
	}
	normalizedTokenID := strings.TrimSpace(tokenID)
	if normalizedTokenID == "" {
		return nil, gorm.ErrRecordNotFound
	}

	var row passwordResetTokenRow
	if err := r.db.WithContext(ctx).
		Model(&models.PasswordResetToken{}).
		Select(
			models.PasswordResetTokenColumns.ID,
			models.PasswordResetTokenColumns.TokenID,
			models.PasswordResetTokenColumns.TokenSecretHash,
			models.PasswordResetTokenColumns.UserID,
			models.PasswordResetTokenColumns.Source,
			models.PasswordResetTokenColumns.RequestedByUserID,
			models.PasswordResetTokenColumns.RequestIPHash,
			models.PasswordResetTokenColumns.ExpiresAt+" AS expires_at_raw",
			models.PasswordResetTokenColumns.ConsumedAt+" AS consumed_at_raw",
			models.PasswordResetTokenColumns.InvalidatedAt+" AS invalidated_at_raw",
			models.PasswordResetTokenColumns.CreatedAt+" AS created_at_raw",
			models.PasswordResetTokenColumns.UpdatedAt+" AS updated_at_raw",
		).
		Where(models.PasswordResetTokenColumns.TokenID+" = ?", normalizedTokenID).
		Take(&row).Error; err != nil {
		return nil, err
	}
	result := mapPasswordResetTokenRow(row)
	return &result, nil
}

func (r *gormPasswordResetTokenRepository) CountRecent(
	ctx context.Context,
	params CountPasswordResetTokensParams,
) (int64, error) {
	if r == nil || r.db == nil {
		return 0, fmt.Errorf("password reset token repository db is nil")
	}
	query := r.db.WithContext(ctx).Model(&models.PasswordResetToken{})
	if normalizedUserID := strings.TrimSpace(params.UserID); normalizedUserID != "" {
		query = query.Where(models.PasswordResetTokenColumns.UserID+" = ?", normalizedUserID)
	}
	if normalizedRequestIPHash := strings.TrimSpace(params.RequestIPHash); normalizedRequestIPHash != "" {
		query = query.Where(models.PasswordResetTokenColumns.RequestIPHash+" = ?", normalizedRequestIPHash)
	}
	if !params.Since.IsZero() {
		query = query.Where(models.PasswordResetTokenColumns.CreatedAt+" >= ?", params.Since.UTC())
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (r *gormPasswordResetTokenRepository) InvalidateActiveByUserID(
	ctx context.Context,
	params InvalidatePasswordResetTokensParams,
) (int64, error) {
	if r == nil || r.db == nil {
		return 0, fmt.Errorf("password reset token repository db is nil")
	}
	normalizedUserID := strings.TrimSpace(params.UserID)
	if normalizedUserID == "" {
		return 0, nil
	}
	invalidatedAt := params.InvalidatedAt
	if invalidatedAt.IsZero() {
		invalidatedAt = time.Now().UTC()
	}

	tx := r.db.WithContext(ctx).
		Model(&models.PasswordResetToken{}).
		Where(models.PasswordResetTokenColumns.UserID+" = ?", normalizedUserID).
		Where(models.PasswordResetTokenColumns.ConsumedAt + " IS NULL").
		Where(models.PasswordResetTokenColumns.InvalidatedAt + " IS NULL").
		Updates(map[string]any{
			models.PasswordResetTokenColumns.InvalidatedAt: invalidatedAt,
			models.PasswordResetTokenColumns.UpdatedAt:     invalidatedAt,
		})
	if tx.Error != nil {
		return 0, tx.Error
	}
	return tx.RowsAffected, nil
}

func (r *gormPasswordResetTokenRepository) Consume(
	ctx context.Context,
	params ConsumePasswordResetTokenParams,
) (*models.PasswordResetToken, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("password reset token repository db is nil")
	}

	tokenID := strings.TrimSpace(params.TokenID)
	tokenSecretHash := strings.TrimSpace(params.TokenSecretHash)
	if tokenID == "" || tokenSecretHash == "" {
		return nil, gorm.ErrRecordNotFound
	}

	now := params.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	consumedAt := params.ConsumedAt
	if consumedAt.IsZero() {
		consumedAt = now
	}

	var result models.PasswordResetToken
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row passwordResetTokenRow
		if err := tx.Model(&models.PasswordResetToken{}).
			Select(
				models.PasswordResetTokenColumns.ID,
				models.PasswordResetTokenColumns.TokenID,
				models.PasswordResetTokenColumns.TokenSecretHash,
				models.PasswordResetTokenColumns.UserID,
				models.PasswordResetTokenColumns.Source,
				models.PasswordResetTokenColumns.RequestedByUserID,
				models.PasswordResetTokenColumns.RequestIPHash,
				models.PasswordResetTokenColumns.ExpiresAt+" AS expires_at_raw",
				models.PasswordResetTokenColumns.ConsumedAt+" AS consumed_at_raw",
				models.PasswordResetTokenColumns.InvalidatedAt+" AS invalidated_at_raw",
				models.PasswordResetTokenColumns.CreatedAt+" AS created_at_raw",
				models.PasswordResetTokenColumns.UpdatedAt+" AS updated_at_raw",
			).
			Where(models.PasswordResetTokenColumns.TokenID+" = ?", tokenID).
			Where(models.PasswordResetTokenColumns.TokenSecretHash+" = ?", tokenSecretHash).
			Where(models.PasswordResetTokenColumns.ConsumedAt+" IS NULL").
			Where(models.PasswordResetTokenColumns.InvalidatedAt+" IS NULL").
			Where(models.PasswordResetTokenColumns.ExpiresAt+" > ?", now).
			Take(&row).Error; err != nil {
			return err
		}

		updateTx := tx.Model(&models.PasswordResetToken{}).
			Where(models.PasswordResetTokenColumns.ID+" = ?", row.ID).
			Where(models.PasswordResetTokenColumns.ConsumedAt+" IS NULL").
			Where(models.PasswordResetTokenColumns.InvalidatedAt+" IS NULL").
			Where(models.PasswordResetTokenColumns.ExpiresAt+" > ?", now).
			Updates(map[string]any{
				models.PasswordResetTokenColumns.ConsumedAt: consumedAt,
				models.PasswordResetTokenColumns.UpdatedAt:  consumedAt,
			})
		if updateTx.Error != nil {
			return updateTx.Error
		}
		if updateTx.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}

		mapped := mapPasswordResetTokenRow(row)
		mapped.ConsumedAt = &consumedAt
		mapped.UpdatedAt = consumedAt
		result = mapped
		return nil
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &result, nil
}

func mapPasswordResetTokenRow(row passwordResetTokenRow) models.PasswordResetToken {
	return models.PasswordResetToken{
		ID:                row.ID,
		TokenID:           row.TokenID,
		TokenSecretHash:   row.TokenSecretHash,
		UserID:            row.UserID,
		Source:            row.Source,
		RequestedByUserID: row.RequestedByUserID,
		RequestIPHash:     row.RequestIPHash,
		ExpiresAt:         recordtime.Parse(row.ExpiresAtRaw),
		ConsumedAt:        recordtime.ParseNullable(row.ConsumedAtRaw),
		InvalidatedAt:     recordtime.ParseNullable(row.InvalidatedAtRaw),
		CreatedAt:         recordtime.Parse(row.CreatedAtRaw),
		UpdatedAt:         recordtime.Parse(row.UpdatedAtRaw),
	}
}
