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
			Select("session_id", "user_id", "refresh_token_hash", "expires_at", "revoked_at").
			Where("session_id = ? AND user_id = ?", params.CurrentSessionID, params.UserID).
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
			Where("session_id = ? AND user_id = ? AND revoked_at IS NULL", params.CurrentSessionID, params.UserID).
			Updates(map[string]any{
				"revoked_at":             params.Now,
				"replaced_by_session_id": params.NextSessionID,
				"updated_at":             params.Now,
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
		Where("session_id = ? AND user_id = ? AND revoked_at IS NULL", sessionID, userID).
		Updates(map[string]any{
			"revoked_at": revokedAt,
			"updated_at": revokedAt,
		}).Error
}
