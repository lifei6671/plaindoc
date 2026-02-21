package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

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

func (r *gormAdminRoleRepository) ListByUserIDs(
	ctx context.Context,
	userIDs []string,
) (map[string][]models.AdminRole, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("admin role repository db is nil")
	}

	normalizedUserIDs := make([]string, 0, len(userIDs))
	seenUserIDs := make(map[string]struct{}, len(userIDs))
	for _, userID := range userIDs {
		normalizedUserID := strings.TrimSpace(userID)
		if normalizedUserID == "" {
			continue
		}
		if _, exists := seenUserIDs[normalizedUserID]; exists {
			continue
		}
		seenUserIDs[normalizedUserID] = struct{}{}
		normalizedUserIDs = append(normalizedUserIDs, normalizedUserID)
	}
	if len(normalizedUserIDs) == 0 {
		return map[string][]models.AdminRole{}, nil
	}

	var rows []struct {
		UserID string `gorm:"column:user_id"`
		Role   string `gorm:"column:role"`
	}
	if err := r.db.WithContext(ctx).
		Table("user_admin_roles").
		Select("user_id", "role").
		Where("user_id IN ?", normalizedUserIDs).
		Order("user_id ASC, role ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	results := make(map[string][]models.AdminRole, len(normalizedUserIDs))
	for _, userID := range normalizedUserIDs {
		results[userID] = []models.AdminRole{}
	}

	for _, row := range rows {
		role := models.AdminRole(row.Role)
		if !models.IsValidAdminRole(role) {
			continue
		}
		results[row.UserID] = append(results[row.UserID], role)
	}
	return results, nil
}

func (r *gormAdminRoleRepository) ReplaceByUserID(
	ctx context.Context,
	userID string,
	roles []models.AdminRole,
) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("admin role repository db is nil")
	}

	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return nil
	}

	normalizedRoles := normalizeAdminRoles(roles)
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("user_admin_roles").Where("user_id = ?", normalizedUserID).Delete(&models.UserAdminRole{}).Error; err != nil {
			return err
		}

		if len(normalizedRoles) == 0 {
			return nil
		}

		rows := make([]models.UserAdminRole, 0, len(normalizedRoles))
		for _, role := range normalizedRoles {
			rows = append(rows, models.UserAdminRole{
				UserID:    normalizedUserID,
				Role:      role,
				CreatedAt: now,
				UpdatedAt: now,
			})
		}
		return tx.Table("user_admin_roles").Create(&rows).Error
	})
}

func normalizeAdminRoles(roles []models.AdminRole) []models.AdminRole {
	if len(roles) == 0 {
		return []models.AdminRole{}
	}

	seen := make(map[models.AdminRole]struct{}, len(roles))
	normalized := make([]models.AdminRole, 0, len(roles))
	for _, role := range roles {
		if !models.IsValidAdminRole(role) {
			continue
		}
		if _, exists := seen[role]; exists {
			continue
		}
		seen[role] = struct{}{}
		normalized = append(normalized, role)
	}
	return normalized
}
