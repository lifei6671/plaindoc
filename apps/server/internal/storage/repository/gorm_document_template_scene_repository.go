package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	defaultDocumentTemplateScenePageSize = 20
	maxDocumentTemplateScenePageSize     = 100
)

type gormDocumentTemplateSceneRepository struct {
	db *gorm.DB
}

type documentTemplateSceneListRow = documentTemplateSceneListRowDB

type documentTemplateSceneDetailRow = documentTemplateSceneDetailRowDB

// NewGormDocumentTemplateSceneRepository 创建文档模板场景仓储。
func NewGormDocumentTemplateSceneRepository(db *gorm.DB) DocumentTemplateSceneRepository {
	return &gormDocumentTemplateSceneRepository{db: db}
}

func (r *gormDocumentTemplateSceneRepository) List(
	ctx context.Context,
	params ListDocumentTemplateScenesParams,
) ([]DocumentTemplateSceneSummaryRecord, int64, error) {
	if r == nil || r.db == nil {
		return nil, 0, fmt.Errorf("document template scene repository db is nil")
	}

	keyword := strings.ToLower(strings.TrimSpace(params.Keyword))
	limit := params.Limit
	if limit <= 0 {
		limit = defaultDocumentTemplateScenePageSize
	}
	if limit > maxDocumentTemplateScenePageSize {
		limit = maxDocumentTemplateScenePageSize
	}
	offset := params.Offset
	if offset < 0 {
		offset = 0
	}

	sceneTableName := (models.DocumentTemplateScene{}).TableName()
	templateTableName := (models.DocumentTemplate{}).TableName()
	sceneAlias := "s"
	templateAlias := "t"

	query := r.db.WithContext(ctx).Table(sceneTableName + " AS " + sceneAlias)
	if keyword != "" {
		searchKeyword := "%" + keyword + "%"
		query = query.Where(
			"(LOWER("+qualifiedColumn(sceneAlias, models.DocumentTemplateSceneColumns.SceneKey)+") LIKE ? OR LOWER("+qualifiedColumn(sceneAlias, models.DocumentTemplateSceneColumns.SceneName)+") LIKE ? OR LOWER("+qualifiedColumn(sceneAlias, models.DocumentTemplateSceneColumns.Description)+") LIKE ?)",
			searchKeyword,
			searchKeyword,
			searchKeyword,
		)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	rows := make([]documentTemplateSceneListRow, 0, limit)
	if err := query.
		Select(
			qualifiedColumn(sceneAlias, models.DocumentTemplateSceneColumns.SceneKey),
			qualifiedColumn(sceneAlias, models.DocumentTemplateSceneColumns.SceneName),
			qualifiedColumn(sceneAlias, models.DocumentTemplateSceneColumns.Description),
			qualifiedColumn(sceneAlias, models.DocumentTemplateSceneColumns.Sort),
			qualifiedColumn(sceneAlias, models.DocumentTemplateSceneColumns.IsBuiltin),
			qualifiedColumn(sceneAlias, models.DocumentTemplateSceneColumns.UpdatedAt)+" AS updated_at_raw",
			"COUNT("+qualifiedColumn(templateAlias, models.DocumentTemplateColumns.TemplateID)+") AS template_count",
		).
		Joins(
			"LEFT JOIN " + templateTableName + " AS " + templateAlias +
				" ON " + qualifiedColumn(templateAlias, models.DocumentTemplateColumns.SceneKey) +
				" = " + qualifiedColumn(sceneAlias, models.DocumentTemplateSceneColumns.SceneKey),
		).
		Group(
			qualifiedColumn(sceneAlias, models.DocumentTemplateSceneColumns.SceneKey) + ", " +
				qualifiedColumn(sceneAlias, models.DocumentTemplateSceneColumns.SceneName) + ", " +
				qualifiedColumn(sceneAlias, models.DocumentTemplateSceneColumns.Description) + ", " +
				qualifiedColumn(sceneAlias, models.DocumentTemplateSceneColumns.Sort) + ", " +
				qualifiedColumn(sceneAlias, models.DocumentTemplateSceneColumns.IsBuiltin) + ", " +
				qualifiedColumn(sceneAlias, models.DocumentTemplateSceneColumns.UpdatedAt),
		).
		Order(clause.OrderByColumn{
			Column: clause.Column{Table: sceneAlias, Name: models.DocumentTemplateSceneColumns.Sort},
		}).
		Order(clause.OrderByColumn{
			Column: clause.Column{Table: sceneAlias, Name: models.DocumentTemplateSceneColumns.SceneKey},
		}).
		Offset(offset).
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	items := make([]DocumentTemplateSceneSummaryRecord, 0, len(rows))
	for _, row := range rows {
		items = append(items, DocumentTemplateSceneSummaryRecord{
			SceneKey:      strings.TrimSpace(row.SceneKey),
			SceneName:     strings.TrimSpace(row.SceneName),
			Description:   strings.TrimSpace(row.Description),
			Sort:          row.Sort,
			IsBuiltin:     row.IsBuiltin,
			TemplateCount: row.TemplateCount,
			UpdatedAtRaw:  strings.TrimSpace(row.UpdatedAtRaw),
		})
	}

	return items, total, nil
}

func (r *gormDocumentTemplateSceneRepository) GetBySceneKey(
	ctx context.Context,
	sceneKey string,
) (*DocumentTemplateSceneDetailRecord, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("document template scene repository db is nil")
	}

	normalizedSceneKey := strings.ToLower(strings.TrimSpace(sceneKey))
	if normalizedSceneKey == "" {
		return nil, gorm.ErrRecordNotFound
	}

	sceneTableName := (models.DocumentTemplateScene{}).TableName()
	sceneAlias := "s"

	var row documentTemplateSceneDetailRow
	if err := r.db.WithContext(ctx).
		Table(sceneTableName+" AS "+sceneAlias).
		Select(
			qualifiedColumn(sceneAlias, models.DocumentTemplateSceneColumns.SceneKey),
			qualifiedColumn(sceneAlias, models.DocumentTemplateSceneColumns.SceneName),
			qualifiedColumn(sceneAlias, models.DocumentTemplateSceneColumns.Description),
			qualifiedColumn(sceneAlias, models.DocumentTemplateSceneColumns.Sort),
			qualifiedColumn(sceneAlias, models.DocumentTemplateSceneColumns.IsBuiltin),
			qualifiedColumn(sceneAlias, models.DocumentTemplateSceneColumns.CreatedByUserID),
			qualifiedColumn(sceneAlias, models.DocumentTemplateSceneColumns.UpdatedByUserID),
			qualifiedColumn(sceneAlias, models.DocumentTemplateSceneColumns.CreatedAt)+" AS created_at_raw",
			qualifiedColumn(sceneAlias, models.DocumentTemplateSceneColumns.UpdatedAt)+" AS updated_at_raw",
		).
		Where(qualifiedColumn(sceneAlias, models.DocumentTemplateSceneColumns.SceneKey)+" = ?", normalizedSceneKey).
		Take(&row).Error; err != nil {
		return nil, err
	}

	return &DocumentTemplateSceneDetailRecord{
		SceneKey:        strings.TrimSpace(row.SceneKey),
		SceneName:       strings.TrimSpace(row.SceneName),
		Description:     strings.TrimSpace(row.Description),
		Sort:            row.Sort,
		IsBuiltin:       row.IsBuiltin,
		CreatedByUserID: trimOptionalString(row.CreatedByUserID),
		UpdatedByUserID: trimOptionalString(row.UpdatedByUserID),
		CreatedAtRaw:    strings.TrimSpace(row.CreatedAtRaw),
		UpdatedAtRaw:    strings.TrimSpace(row.UpdatedAtRaw),
	}, nil
}

