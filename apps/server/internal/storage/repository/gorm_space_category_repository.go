package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

type gormSpaceCategoryRepository struct {
	db *gorm.DB
}

type spaceCategoryRow struct {
	ID         int64  `gorm:"column:id"`
	CategoryID string `gorm:"column:category_id"`
	Name       string `gorm:"column:name"`
	IsDefault  bool   `gorm:"column:is_default"`
	CreatedAt  string `gorm:"column:created_at"`
	UpdatedAt  string `gorm:"column:updated_at"`
}

// NewGormSpaceCategoryRepository 创建基于 GORM 的空间分类仓储实现。
func NewGormSpaceCategoryRepository(db *gorm.DB) SpaceCategoryRepository {
	return &gormSpaceCategoryRepository{db: db}
}

func (r *gormSpaceCategoryRepository) List(ctx context.Context) ([]models.SpaceCategory, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("space category repository db is nil")
	}

	var rows []spaceCategoryRow
	if err := r.db.WithContext(ctx).
		Table("space_categories").
		Select("id", "category_id", "name", "is_default", "created_at", "updated_at").
		Order("is_default DESC, name ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]models.SpaceCategory, 0, len(rows))
	for _, row := range rows {
		result = append(result, mapSpaceCategoryRow(row))
	}
	return result, nil
}

func (r *gormSpaceCategoryRepository) GetByCategoryID(
	ctx context.Context,
	categoryID string,
) (*models.SpaceCategory, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("space category repository db is nil")
	}
	normalizedCategoryID := strings.TrimSpace(categoryID)
	if normalizedCategoryID == "" {
		return nil, gorm.ErrRecordNotFound
	}

	var row spaceCategoryRow
	if err := r.db.WithContext(ctx).
		Table("space_categories").
		Select("id", "category_id", "name", "is_default", "created_at", "updated_at").
		Where("category_id = ?", normalizedCategoryID).
		Take(&row).Error; err != nil {
		return nil, err
	}

	category := mapSpaceCategoryRow(row)
	return &category, nil
}

func (r *gormSpaceCategoryRepository) GetByName(ctx context.Context, name string) (*models.SpaceCategory, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("space category repository db is nil")
	}
	normalizedName := strings.TrimSpace(name)
	if normalizedName == "" {
		return nil, gorm.ErrRecordNotFound
	}

	var row spaceCategoryRow
	if err := r.db.WithContext(ctx).
		Table("space_categories").
		Select("id", "category_id", "name", "is_default", "created_at", "updated_at").
		Where("LOWER(name) = ?", strings.ToLower(normalizedName)).
		Take(&row).Error; err != nil {
		return nil, err
	}

	category := mapSpaceCategoryRow(row)
	return &category, nil
}

func (r *gormSpaceCategoryRepository) GetDefault(ctx context.Context) (*models.SpaceCategory, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("space category repository db is nil")
	}

	var row spaceCategoryRow
	if err := r.db.WithContext(ctx).
		Table("space_categories").
		Select("id", "category_id", "name", "is_default", "created_at", "updated_at").
		Where("is_default = ?", true).
		Order("id ASC").
		Take(&row).Error; err != nil {
		return nil, err
	}

	category := mapSpaceCategoryRow(row)
	return &category, nil
}

func (r *gormSpaceCategoryRepository) Create(ctx context.Context, category *models.SpaceCategory) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("space category repository db is nil")
	}
	if category == nil {
		return fmt.Errorf("space category is nil")
	}

	category.CategoryID = strings.TrimSpace(category.CategoryID)
	category.Name = strings.TrimSpace(category.Name)
	return r.db.WithContext(ctx).Create(category).Error
}

func (r *gormSpaceCategoryRepository) RenameAndSyncSpaces(
	ctx context.Context,
	categoryID string,
	name string,
	updatedAt time.Time,
) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("space category repository db is nil")
	}

	normalizedCategoryID := strings.TrimSpace(categoryID)
	normalizedName := strings.TrimSpace(name)
	if normalizedCategoryID == "" || normalizedName == "" {
		return false, nil
	}

	effectiveUpdatedAt := updatedAt
	if effectiveUpdatedAt.IsZero() {
		effectiveUpdatedAt = time.Now().UTC()
	}

	updated := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updateCategoryTx := tx.Model(&models.SpaceCategory{}).
			Where("category_id = ?", normalizedCategoryID).
			Updates(map[string]any{
				"name":       normalizedName,
				"updated_at": effectiveUpdatedAt,
			})
		if updateCategoryTx.Error != nil {
			return updateCategoryTx.Error
		}
		if updateCategoryTx.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		if err := tx.Model(&models.Space{}).
			Where("category_id = ?", normalizedCategoryID).
			Updates(map[string]any{
				"category":   normalizedName,
				"updated_at": effectiveUpdatedAt,
			}).Error; err != nil {
			return err
		}

		updated = true
		return nil
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, err
	}
	return updated, nil
}

func (r *gormSpaceCategoryRepository) DeleteAndReassignSpaces(
	ctx context.Context,
	categoryID string,
	fallbackCategoryID string,
	fallbackCategoryName string,
	updatedAt time.Time,
) (int64, bool, error) {
	if r == nil || r.db == nil {
		return 0, false, fmt.Errorf("space category repository db is nil")
	}
	normalizedCategoryID := strings.TrimSpace(categoryID)
	normalizedFallbackCategoryID := strings.TrimSpace(fallbackCategoryID)
	normalizedFallbackCategoryName := strings.TrimSpace(fallbackCategoryName)
	if normalizedCategoryID == "" || normalizedFallbackCategoryID == "" || normalizedFallbackCategoryName == "" {
		return 0, false, nil
	}

	effectiveUpdatedAt := updatedAt
	if effectiveUpdatedAt.IsZero() {
		effectiveUpdatedAt = time.Now().UTC()
	}

	var movedCount int64
	deleted := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		moveTx := tx.Model(&models.Space{}).
			Where("category_id = ?", normalizedCategoryID).
			Updates(map[string]any{
				"category_id": normalizedFallbackCategoryID,
				"category":    normalizedFallbackCategoryName,
				"updated_at":  effectiveUpdatedAt,
			})
		if moveTx.Error != nil {
			return moveTx.Error
		}
		movedCount = moveTx.RowsAffected

		deleteTx := tx.Where("category_id = ? AND is_default = ?", normalizedCategoryID, false).
			Delete(&models.SpaceCategory{})
		if deleteTx.Error != nil {
			return deleteTx.Error
		}
		if deleteTx.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		deleted = true
		return nil
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, false, nil
		}
		return 0, false, err
	}

	return movedCount, deleted, nil
}

func mapSpaceCategoryRow(row spaceCategoryRow) models.SpaceCategory {
	return models.SpaceCategory{
		ID:         row.ID,
		CategoryID: strings.TrimSpace(row.CategoryID),
		Name:       strings.TrimSpace(row.Name),
		IsDefault:  row.IsDefault,
		CreatedAt:  parseSpaceRecordTime(row.CreatedAt),
		UpdatedAt:  parseSpaceRecordTime(row.UpdatedAt),
	}
}
