package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

type gormThemeRepository struct {
	db *gorm.DB
}

type themeRow struct {
	ID                     int64  `gorm:"column:id"`
	ThemeID                string `gorm:"column:theme_id"`
	Name                   string `gorm:"column:name"`
	Description            string `gorm:"column:description"`
	VariablesJSON          string `gorm:"column:variables_json"`
	SyntaxTheme            string `gorm:"column:syntax_theme"`
	CodeBlockStyleJSON     string `gorm:"column:code_block_style_json"`
	CodeBlockCodeStyleJSON string `gorm:"column:code_block_code_style_json"`
	InlineCodeStyleJSON    string `gorm:"column:inline_code_style_json"`
	CustomCSS              string `gorm:"column:custom_css"`
	IsBuiltin              bool   `gorm:"column:is_builtin"`
	IsEnabled              bool   `gorm:"column:is_enabled"`
	CreatedAtRaw           string `gorm:"column:created_at"`
	UpdatedAtRaw           string `gorm:"column:updated_at"`
}

// NewGormThemeRepository 创建基于 GORM 的主题仓储实现。
func NewGormThemeRepository(db *gorm.DB) ThemeRepository {
	return &gormThemeRepository{db: db}
}

func (r *gormThemeRepository) List(ctx context.Context, includeDisabled bool) ([]models.Theme, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("theme repository db is nil")
	}

	query := r.db.WithContext(ctx).
		Table("themes").
		Select(
			"id",
			"theme_id",
			"name",
			"description",
			"variables_json",
			"syntax_theme",
			"code_block_style_json",
			"code_block_code_style_json",
			"inline_code_style_json",
			"custom_css",
			"is_builtin",
			"is_enabled",
			"created_at",
			"updated_at",
		)
	if !includeDisabled {
		query = query.Where("is_enabled = ?", true)
	}
	query = query.Order("is_builtin DESC").Order("updated_at DESC")

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
		Table("themes").
		Select(
			"id",
			"theme_id",
			"name",
			"description",
			"variables_json",
			"syntax_theme",
			"code_block_style_json",
			"code_block_code_style_json",
			"inline_code_style_json",
			"custom_css",
			"is_builtin",
			"is_enabled",
			"created_at",
			"updated_at",
		).
		Where("theme_id = ?", strings.TrimSpace(themeID)).
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
		"updated_at": updatedAt,
	}
	if params.Name != nil {
		updateValues["name"] = *params.Name
	}
	if params.Description != nil {
		updateValues["description"] = *params.Description
	}
	if params.VariablesJSON != nil {
		updateValues["variables_json"] = *params.VariablesJSON
	}
	if params.SyntaxTheme != nil {
		updateValues["syntax_theme"] = *params.SyntaxTheme
	}
	if params.CodeBlockStyleJSON != nil {
		updateValues["code_block_style_json"] = *params.CodeBlockStyleJSON
	}
	if params.CodeBlockCodeStyleJSON != nil {
		updateValues["code_block_code_style_json"] = *params.CodeBlockCodeStyleJSON
	}
	if params.InlineCodeStyleJSON != nil {
		updateValues["inline_code_style_json"] = *params.InlineCodeStyleJSON
	}
	if params.CustomCSS != nil {
		updateValues["custom_css"] = *params.CustomCSS
	}
	if params.IsEnabled != nil {
		updateValues["is_enabled"] = *params.IsEnabled
	}

	tx := r.db.WithContext(ctx).
		Model(&models.Theme{}).
		Where("theme_id = ?", themeID).
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
		Where("theme_id = ?", strings.TrimSpace(themeID)).
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
		Table("documents").
		Where("theme_id = ?", strings.TrimSpace(themeID)).
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
		CreatedAt:              parseThemeRecordTime(row.CreatedAtRaw),
		UpdatedAt:              parseThemeRecordTime(row.UpdatedAtRaw),
	}
}

func parseThemeRecordTime(raw string) time.Time {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05.999999-07:00",
		"2006-01-02 15:04:05.999-07:00",
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05",
	}
	for _, layout := range layouts {
		if parsedAt, err := time.Parse(layout, value); err == nil {
			return parsedAt.UTC()
		}
	}
	// 兼容数据库返回的 "YYYY-MM-DD HH:MM:SS+00:00" 等格式，统一转为 RFC3339 再解析。
	normalized := strings.Replace(value, " ", "T", 1)
	timePart := normalized
	if index := strings.IndexByte(normalized, 'T'); index >= 0 && index < len(normalized)-1 {
		timePart = normalized[index+1:]
	}
	if !strings.ContainsAny(timePart, "Zz+-") {
		normalized += "Z"
	}
	if parsedAt, err := time.Parse(time.RFC3339Nano, normalized); err == nil {
		return parsedAt.UTC()
	}
	if parsedAt, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.UTC); err == nil {
		return parsedAt.UTC()
	}
	return time.Time{}
}
