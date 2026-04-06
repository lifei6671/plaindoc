package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

const defaultDataRetentionDeleteBatchSize = 500

type gormDataRetentionRepository struct {
	db *gorm.DB
}

type dataRetentionTableMeta struct {
	tableName  string
	idColumn   string
	timeColumn string
}

// NewGormDataRetentionRepository 创建数据保留清理仓储实现。
func NewGormDataRetentionRepository(db *gorm.DB) DataRetentionRepository {
	return &gormDataRetentionRepository{db: db}
}

func (r *gormDataRetentionRepository) DeleteAuditLogsBefore(
	ctx context.Context,
	cutoff time.Time,
	batchSize int,
) (int64, error) {
	deleted, err := r.deleteRowsByID(
		ctx,
		dataRetentionTableMeta{
			tableName:  (models.AuditLog{}).TableName(),
			idColumn:   models.AuditLogColumns.ID,
			timeColumn: models.AuditLogColumns.CreatedAt,
		},
		batchSize,
		func(query *gorm.DB) *gorm.DB {
			return query.Where(models.AuditLogColumns.CreatedAt+" < ?", cutoff)
		},
	)
	if err != nil {
		return deleted, fmt.Errorf("cleanup audit_logs failed: %w", err)
	}
	return deleted, nil
}

func (r *gormDataRetentionRepository) DeleteAuthCaptchaChallengesBefore(
	ctx context.Context,
	cutoff time.Time,
	batchSize int,
) (int64, error) {
	deleted, err := r.deleteRowsByID(
		ctx,
		dataRetentionTableMeta{
			tableName:  (models.AuthCaptchaChallenge{}).TableName(),
			idColumn:   models.AuthCaptchaChallengeColumns.ID,
			timeColumn: models.AuthCaptchaChallengeColumns.ExpiresAt,
		},
		batchSize,
		func(query *gorm.DB) *gorm.DB {
			return query.Where(models.AuthCaptchaChallengeColumns.ExpiresAt+" < ?", cutoff)
		},
	)
	if err != nil {
		return deleted, fmt.Errorf("cleanup auth_captcha_challenges failed: %w", err)
	}
	return deleted, nil
}

func (r *gormDataRetentionRepository) DeleteAuthRiskStatesBefore(
	ctx context.Context,
	cutoff time.Time,
	batchSize int,
) (int64, error) {
	deleted, err := r.deleteRowsByID(
		ctx,
		dataRetentionTableMeta{
			tableName:  (models.AuthRiskState{}).TableName(),
			idColumn:   models.AuthRiskStateColumns.ID,
			timeColumn: models.AuthRiskStateColumns.UpdatedAt,
		},
		batchSize,
		func(query *gorm.DB) *gorm.DB {
			return query.
				Where(models.AuthRiskStateColumns.UpdatedAt+" < ?", cutoff).
				Where(
					models.AuthRiskStateColumns.LockUntil+" IS NULL OR "+models.AuthRiskStateColumns.LockUntil+" < ?",
					cutoff,
				)
		},
	)
	if err != nil {
		return deleted, fmt.Errorf("cleanup auth_risk_states failed: %w", err)
	}
	return deleted, nil
}

func (r *gormDataRetentionRepository) DeleteUserSessionsBefore(
	ctx context.Context,
	cutoff time.Time,
	batchSize int,
) (int64, error) {
	deleted, err := r.deleteRowsByID(
		ctx,
		dataRetentionTableMeta{
			tableName:  (models.UserSession{}).TableName(),
			idColumn:   models.UserSessionColumns.ID,
			timeColumn: models.UserSessionColumns.ExpiresAt,
		},
		batchSize,
		func(query *gorm.DB) *gorm.DB {
			return query.
				Where(models.UserSessionColumns.ExpiresAt+" < ?", cutoff).
				Or(
					models.UserSessionColumns.RevokedAt+" IS NOT NULL AND "+models.UserSessionColumns.RevokedAt+" < ?",
					cutoff,
				)
		},
	)
	if err != nil {
		return deleted, fmt.Errorf("cleanup user_sessions failed: %w", err)
	}
	return deleted, nil
}

func (r *gormDataRetentionRepository) deleteRowsByID(
	ctx context.Context,
	tableMeta dataRetentionTableMeta,
	batchSize int,
	filterBuilder func(query *gorm.DB) *gorm.DB,
) (int64, error) {
	if r == nil || r.db == nil {
		return 0, fmt.Errorf("data retention repository db is nil")
	}
	if batchSize <= 0 {
		batchSize = defaultDataRetentionDeleteBatchSize
	}

	var totalDeleted int64
	for {
		query := r.db.WithContext(ctx).
			Table(tableMeta.tableName).
			Select(tableMeta.idColumn).
			Order(tableMeta.idColumn + " ASC").
			Limit(batchSize)
		if filterBuilder != nil {
			query = filterBuilder(query)
		}

		ids := make([]int64, 0, batchSize)
		if err := query.Pluck(tableMeta.idColumn, &ids).Error; err != nil {
			return totalDeleted, err
		}
		if len(ids) == 0 {
			break
		}

		deleteTx := r.db.WithContext(ctx).
			Table(tableMeta.tableName).
			Where(tableMeta.idColumn+" IN ?", ids).
			Delete(nil)
		if deleteTx.Error != nil {
			return totalDeleted, deleteTx.Error
		}

		totalDeleted += deleteTx.RowsAffected
		if len(ids) < batchSize {
			break
		}
	}
	return totalDeleted, nil
}
