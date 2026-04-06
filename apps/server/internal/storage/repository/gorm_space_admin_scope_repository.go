package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
)

type gormSpaceAdminScopeRepository struct {
	db *gorm.DB
}

// NewGormSpaceAdminScopeRepository 创建基于 GORM 的空间管理范围仓储实现。
func NewGormSpaceAdminScopeRepository(db *gorm.DB) SpaceAdminScopeRepository {
	return &gormSpaceAdminScopeRepository{db: db}
}

func (r *gormSpaceAdminScopeRepository) HasScope(
	ctx context.Context,
	userID string,
	spaceID string,
) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("space admin scope repository db is nil")
	}
	if userID == "" || spaceID == "" {
		return false, nil
	}

	var count int64
	if err := r.db.WithContext(ctx).
		Model(&models.SpaceAdminScope{}).
		Where(models.SpaceAdminScopeColumns.UserID+" = ?", userID).
		Where(models.SpaceAdminScopeColumns.SpaceID+" = ?", spaceID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *gormSpaceAdminScopeRepository) UpsertScope(
	ctx context.Context,
	userID string,
	spaceID string,
) error {
	// 关键函数：确保空间管理范围存在（转让/授权场景）。
	if r == nil || r.db == nil {
		return fmt.Errorf("space admin scope repository db is nil")
	}
	if userID == "" || spaceID == "" {
		return nil
	}

	now := time.Now().UTC()
	return r.db.WithContext(ctx).
		Model(&models.SpaceAdminScope{}).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: models.SpaceAdminScopeColumns.UserID},
				{Name: models.SpaceAdminScopeColumns.SpaceID},
			},
			DoUpdates: clause.Assignments(map[string]any{
				models.SpaceAdminScopeColumns.UpdatedAt: now,
			}),
		}).
		Create(&models.SpaceAdminScope{
			UserID:    userID,
			SpaceID:   spaceID,
			CreatedAt: now,
			UpdatedAt: now,
		}).Error
}

func (r *gormSpaceAdminScopeRepository) DeleteScope(
	ctx context.Context,
	userID string,
	spaceID string,
) error {
	// 关键函数：移除空间管理范围（转让降级场景）。
	if r == nil || r.db == nil {
		return fmt.Errorf("space admin scope repository db is nil")
	}
	if userID == "" || spaceID == "" {
		return nil
	}
	return r.db.WithContext(ctx).
		Model(&models.SpaceAdminScope{}).
		Where(models.SpaceAdminScopeColumns.UserID+" = ?", userID).
		Where(models.SpaceAdminScopeColumns.SpaceID+" = ?", spaceID).
		Delete(&models.SpaceAdminScope{}).Error
}

func (r *gormSpaceAdminScopeRepository) ListByUserID(
	ctx context.Context,
	userID string,
) ([]string, error) {
	// 关键函数：获取用户所有管理范围，用于判断是否保留管理员角色。
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("space admin scope repository db is nil")
	}
	if userID == "" {
		return []string{}, nil
	}

	var rows []models.SpaceAdminScope
	if err := r.db.WithContext(ctx).
		Model(&models.SpaceAdminScope{}).
		Select(models.SpaceAdminScopeColumns.SpaceID).
		Where(models.SpaceAdminScopeColumns.UserID+" = ?", userID).
		Order(clause.OrderByColumn{
			Column: clause.Column{Name: models.SpaceAdminScopeColumns.SpaceID},
		}).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	spaceIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		spaceID := row.SpaceID
		if spaceID == "" {
			continue
		}
		spaceIDs = append(spaceIDs, spaceID)
	}
	return spaceIDs, nil
}