func (r *gormDocumentTemplateSceneRepository) Create(ctx context.Context, scene *models.DocumentTemplateScene) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("document template scene repository db is nil")
	}
	if scene == nil {
		return fmt.Errorf("document template scene is nil")
	}
	return r.db.WithContext(ctx).Create(scene).Error
}

func (r *gormDocumentTemplateSceneRepository) UpdateBySceneKey(
	ctx context.Context,
	params UpdateDocumentTemplateSceneParams,
) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("document template scene repository db is nil")
	}

	normalizedSceneKey := strings.ToLower(strings.TrimSpace(params.SceneKey))
	if normalizedSceneKey == "" {
		return false, nil
	}

	updateValues := map[string]any{}
	if params.SceneName != nil {
		updateValues[models.DocumentTemplateSceneColumns.SceneName] = strings.TrimSpace(*params.SceneName)
	}
	if params.Description != nil {
		updateValues[models.DocumentTemplateSceneColumns.Description] = strings.TrimSpace(*params.Description)
	}
	if params.Sort != nil {
		updateValues[models.DocumentTemplateSceneColumns.Sort] = *params.Sort
	}
	if params.UpdatedByUserID != nil {
		normalizedActorUserID := strings.TrimSpace(*params.UpdatedByUserID)
		if normalizedActorUserID == "" {
			updateValues[models.DocumentTemplateSceneColumns.UpdatedByUserID] = nil
		} else {
			updateValues[models.DocumentTemplateSceneColumns.UpdatedByUserID] = normalizedActorUserID
		}
	}

	updatedAt := params.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	updateValues[models.DocumentTemplateSceneColumns.UpdatedAt] = updatedAt

	if len(updateValues) == 0 {
		return false, nil
	}

	tx := r.db.WithContext(ctx).
		Model(&models.DocumentTemplateScene{}).
		Where(models.DocumentTemplateSceneColumns.SceneKey+" = ?", normalizedSceneKey).
		Updates(updateValues)
	if tx.Error != nil {
		return false, tx.Error
	}
	return tx.RowsAffected > 0, nil
}

func (r *gormDocumentTemplateSceneRepository) DeleteBySceneKey(
	ctx context.Context,
	sceneKey string,
) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("document template scene repository db is nil")
	}

	normalizedSceneKey := strings.ToLower(strings.TrimSpace(sceneKey))
	if normalizedSceneKey == "" {
		return false, nil
	}

	tx := r.db.WithContext(ctx).
		Model(&models.DocumentTemplateScene{}).
		Where(models.DocumentTemplateSceneColumns.SceneKey+" = ?", normalizedSceneKey).
		Delete(&models.DocumentTemplateScene{})
	if tx.Error != nil {
		return false, tx.Error
	}
	return tx.RowsAffected > 0, nil
}

func (r *gormDocumentTemplateSceneRepository) CountTemplatesBySceneKey(
	ctx context.Context,
	sceneKey string,
) (int64, error) {
	if r == nil || r.db == nil {
		return 0, fmt.Errorf("document template scene repository db is nil")
	}

	normalizedSceneKey := strings.ToLower(strings.TrimSpace(sceneKey))
	if normalizedSceneKey == "" {
		return 0, nil
	}

	templateTableName := (models.DocumentTemplate{}).TableName()
	templateAlias := "t"

	var total int64
	if err := r.db.WithContext(ctx).
		Table(templateTableName+" AS "+templateAlias).
		Where(qualifiedColumn(templateAlias, models.DocumentTemplateColumns.SceneKey)+" = ?", normalizedSceneKey).
		Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}
