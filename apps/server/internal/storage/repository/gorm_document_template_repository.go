package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

const (
	defaultDocumentTemplatePageSize = 20
	maxDocumentTemplatePageSize     = 100
)

type gormDocumentTemplateRepository struct {
	db *gorm.DB
}

type documentTemplateListRow struct {
	TemplateID   string `gorm:"column:template_id"`
	SceneKey     string `gorm:"column:scene_key"`
	SceneName    string `gorm:"column:scene_name"`
	Name         string `gorm:"column:name"`
	Description  string `gorm:"column:description"`
	DefaultTitle string `gorm:"column:default_title"`
	Sort         int    `gorm:"column:sort"`
	IsBuiltin    bool   `gorm:"column:is_builtin"`
	IsEnabled    bool   `gorm:"column:is_enabled"`
	UpdatedAtRaw string `gorm:"column:updated_at"`
}

type documentTemplateDetailRow struct {
	TemplateID      string  `gorm:"column:template_id"`
	SceneKey        string  `gorm:"column:scene_key"`
	SceneName       string  `gorm:"column:scene_name"`
	Name            string  `gorm:"column:name"`
	Description     string  `gorm:"column:description"`
	DefaultTitle    string  `gorm:"column:default_title"`
	ContentMD       string  `gorm:"column:content_md"`
	Sort            int     `gorm:"column:sort"`
	IsBuiltin       bool    `gorm:"column:is_builtin"`
	IsEnabled       bool    `gorm:"column:is_enabled"`
	CreatedByUserID *string `gorm:"column:created_by_user_id"`
	UpdatedByUserID *string `gorm:"column:updated_by_user_id"`
	CreatedAtRaw    string  `gorm:"column:created_at"`
	UpdatedAtRaw    string  `gorm:"column:updated_at"`
}

// NewGormDocumentTemplateRepository 创建文档模板仓储。
func NewGormDocumentTemplateRepository(db *gorm.DB) DocumentTemplateRepository {
	return &gormDocumentTemplateRepository{db: db}
}

func (r *gormDocumentTemplateRepository) List(
	ctx context.Context,
	params ListDocumentTemplatesParams,
) ([]DocumentTemplateSummaryRecord, int64, error) {
	if r == nil || r.db == nil {
		return nil, 0, fmt.Errorf("document template repository db is nil")
	}

	sceneKey := strings.TrimSpace(params.SceneKey)
	keyword := strings.ToLower(strings.TrimSpace(params.Keyword))
	limit := params.Limit
	if limit <= 0 {
		limit = defaultDocumentTemplatePageSize
	}
	if limit > maxDocumentTemplatePageSize {
		limit = maxDocumentTemplatePageSize
	}
	offset := params.Offset
	if offset < 0 {
		offset = 0
	}

	query := r.db.WithContext(ctx).Table("document_templates AS t")
	if params.EnabledOnly {
		query = query.Where("t.is_enabled = ?", true)
	}
	if sceneKey != "" {
		query = query.Where("t.scene_key = ?", sceneKey)
	}
	if keyword != "" {
		searchKeyword := "%" + keyword + "%"
		query = query.Where(
			"(LOWER(t.template_id) LIKE ? OR LOWER(t.scene_name) LIKE ? OR LOWER(t.name) LIKE ? OR LOWER(t.description) LIKE ?)",
			searchKeyword,
			searchKeyword,
			searchKeyword,
			searchKeyword,
		)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	rows := make([]documentTemplateListRow, 0, limit)
	if err := query.
		Select(
			"t.template_id",
			"t.scene_key",
			"t.scene_name",
			"t.name",
			"t.description",
			"t.default_title",
			"t.sort",
			"t.is_builtin",
			"t.is_enabled",
			"t.updated_at",
		).
		Order("t.scene_key ASC").
		Order("t.sort ASC").
		Order("t.updated_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	items := make([]DocumentTemplateSummaryRecord, 0, len(rows))
	for _, row := range rows {
		items = append(items, DocumentTemplateSummaryRecord{
			TemplateID:   strings.TrimSpace(row.TemplateID),
			SceneKey:     strings.TrimSpace(row.SceneKey),
			SceneName:    strings.TrimSpace(row.SceneName),
			Name:         strings.TrimSpace(row.Name),
			Description:  strings.TrimSpace(row.Description),
			DefaultTitle: strings.TrimSpace(row.DefaultTitle),
			Sort:         row.Sort,
			IsBuiltin:    row.IsBuiltin,
			IsEnabled:    row.IsEnabled,
			UpdatedAtRaw: strings.TrimSpace(row.UpdatedAtRaw),
		})
	}

	return items, total, nil
}

func (r *gormDocumentTemplateRepository) GetByTemplateID(
	ctx context.Context,
	templateID string,
	enabledOnly bool,
) (*DocumentTemplateDetailRecord, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("document template repository db is nil")
	}

	normalizedTemplateID := strings.ToLower(strings.TrimSpace(templateID))
	if normalizedTemplateID == "" {
		return nil, gorm.ErrRecordNotFound
	}

	query := r.db.WithContext(ctx).Table("document_templates AS t")
	if enabledOnly {
		query = query.Where("t.is_enabled = ?", true)
	}

	var row documentTemplateDetailRow
	if err := query.
		Select(
			"t.template_id",
			"t.scene_key",
			"t.scene_name",
			"t.name",
			"t.description",
			"t.default_title",
			"t.content_md",
			"t.sort",
			"t.is_builtin",
			"t.is_enabled",
			"t.created_by_user_id",
			"t.updated_by_user_id",
			"t.created_at",
			"t.updated_at",
		).
		Where("t.template_id = ?", normalizedTemplateID).
		Take(&row).Error; err != nil {
		return nil, err
	}

	return &DocumentTemplateDetailRecord{
		TemplateID:      strings.TrimSpace(row.TemplateID),
		SceneKey:        strings.TrimSpace(row.SceneKey),
		SceneName:       strings.TrimSpace(row.SceneName),
		Name:            strings.TrimSpace(row.Name),
		Description:     strings.TrimSpace(row.Description),
		DefaultTitle:    strings.TrimSpace(row.DefaultTitle),
		ContentMD:       row.ContentMD,
		Sort:            row.Sort,
		IsBuiltin:       row.IsBuiltin,
		IsEnabled:       row.IsEnabled,
		CreatedByUserID: trimOptionalString(row.CreatedByUserID),
		UpdatedByUserID: trimOptionalString(row.UpdatedByUserID),
		CreatedAtRaw:    strings.TrimSpace(row.CreatedAtRaw),
		UpdatedAtRaw:    strings.TrimSpace(row.UpdatedAtRaw),
	}, nil
}

func (r *gormDocumentTemplateRepository) Create(ctx context.Context, template *models.DocumentTemplate) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("document template repository db is nil")
	}
	if template == nil {
		return fmt.Errorf("document template is nil")
	}
	return r.db.WithContext(ctx).Create(template).Error
}

