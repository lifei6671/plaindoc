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
	defaultDocumentTemplateScenePageSize = 20
	maxDocumentTemplateScenePageSize     = 100
)

type gormDocumentTemplateSceneRepository struct {
	db *gorm.DB
}

type documentTemplateSceneListRow struct {
	SceneKey      string `gorm:"column:scene_key"`
	SceneName     string `gorm:"column:scene_name"`
	Description   string `gorm:"column:description"`
	Sort          int    `gorm:"column:sort"`
	IsBuiltin     bool   `gorm:"column:is_builtin"`
	TemplateCount int64  `gorm:"column:template_count"`
	UpdatedAtRaw  string `gorm:"column:updated_at"`
}

type documentTemplateSceneDetailRow struct {
	SceneKey        string  `gorm:"column:scene_key"`
	SceneName       string  `gorm:"column:scene_name"`
	Description     string  `gorm:"column:description"`
	Sort            int     `gorm:"column:sort"`
	IsBuiltin       bool    `gorm:"column:is_builtin"`
	CreatedByUserID *string `gorm:"column:created_by_user_id"`
	UpdatedByUserID *string `gorm:"column:updated_by_user_id"`
	CreatedAtRaw    string  `gorm:"column:created_at"`
	UpdatedAtRaw    string  `gorm:"column:updated_at"`
}

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

	query := r.db.WithContext(ctx).Table("document_template_scenes AS s")
	if keyword != "" {
		searchKeyword := "%" + keyword + "%"
		query = query.Where(
			"(LOWER(s.scene_key) LIKE ? OR LOWER(s.scene_name) LIKE ? OR LOWER(s.description) LIKE ?)",
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
			"s.scene_key",
			"s.scene_name",
			"s.description",
			"s.sort",
			"s.is_builtin",
			"s.updated_at",
			"COUNT(t.template_id) AS template_count",
		).
		Joins("LEFT JOIN document_templates AS t ON t.scene_key = s.scene_key").
		Group("s.scene_key, s.scene_name, s.description, s.sort, s.is_builtin, s.updated_at").
		Order("s.sort ASC").
		Order("s.scene_key ASC").
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

	var row documentTemplateSceneDetailRow
	if err := r.db.WithContext(ctx).
		Table("document_template_scenes AS s").
		Select(
			"s.scene_key",
			"s.scene_name",
			"s.description",
			"s.sort",
			"s.is_builtin",
			"s.created_by_user_id",
			"s.updated_by_user_id",
			"s.created_at",
			"s.updated_at",
		).
		Where("s.scene_key = ?", normalizedSceneKey).
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
		updateValues["scene_name"] = strings.TrimSpace(*params.SceneName)
	}
	if params.Description != nil {
		updateValues["description"] = strings.TrimSpace(*params.Description)
	}
	if params.Sort != nil {
		updateValues["sort"] = *params.Sort
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
		Model(&models.DocumentTemplateScene{}).
		Where("scene_key = ?", normalizedSceneKey).
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
		Where("scene_key = ?", normalizedSceneKey).
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

	var total int64
	if err := r.db.WithContext(ctx).
		Table("document_templates AS t").
		Where("t.scene_key = ?", normalizedSceneKey).
		Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}
