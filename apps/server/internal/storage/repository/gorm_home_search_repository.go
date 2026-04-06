package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/recordtime"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

type gormHomeSearchRepository struct {
	db *gorm.DB
}

type homeSearchMetadataRow = homeSearchMetadataRowDB

// NewGormHomeSearchRepository 创建首页检索仓储实现。
func NewGormHomeSearchRepository(db *gorm.DB) HomeSearchRepository {
	return &gormHomeSearchRepository{db: db}
}

func (r *gormHomeSearchRepository) ListActiveDocumentMetadataByDocumentIDs(
	ctx context.Context,
	documentIDs []string,
) ([]HomeSearchDocumentMetadataRecord, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("home search repository db is nil")
	}
	normalizedDocumentIDs := normalizeHomeSearchDocumentIDs(documentIDs)
	if len(normalizedDocumentIDs) == 0 {
		return []HomeSearchDocumentMetadataRecord{}, nil
	}

	documentAlias := "d"
	nodeAlias := "n"
	spaceAlias := "s"
	rows := make([]homeSearchMetadataRow, 0, len(normalizedDocumentIDs))
	if err := r.db.WithContext(ctx).
		Table(tableWithAlias(models.Document{}, documentAlias)).
		Select(
			qualifiedColumn(spaceAlias, models.SpaceColumns.SpaceID)+" AS space_id",
			qualifiedColumn(spaceAlias, models.SpaceColumns.Name)+" AS space_name",
			qualifiedColumn(documentAlias, models.DocumentColumns.DocumentID)+" AS document_id",
			qualifiedColumn(nodeAlias, models.NodeColumns.ReaderSlug)+" AS reader_slug",
			qualifiedColumn(documentAlias, models.DocumentColumns.Title)+" AS title",
			qualifiedColumn(documentAlias, models.DocumentColumns.UpdatedAt)+" AS updated_at_raw",
		).
		Joins(
			"JOIN "+tableName(models.Node{})+" AS "+nodeAlias+
				" ON "+qualifiedColumn(nodeAlias, models.NodeColumns.NodeID)+
				" = "+qualifiedColumn(documentAlias, models.DocumentColumns.NodeID),
		).
		Joins(
			"JOIN "+tableName(models.Space{})+" AS "+spaceAlias+
				" ON "+qualifiedColumn(spaceAlias, models.SpaceColumns.SpaceID)+
				" = "+qualifiedColumn(nodeAlias, models.NodeColumns.SpaceID),
		).
		Where(qualifiedColumn(documentAlias, models.DocumentColumns.DocumentID)+" IN ?", normalizedDocumentIDs).
		Where(qualifiedColumn(documentAlias, models.DocumentColumns.Status)+" = ?", models.EntityStatusActive).
		Where(qualifiedColumn(documentAlias, models.DocumentColumns.DeletedAt)+" IS NULL").
		Where(qualifiedColumn(documentAlias, models.DocumentColumns.Format)+" = ?", models.DocumentFormatMarkdown).
		Where(qualifiedColumn(spaceAlias, models.SpaceColumns.Status)+" = ?", models.EntityStatusActive).
		Where(qualifiedColumn(spaceAlias, models.SpaceColumns.DeletedAt) + " IS NULL").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]HomeSearchDocumentMetadataRecord, 0, len(rows))
	for _, row := range rows {
		result = append(result, HomeSearchDocumentMetadataRecord{
			SpaceID:          strings.TrimSpace(row.SpaceID),
			SpaceName:        strings.TrimSpace(row.SpaceName),
			DocumentID:       strings.TrimSpace(row.DocumentID),
			DocumentRouteKey: resolveDocumentRouteKey(strings.TrimSpace(row.DocumentID), row.ReaderSlug),
			Title:            strings.TrimSpace(row.Title),
			UpdatedAt:        recordtime.Parse(row.UpdatedAtRaw),
		})
	}
	return result, nil
}

func resolveDocumentRouteKey(documentID string, readerSlug *string) string {
	slug := strings.ToLower(strings.TrimSpace(derefOptionalString(readerSlug)))
	if slug != "" {
		return slug
	}
	return strings.TrimSpace(documentID)
}

func derefOptionalString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func normalizeHomeSearchDocumentIDs(documentIDs []string) []string {
	if len(documentIDs) == 0 {
		return []string{}
	}
	result := make([]string, 0, len(documentIDs))
	seen := make(map[string]struct{}, len(documentIDs))
	for _, item := range documentIDs {
		documentID := strings.TrimSpace(item)
		if documentID == "" {
			continue
		}
		if _, exists := seen[documentID]; exists {
			continue
		}
		seen[documentID] = struct{}{}
		result = append(result, documentID)
	}
	return result
}
