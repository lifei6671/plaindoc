package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

type gormUserSessionRepository struct {
	db *gorm.DB
}

// NewGormUserSessionRepository 创建基于 GORM 的会话仓储实现。
func NewGormUserSessionRepository(db *gorm.DB) UserSessionRepository {
	return &gormUserSessionRepository{db: db}
}

func (r *gormUserSessionRepository) Create(ctx context.Context, session *models.UserSession) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("user session repository db is nil")
	}
	return r.db.WithContext(ctx).Create(session).Error
}

func (r *gormUserSessionRepository) Rotate(ctx context.Context, params RotateUserSessionParams) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("user session repository db is nil")
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current models.UserSession
		if err := tx.
			Model(&models.UserSession{}).
			Select(
				models.UserSessionColumns.SessionID,
				models.UserSessionColumns.UserID,
				models.UserSessionColumns.RefreshTokenHash,
				models.UserSessionColumns.ExpiresAt,
				models.UserSessionColumns.RevokedAt,
			).
			Where(models.UserSessionColumns.SessionID+" = ?", params.CurrentSessionID).
			Where(models.UserSessionColumns.UserID+" = ?", params.UserID).
			Take(&current).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInvalidSession
			}
			return err
		}

		if current.RefreshTokenHash != params.CurrentRefreshTokenHash {
			return ErrInvalidSession
		}
		if current.RevokedAt != nil {
			return ErrInvalidSession
		}
		if !current.ExpiresAt.After(params.Now) {
			return ErrInvalidSession
		}

		nextSession := &models.UserSession{
			SessionID:        params.NextSessionID,
			UserID:           params.UserID,
			RefreshTokenHash: params.NextRefreshTokenHash,
			ExpiresAt:        params.NextExpiresAt,
			CreatedAt:        params.Now,
			UpdatedAt:        params.Now,
		}
		if err := tx.Create(nextSession).Error; err != nil {
			return err
		}

		revokeTx := tx.Model(&models.UserSession{}).
			Where(models.UserSessionColumns.SessionID+" = ?", params.CurrentSessionID).
			Where(models.UserSessionColumns.UserID+" = ?", params.UserID).
			Where(models.UserSessionColumns.RevokedAt + " IS NULL").
			Updates(map[string]any{
				models.UserSessionColumns.RevokedAt:           params.Now,
				models.UserSessionColumns.ReplacedBySessionID: params.NextSessionID,
				models.UserSessionColumns.UpdatedAt:           params.Now,
			})
		if revokeTx.Error != nil {
			return revokeTx.Error
		}
		if revokeTx.RowsAffected != 1 {
			return ErrInvalidSession
		}

		return nil
	})
}

func (r *gormUserSessionRepository) Revoke(
	ctx context.Context,
	userID string,
	sessionID string,
	revokedAt time.Time,
) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("user session repository db is nil")
	}

	return r.db.WithContext(ctx).
		Model(&models.UserSession{}).
		Where(models.UserSessionColumns.SessionID+" = ?", sessionID).
		Where(models.UserSessionColumns.UserID+" = ?", userID).
		Where(models.UserSessionColumns.RevokedAt + " IS NULL").
		Updates(map[string]any{
			models.UserSessionColumns.RevokedAt: revokedAt,
			models.UserSessionColumns.UpdatedAt: revokedAt,
		}).Error
}

func (r *gormUserSessionRepository) RevokeAllByUserID(
	ctx context.Context,
	userID string,
	revokedAt time.Time,
) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("user session repository db is nil")
	}
	if userID == "" {
		return nil
	}
	if revokedAt.IsZero() {
		revokedAt = time.Now().UTC()
	}

	return r.db.WithContext(ctx).
		Model(&models.UserSession{}).
		Where(models.UserSessionColumns.UserID+" = ?", userID).
		Where(models.UserSessionColumns.RevokedAt + " IS NULL").
		Updates(map[string]any{
			models.UserSessionColumns.RevokedAt: revokedAt,
			models.UserSessionColumns.UpdatedAt: revokedAt,
		}).Error
}
