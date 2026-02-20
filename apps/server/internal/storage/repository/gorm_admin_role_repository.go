package repository

import (
	"context"
	"fmt"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

type gormAdminRoleRepository struct {
	db *gorm.DB
}

// NewGormAdminRoleRepository 创建基于 GORM 的管理角色仓储实现。
func NewGormAdminRoleRepository(db *gorm.DB) AdminRoleRepository {
	return &gormAdminRoleRepository{db: db}
}

func (r *gormAdminRoleRepository) HasRole(
	ctx context.Context,
	userID string,
	role models.AdminRole,
) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("admin role repository db is nil")
	}
	if userID == "" || !models.IsValidAdminRole(role) {
		return false, nil
	}

	var count int64
	if err := r.db.WithContext(ctx).
		Table("user_admin_roles").
		Where("user_id = ? AND role = ?", userID, role).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *gormAdminRoleRepository) ListByUserID(ctx context.Context, userID string) ([]models.AdminRole, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("admin role repository db is nil")
	}
	if userID == "" {
		return []models.AdminRole{}, nil
	}

	var rows []struct {
		Role string `gorm:"column:role"`
	}
	if err := r.db.WithContext(ctx).
		Table("user_admin_roles").
		Select("role").
		Where("user_id = ?", userID).
		Order("role ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	roles := make([]models.AdminRole, 0, len(rows))
	for _, row := range rows {
		role := models.AdminRole(row.Role)
		if !models.IsValidAdminRole(role) {
			continue
		}
		roles = append(roles, role)
	}
	return roles, nil
}
