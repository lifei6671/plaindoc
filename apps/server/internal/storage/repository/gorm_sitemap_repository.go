package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/recordtime"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

type gormSitemapRepository struct {
	db *gorm.DB
}

type sitemapPublicDocumentSourceRow = sitemapPublicDocumentSourceRowDB

// NewGormSitemapRepository 创建 sitemap 仓储实现。
func NewGormSitemapRepository(db *gorm.DB) SitemapRepository {
	return &gormSitemapRepository{db: db}
}

func (r *gormSitemapRepository) ListPublicDocuments(
	ctx context.Context,
) ([]SitemapPublicDocumentSourceRecord, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("sitemap repository db is nil")
	}

	documentAlias := "d"
	nodeAlias := "n"
	spaceAlias := "s"
	rows := make([]sitemapPublicDocumentSourceRow, 0, 256)
	if err := r.db.WithContext(ctx).
		Table(tableWithAlias(models.Document{}, documentAlias)).
		Select(
			qualifiedColumn(spaceAlias, models.SpaceColumns.SpaceID)+" AS space_id",
			qualifiedColumn(spaceAlias, models.SpaceColumns.UpdatedAt)+" AS space_updated_at_raw",
			qualifiedColumn(documentAlias, models.DocumentColumns.DocumentID)+" AS document_id",
			qualifiedColumn(nodeAlias, models.NodeColumns.ReaderSlug)+" AS reader_slug",
			qualifiedColumn(documentAlias, models.DocumentColumns.Format)+" AS document_format",
			qualifiedColumn(documentAlias, models.DocumentColumns.ContentMD)+" AS document_content_md",
			qualifiedColumn(documentAlias, models.DocumentColumns.UpdatedAt)+" AS document_updated_at_raw",
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
		Where(qualifiedColumn(nodeAlias, models.NodeColumns.Type)+" = ?", models.NodeTypeDoc).
		Where(qualifiedColumn(spaceAlias, models.SpaceColumns.Visibility)+" = ?", models.VisibilityPublic).
		Where(qualifiedColumn(spaceAlias, models.SpaceColumns.Status)+" = ?", models.EntityStatusActive).
		Where(qualifiedColumn(spaceAlias, models.SpaceColumns.DeletedAt)+" IS NULL").
		Where(qualifiedColumn(documentAlias, models.DocumentColumns.Visibility)+" = ?", models.VisibilityPublic).
		Where(qualifiedColumn(documentAlias, models.DocumentColumns.Status)+" = ?", models.EntityStatusActive).
		Where(qualifiedColumn(documentAlias, models.DocumentColumns.DeletedAt)+" IS NULL").
		Where(qualifiedColumn(documentAlias, models.DocumentColumns.Format)+" = ?", models.DocumentFormatMarkdown).
		Order(qualifiedColumn(spaceAlias, models.SpaceColumns.SpaceID) + " ASC, " + qualifiedColumn(documentAlias, models.DocumentColumns.DocumentID) + " ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]SitemapPublicDocumentSourceRecord, 0, len(rows))
	for _, row := range rows {
		result = append(result, SitemapPublicDocumentSourceRecord{
			SpaceID:           strings.TrimSpace(row.SpaceID),
			DocumentID:        strings.TrimSpace(row.DocumentID),
			DocumentRouteKey:  resolveSitemapDocumentRouteKey(strings.TrimSpace(row.DocumentID), row.ReaderSlug),
			DocumentFormat:    models.NormalizeDocumentFormat(models.DocumentFormat(row.DocumentFormat)),
			DocumentContentMD: row.DocumentContentMD,
			SpaceUpdatedAt:    recordtime.Parse(row.SpaceUpdatedAtRaw),
			DocumentUpdatedAt: recordtime.Parse(row.DocumentUpdatedAtRaw),
		})
	}
	return result, nil
}

func resolveSitemapDocumentRouteKey(documentID string, readerSlug *string) string {
	if readerSlug != nil {
		normalizedReaderSlug := strings.ToLower(strings.TrimSpace(*readerSlug))
		if normalizedReaderSlug != "" {
			return normalizedReaderSlug
		}
	}
	return strings.TrimSpace(documentID)
}
