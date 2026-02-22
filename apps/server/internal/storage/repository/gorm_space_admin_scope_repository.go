package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
		Table("space_admin_scopes").
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "space_id"}},
			DoUpdates: clause.Assignments(map[string]any{"updated_at": now}),
		}).
		Create(map[string]any{
			"user_id":    userID,
			"space_id":   spaceID,
			"created_at": now,
			"updated_at": now,
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
		Table("space_admin_scopes").
		Where("user_id = ? AND space_id = ?", userID, spaceID).
		Delete(nil).Error
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

	type scopeRow struct {
		SpaceID string `gorm:"column:space_id"`
	}
	var rows []scopeRow
	if err := r.db.WithContext(ctx).
		Table("space_admin_scopes").
		Select("space_id").
		Where("user_id = ?", userID).
		Order("space_id ASC").
		Scan(&rows).Error; err != nil {
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
