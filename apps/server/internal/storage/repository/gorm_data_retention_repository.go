package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
)

const defaultDataRetentionDeleteBatchSize = 500

type gormDataRetentionRepository struct {
	db *gorm.DB
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
		"audit_logs",
		batchSize,
		func(query *gorm.DB) *gorm.DB {
			return query.Where("created_at < ?", cutoff)
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
		"auth_captcha_challenges",
		batchSize,
		func(query *gorm.DB) *gorm.DB {
			return query.Where("expires_at < ?", cutoff)
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
		"auth_risk_states",
		batchSize,
		func(query *gorm.DB) *gorm.DB {
			return query.Where("updated_at < ? AND (lock_until IS NULL OR lock_until < ?)", cutoff, cutoff)
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
		"user_sessions",
		batchSize,
		func(query *gorm.DB) *gorm.DB {
			return query.Where("(expires_at < ?) OR (revoked_at IS NOT NULL AND revoked_at < ?)", cutoff, cutoff)
		},
	)
	if err != nil {
		return deleted, fmt.Errorf("cleanup user_sessions failed: %w", err)
	}
	return deleted, nil
}

func (r *gormDataRetentionRepository) deleteRowsByID(
	ctx context.Context,
	tableName string,
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
		query := r.db.WithContext(ctx).Table(tableName).Select("id").Order("id ASC").Limit(batchSize)
		if filterBuilder != nil {
			query = filterBuilder(query)
		}

		ids := make([]int64, 0, batchSize)
		if err := query.Pluck("id", &ids).Error; err != nil {
			return totalDeleted, err
		}
		if len(ids) == 0 {
			break
		}

		deleteTx := r.db.WithContext(ctx).Table(tableName).Where("id IN ?", ids).Delete(nil)
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
