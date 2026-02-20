package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
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
		Table("space_admin_scopes").
		Where("user_id = ? AND space_id = ?", userID, spaceID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
