package repository

import (
	"context"
	"fmt"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

type gormSpaceRepository struct {
	db *gorm.DB
}

// NewGormSpaceRepository 创建基于 GORM 的空间仓储实现。
func NewGormSpaceRepository(db *gorm.DB) SpaceRepository {
	return &gormSpaceRepository{db: db}
}

func (r *gormSpaceRepository) Create(ctx context.Context, space *models.Space) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("space repository db is nil")
	}
	if space != nil {
		if !models.IsValidVisibility(space.Visibility) {
			space.Visibility = models.VisibilityMember
		}
		if !models.IsValidEntityStatus(space.Status) {
			space.Status = models.EntityStatusActive
		}
	}
	return r.db.WithContext(ctx).Create(space).Error
}

func (r *gormSpaceRepository) GetBySpaceID(ctx context.Context, spaceID string) (*models.Space, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("space repository db is nil")
	}

	var space models.Space
	if err := r.db.WithContext(ctx).
		Select(
			"id",
			"space_id",
			"name",
			"owner_user_id",
			"visibility",
			"status",
			"banned_reason",
			"banned_at",
			"deleted_at",
		).
		Where("space_id = ?", spaceID).
		Take(&space).Error; err != nil {
		return nil, err
	}
	if !models.IsValidVisibility(space.Visibility) {
		space.Visibility = models.VisibilityMember
	}
	if !models.IsValidEntityStatus(space.Status) {
		space.Status = models.EntityStatusActive
	}
	return &space, nil
}

func (r *gormSpaceRepository) ListByUserID(ctx context.Context, userID string) ([]models.Space, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("space repository db is nil")
	}

	var spaces []models.Space
	if err := r.db.WithContext(ctx).
		Table("spaces AS s").
		Select(
			"s.id",
			"s.space_id",
			"s.name",
			"s.owner_user_id",
			"s.visibility",
			"s.status",
			"s.banned_reason",
			"s.banned_at",
			"s.deleted_at",
		).
		Joins("LEFT JOIN space_members AS sm ON sm.space_id = s.space_id AND sm.user_id = ?", userID).
		Where("s.owner_user_id = ? OR sm.id IS NOT NULL", userID).
		Order("s.id DESC").
		Find(&spaces).Error; err != nil {
		return nil, err
	}

	for i := range spaces {
		if !models.IsValidVisibility(spaces[i].Visibility) {
			spaces[i].Visibility = models.VisibilityMember
		}
		if !models.IsValidEntityStatus(spaces[i].Status) {
			spaces[i].Status = models.EntityStatusActive
		}
	}

	return spaces, nil
}

func (r *gormSpaceRepository) UpdateVisibility(
	ctx context.Context,
	spaceID string,
	visibility models.Visibility,
) (*models.Space, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("space repository db is nil")
	}

	updateTx := r.db.WithContext(ctx).
		Model(&models.Space{}).
		Where("space_id = ?", spaceID).
		Updates(map[string]any{
			"visibility": visibility,
			"updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
		})
	if updateTx.Error != nil {
		return nil, updateTx.Error
	}
	if updateTx.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	return r.GetBySpaceID(ctx, spaceID)
}

func (r *gormSpaceRepository) HasReaderAccess(ctx context.Context, spaceID string, userID string) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("space repository db is nil")
	}
	if userID == "" {
		return false, nil
	}

	var count int64
	if err := r.db.WithContext(ctx).
		Table("spaces AS s").
		Joins("LEFT JOIN space_members AS sm ON sm.space_id = s.space_id AND sm.user_id = ?", userID).
		Where(
			"s.space_id = ? AND s.status = ? AND s.deleted_at IS NULL AND (s.owner_user_id = ? OR (sm.id IS NOT NULL AND sm.role IN ?))",
			spaceID,
			models.EntityStatusActive,
			userID,
			[]models.Role{models.RoleOwner, models.RoleCollaborator, models.RoleReader},
		).
		Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}
