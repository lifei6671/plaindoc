package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

type gormPasswordResetTokenRepository struct {
	db *gorm.DB
}

type passwordResetTokenRow struct {
	ID                   int64   `gorm:"column:id"`
	TokenID              string  `gorm:"column:token_id"`
	TokenSecretHash      string  `gorm:"column:token_secret_hash"`
	UserID               string  `gorm:"column:user_id"`
	Source               string  `gorm:"column:source"`
	RequestedByUserID    *string `gorm:"column:requested_by_user_id"`
	RequestIPHash        string  `gorm:"column:request_ip_hash"`
	ExpiresAtRaw         string  `gorm:"column:expires_at"`
	ConsumedAtRaw        *string `gorm:"column:consumed_at"`
	InvalidatedAtRaw     *string `gorm:"column:invalidated_at"`
	CreatedAtRaw         string  `gorm:"column:created_at"`
	UpdatedAtRaw         string  `gorm:"column:updated_at"`
}

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
		Table("password_reset_tokens").
		Select(
			"id",
			"token_id",
			"token_secret_hash",
			"user_id",
			"source",
			"requested_by_user_id",
			"request_ip_hash",
			"expires_at",
			"consumed_at",
			"invalidated_at",
			"created_at",
			"updated_at",
		).
		Where("token_id = ?", normalizedTokenID).
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
		query = query.Where("user_id = ?", normalizedUserID)
	}
	if normalizedRequestIPHash := strings.TrimSpace(params.RequestIPHash); normalizedRequestIPHash != "" {
		query = query.Where("request_ip_hash = ?", normalizedRequestIPHash)
	}
	if !params.Since.IsZero() {
		query = query.Where("created_at >= ?", params.Since.UTC())
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
		Where("user_id = ? AND consumed_at IS NULL AND invalidated_at IS NULL", normalizedUserID).
		Updates(map[string]any{
			"invalidated_at": invalidatedAt,
			"updated_at":     invalidatedAt,
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
		if err := tx.Table("password_reset_tokens").
			Select(
				"id",
				"token_id",
				"token_secret_hash",
				"user_id",
				"source",
				"requested_by_user_id",
				"request_ip_hash",
				"expires_at",
				"consumed_at",
				"invalidated_at",
				"created_at",
				"updated_at",
			).
			Where(
				"token_id = ? AND token_secret_hash = ? AND consumed_at IS NULL AND invalidated_at IS NULL AND expires_at > ?",
				tokenID,
				tokenSecretHash,
				now,
			).
			Take(&row).Error; err != nil {
			return err
		}

		updateTx := tx.Model(&models.PasswordResetToken{}).
			Where("id = ? AND consumed_at IS NULL AND invalidated_at IS NULL AND expires_at > ?", row.ID, now).
			Updates(map[string]any{
				"consumed_at": consumedAt,
				"updated_at":  consumedAt,
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
		ExpiresAt:         parseRecordTime(row.ExpiresAtRaw),
		ConsumedAt:        parseNullableRecordTime(row.ConsumedAtRaw),
		InvalidatedAt:     parseNullableRecordTime(row.InvalidatedAtRaw),
		CreatedAt:         parseRecordTime(row.CreatedAtRaw),
		UpdatedAt:         parseRecordTime(row.UpdatedAtRaw),
	}
}
