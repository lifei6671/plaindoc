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

type documentTemplateListRow = documentTemplateListRowDB

type documentTemplateDetailRow = documentTemplateDetailRowDB

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

	templateAlias := "t"
	sceneAlias := "s"
	query := r.db.WithContext(ctx).
		Table(tableWithAlias(models.DocumentTemplate{}, templateAlias))
	query = query.Joins(
		"LEFT JOIN " + tableName(models.DocumentTemplateScene{}) + " AS " + sceneAlias +
			" ON " + qualifiedColumn(sceneAlias, models.DocumentTemplateSceneColumns.SceneKey) +
			" = " + qualifiedColumn(templateAlias, models.DocumentTemplateColumns.SceneKey),
	)
	if params.EnabledOnly {
		query = query.Where(qualifiedColumn(templateAlias, models.DocumentTemplateColumns.IsEnabled)+" = ?", true)
	}
	if sceneKey != "" {
		query = query.Where(qualifiedColumn(templateAlias, models.DocumentTemplateColumns.SceneKey)+" = ?", sceneKey)
	}
	if keyword != "" {
		searchKeyword := "%" + keyword + "%"
		query = query.Where(
			"(LOWER("+qualifiedColumn(templateAlias, models.DocumentTemplateColumns.TemplateID)+") LIKE ? OR "+
				"LOWER(COALESCE("+qualifiedColumn(sceneAlias, models.DocumentTemplateSceneColumns.SceneName)+", '')) LIKE ? OR "+
				"LOWER("+qualifiedColumn(templateAlias, models.DocumentTemplateColumns.Name)+") LIKE ? OR "+
				"LOWER("+qualifiedColumn(templateAlias, models.DocumentTemplateColumns.Description)+") LIKE ?)",
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
			qualifiedColumn(templateAlias, models.DocumentTemplateColumns.TemplateID)+" AS TemplateID",
			qualifiedColumn(templateAlias, models.DocumentTemplateColumns.SceneKey)+" AS SceneKey",
			"COALESCE("+qualifiedColumn(sceneAlias, models.DocumentTemplateSceneColumns.SceneName)+", '') AS SceneName",
			qualifiedColumn(templateAlias, models.DocumentTemplateColumns.Name)+" AS Name",
			qualifiedColumn(templateAlias, models.DocumentTemplateColumns.Description)+" AS Description",
			qualifiedColumn(templateAlias, models.DocumentTemplateColumns.DefaultTitle)+" AS DefaultTitle",
			qualifiedColumn(templateAlias, models.DocumentTemplateColumns.Sort)+" AS Sort",
			qualifiedColumn(templateAlias, models.DocumentTemplateColumns.IsBuiltin)+" AS IsBuiltin",
			qualifiedColumn(templateAlias, models.DocumentTemplateColumns.IsEnabled)+" AS IsEnabled",
			qualifiedColumn(templateAlias, models.DocumentTemplateColumns.UpdatedAt)+" AS UpdatedAtRaw",
		).
		Order(qualifiedColumn(templateAlias, models.DocumentTemplateColumns.SceneKey) + " ASC").
		Order(qualifiedColumn(templateAlias, models.DocumentTemplateColumns.Sort) + " ASC").
		Order(qualifiedColumn(templateAlias, models.DocumentTemplateColumns.UpdatedAt) + " DESC").
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

	templateAlias := "t"
	sceneAlias := "s"
	query := r.db.WithContext(ctx).
		Table(tableWithAlias(models.DocumentTemplate{}, templateAlias)).
		Joins(
			"LEFT JOIN " + tableName(models.DocumentTemplateScene{}) + " AS " + sceneAlias +
				" ON " + qualifiedColumn(sceneAlias, models.DocumentTemplateSceneColumns.SceneKey) +
				" = " + qualifiedColumn(templateAlias, models.DocumentTemplateColumns.SceneKey),
		)
	if enabledOnly {
		query = query.Where(qualifiedColumn(templateAlias, models.DocumentTemplateColumns.IsEnabled)+" = ?", true)
	}

	var row documentTemplateDetailRow
	if err := query.
		Select(
			qualifiedColumn(templateAlias, models.DocumentTemplateColumns.TemplateID)+" AS TemplateID",
			qualifiedColumn(templateAlias, models.DocumentTemplateColumns.SceneKey)+" AS SceneKey",
			"COALESCE("+qualifiedColumn(sceneAlias, models.DocumentTemplateSceneColumns.SceneName)+", '') AS SceneName",
			qualifiedColumn(templateAlias, models.DocumentTemplateColumns.Name)+" AS Name",
			qualifiedColumn(templateAlias, models.DocumentTemplateColumns.Description)+" AS Description",
			qualifiedColumn(templateAlias, models.DocumentTemplateColumns.DefaultTitle)+" AS DefaultTitle",
			qualifiedColumn(templateAlias, models.DocumentTemplateColumns.ContentMD)+" AS ContentMD",
			qualifiedColumn(templateAlias, models.DocumentTemplateColumns.Sort)+" AS Sort",
			qualifiedColumn(templateAlias, models.DocumentTemplateColumns.IsBuiltin)+" AS IsBuiltin",
			qualifiedColumn(templateAlias, models.DocumentTemplateColumns.IsEnabled)+" AS IsEnabled",
			qualifiedColumn(templateAlias, models.DocumentTemplateColumns.CreatedByUserID)+" AS CreatedByUserID",
			qualifiedColumn(templateAlias, models.DocumentTemplateColumns.UpdatedByUserID)+" AS UpdatedByUserID",
			qualifiedColumn(templateAlias, models.DocumentTemplateColumns.CreatedAt)+" AS CreatedAtRaw",
			qualifiedColumn(templateAlias, models.DocumentTemplateColumns.UpdatedAt)+" AS UpdatedAtRaw",
		).
		Where(qualifiedColumn(templateAlias, models.DocumentTemplateColumns.TemplateID)+" = ?", normalizedTemplateID).
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
	if params.Name != nil {
		updateValues[models.DocumentTemplateColumns.Name] = strings.TrimSpace(*params.Name)
	}
	if params.Description != nil {
		updateValues[models.DocumentTemplateColumns.Description] = strings.TrimSpace(*params.Description)
	}
	if params.DefaultTitle != nil {
		updateValues[models.DocumentTemplateColumns.DefaultTitle] = strings.TrimSpace(*params.DefaultTitle)
	}
	if params.ContentMD != nil {
		updateValues[models.DocumentTemplateColumns.ContentMD] = *params.ContentMD
	}
	if params.Sort != nil {
		updateValues[models.DocumentTemplateColumns.Sort] = *params.Sort
	}
	if params.IsEnabled != nil {
		updateValues[models.DocumentTemplateColumns.IsEnabled] = *params.IsEnabled
	}
	if params.UpdatedByUserID != nil {
		normalizedActorUserID := strings.TrimSpace(*params.UpdatedByUserID)
		if normalizedActorUserID == "" {
			updateValues[models.DocumentTemplateColumns.UpdatedByUserID] = nil
		} else {
			updateValues[models.DocumentTemplateColumns.UpdatedByUserID] = normalizedActorUserID
		}
	}

	updatedAt := params.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	updateValues[models.DocumentTemplateColumns.UpdatedAt] = updatedAt

	if len(updateValues) == 0 {
		return false, nil
	}

	tx := r.db.WithContext(ctx).
		Model(&models.DocumentTemplate{}).
		Where(models.DocumentTemplateColumns.TemplateID+" = ?", normalizedTemplateID).
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
		Where(models.DocumentTemplateColumns.TemplateID+" = ?", normalizedTemplateID).
		Delete(&models.DocumentTemplate{})
	if tx.Error != nil {
		return false, tx.Error
	}
	return tx.RowsAffected > 0, nil
}
