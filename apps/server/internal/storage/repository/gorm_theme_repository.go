package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/recordtime"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type gormThemeRepository struct {
	db *gorm.DB
}

type themeRow = themeRowDB

// NewGormThemeRepository 创建基于 GORM 的主题仓储实现。
func NewGormThemeRepository(db *gorm.DB) ThemeRepository {
	return &gormThemeRepository{db: db}
}

func (r *gormThemeRepository) List(ctx context.Context, includeDisabled bool) ([]models.Theme, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("theme repository db is nil")
	}

	query := r.db.WithContext(ctx).
		Model(&models.Theme{}).
		Select(
			models.ThemeColumns.ID,
			models.ThemeColumns.ThemeID,
			models.ThemeColumns.Name,
			models.ThemeColumns.Description,
			models.ThemeColumns.VariablesJSON,
			models.ThemeColumns.SyntaxTheme,
			models.ThemeColumns.CodeBlockStyleJSON,
			models.ThemeColumns.CodeBlockCodeStyleJSON,
			models.ThemeColumns.InlineCodeStyleJSON,
			models.ThemeColumns.CustomCSS,
			models.ThemeColumns.IsBuiltin,
			models.ThemeColumns.IsEnabled,
			models.ThemeColumns.CreatedAt+" AS created_at_raw",
			models.ThemeColumns.UpdatedAt+" AS updated_at_raw",
		)
	if !includeDisabled {
		query = query.Where(models.ThemeColumns.IsEnabled+" = ?", true)
	}
	query = query.Order(clause.OrderByColumn{
		Column: clause.Column{Name: models.ThemeColumns.ID},
		Desc:   true,
	})

	var rows []themeRow
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]models.Theme, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapThemeRow(row))
	}
	return items, nil
}

func (r *gormThemeRepository) GetByThemeID(ctx context.Context, themeID string) (*models.Theme, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("theme repository db is nil")
	}

	var row themeRow
	if err := r.db.WithContext(ctx).
		Model(&models.Theme{}).
		Select(
			models.ThemeColumns.ID,
			models.ThemeColumns.ThemeID,
			models.ThemeColumns.Name,
			models.ThemeColumns.Description,
			models.ThemeColumns.VariablesJSON,
			models.ThemeColumns.SyntaxTheme,
			models.ThemeColumns.CodeBlockStyleJSON,
			models.ThemeColumns.CodeBlockCodeStyleJSON,
			models.ThemeColumns.InlineCodeStyleJSON,
			models.ThemeColumns.CustomCSS,
			models.ThemeColumns.IsBuiltin,
			models.ThemeColumns.IsEnabled,
			models.ThemeColumns.CreatedAt+" AS created_at_raw",
			models.ThemeColumns.UpdatedAt+" AS updated_at_raw",
		).
		Where(models.ThemeColumns.ThemeID+" = ?", strings.TrimSpace(themeID)).
		Take(&row).Error; err != nil {
		return nil, err
	}

	item := mapThemeRow(row)
	return &item, nil
}

func (r *gormThemeRepository) Create(ctx context.Context, theme *models.Theme) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("theme repository db is nil")
	}
	if theme == nil {
		return fmt.Errorf("theme is nil")
	}
	return r.db.WithContext(ctx).Create(theme).Error
}

func (r *gormThemeRepository) Update(ctx context.Context, params UpdateThemeParams) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("theme repository db is nil")
	}

	themeID := strings.TrimSpace(params.ThemeID)
	if themeID == "" {
		return false, nil
	}

	updatedAt := params.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}

	updateValues := map[string]any{
		models.ThemeColumns.UpdatedAt: updatedAt,
	}
	if params.Name != nil {
		updateValues[models.ThemeColumns.Name] = *params.Name
	}
	if params.Description != nil {
		updateValues[models.ThemeColumns.Description] = *params.Description
	}
	if params.VariablesJSON != nil {
		updateValues[models.ThemeColumns.VariablesJSON] = *params.VariablesJSON
	}
	if params.SyntaxTheme != nil {
		updateValues[models.ThemeColumns.SyntaxTheme] = *params.SyntaxTheme
	}
	if params.CodeBlockStyleJSON != nil {
		updateValues[models.ThemeColumns.CodeBlockStyleJSON] = *params.CodeBlockStyleJSON
	}
	if params.CodeBlockCodeStyleJSON != nil {
		updateValues[models.ThemeColumns.CodeBlockCodeStyleJSON] = *params.CodeBlockCodeStyleJSON
	}
	if params.InlineCodeStyleJSON != nil {
		updateValues[models.ThemeColumns.InlineCodeStyleJSON] = *params.InlineCodeStyleJSON
	}
	if params.CustomCSS != nil {
		updateValues[models.ThemeColumns.CustomCSS] = *params.CustomCSS
	}
	if params.IsEnabled != nil {
		updateValues[models.ThemeColumns.IsEnabled] = *params.IsEnabled
	}

	tx := r.db.WithContext(ctx).
		Model(&models.Theme{}).
		Where(models.ThemeColumns.ThemeID+" = ?", themeID).
		Updates(updateValues)
	if tx.Error != nil {
		return false, tx.Error
	}
	return tx.RowsAffected > 0, nil
}

func (r *gormThemeRepository) Delete(ctx context.Context, themeID string) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("theme repository db is nil")
	}

	tx := r.db.WithContext(ctx).
		Model(&models.Theme{}).
		Where(models.ThemeColumns.ThemeID+" = ?", strings.TrimSpace(themeID)).
		Delete(&models.Theme{})
	if tx.Error != nil {
		return false, tx.Error
	}
	return tx.RowsAffected > 0, nil
}

func (r *gormThemeRepository) CountDocumentReferences(ctx context.Context, themeID string) (int64, error) {
	if r == nil || r.db == nil {
		return 0, fmt.Errorf("theme repository db is nil")
	}

	var count int64
	if err := r.db.WithContext(ctx).
		Model(&models.Document{}).
		Where(models.DocumentColumns.ThemeID+" = ?", strings.TrimSpace(themeID)).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func mapThemeRow(row themeRow) models.Theme {
	return models.Theme{
		ID:                     row.ID,
		ThemeID:                row.ThemeID,
		Name:                   row.Name,
		Description:            row.Description,
		VariablesJSON:          row.VariablesJSON,
		SyntaxTheme:            row.SyntaxTheme,
		CodeBlockStyleJSON:     row.CodeBlockStyleJSON,
		CodeBlockCodeStyleJSON: row.CodeBlockCodeStyleJSON,
		InlineCodeStyleJSON:    row.InlineCodeStyleJSON,
		CustomCSS:              row.CustomCSS,
		IsBuiltin:              row.IsBuiltin,
		IsEnabled:              row.IsEnabled,
		CreatedAt:              recordtime.Parse(row.CreatedAtRaw),
		UpdatedAt:              recordtime.Parse(row.UpdatedAtRaw),
	}
}