func (r *gormDocumentTemplateRepository) UpdateByTemplateID(
	ctx context.Context,
	params UpdateDocumentTemplateParams,
) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("document template repository db is nil")
	}

	normalizedTemplateID := strings.ToLower(strings.TrimSpace(params.TemplateID))
	if normalizedTemplateID == "" {
		return false, nil
	}

	updateValues := map[string]any{}
	if params.SceneKey != nil {
		updateValues["scene_key"] = strings.ToLower(strings.TrimSpace(*params.SceneKey))
	}
	if params.SceneName != nil {
		updateValues["scene_name"] = strings.TrimSpace(*params.SceneName)
	}
	if params.Name != nil {
		updateValues["name"] = strings.TrimSpace(*params.Name)
	}
	if params.Description != nil {
		updateValues["description"] = strings.TrimSpace(*params.Description)
	}
	if params.DefaultTitle != nil {
		updateValues["default_title"] = strings.TrimSpace(*params.DefaultTitle)
	}
	if params.ContentMD != nil {
		updateValues["content_md"] = *params.ContentMD
	}
	if params.Sort != nil {
		updateValues["sort"] = *params.Sort
	}
	if params.IsEnabled != nil {
		updateValues["is_enabled"] = *params.IsEnabled
	}
	if params.UpdatedByUserID != nil {
		normalizedActorUserID := strings.TrimSpace(*params.UpdatedByUserID)
		if normalizedActorUserID == "" {
			updateValues["updated_by_user_id"] = nil
		} else {
			updateValues["updated_by_user_id"] = normalizedActorUserID
		}
	}

	updatedAt := params.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	updateValues["updated_at"] = updatedAt

	if len(updateValues) == 0 {
		return false, nil
	}

	tx := r.db.WithContext(ctx).
		Model(&models.DocumentTemplate{}).
		Where("template_id = ?", normalizedTemplateID).
		Updates(updateValues)
	if tx.Error != nil {
		return false, tx.Error
	}
	return tx.RowsAffected > 0, nil
}

func (r *gormDocumentTemplateRepository) DeleteByTemplateID(
	ctx context.Context,
	templateID string,
) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("document template repository db is nil")
	}

	normalizedTemplateID := strings.ToLower(strings.TrimSpace(templateID))
	if normalizedTemplateID == "" {
		return false, nil
	}

	tx := r.db.WithContext(ctx).
		Where("template_id = ?", normalizedTemplateID).
		Delete(&models.DocumentTemplate{})
	if tx.Error != nil {
		return false, tx.Error
	}
	return tx.RowsAffected > 0, nil
}
