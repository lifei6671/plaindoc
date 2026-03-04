package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

type gormSitemapRepository struct {
	db *gorm.DB
}

type sitemapPublicDocumentSourceRow struct {
	SpaceID              string  `gorm:"column:space_id"`
	SpaceUpdatedAtRaw    string  `gorm:"column:space_updated_at"`
	DocumentID           string  `gorm:"column:document_id"`
	ReaderSlug           *string `gorm:"column:reader_slug"`
	DocumentContentMD    string  `gorm:"column:document_content_md"`
	DocumentUpdatedAtRaw string  `gorm:"column:document_updated_at"`
}

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

	rows := make([]sitemapPublicDocumentSourceRow, 0, 256)
	if err := r.db.WithContext(ctx).
		Table("documents AS d").
		Select(
			"s.space_id AS space_id",
			"s.updated_at AS space_updated_at",
			"d.document_id AS document_id",
			"n.reader_slug AS reader_slug",
			"d.content_md AS document_content_md",
			"d.updated_at AS document_updated_at",
		).
		Joins("JOIN nodes AS n ON n.node_id = d.node_id").
		Joins("JOIN spaces AS s ON s.space_id = n.space_id").
		Where("n.type = ?", models.NodeTypeDoc).
		Where("s.visibility = ?", models.VisibilityPublic).
		Where("s.status = ?", models.EntityStatusActive).
		Where("s.deleted_at IS NULL").
		Where("d.visibility = ?", models.VisibilityPublic).
		Where("d.status = ?", models.EntityStatusActive).
		Where("d.deleted_at IS NULL").
		Order("s.space_id ASC, d.document_id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]SitemapPublicDocumentSourceRecord, 0, len(rows))
	for _, row := range rows {
		result = append(result, SitemapPublicDocumentSourceRecord{
			SpaceID:           strings.TrimSpace(row.SpaceID),
			DocumentID:        strings.TrimSpace(row.DocumentID),
			DocumentRouteKey:  resolveSitemapDocumentRouteKey(strings.TrimSpace(row.DocumentID), row.ReaderSlug),
			DocumentContentMD: row.DocumentContentMD,
			SpaceUpdatedAt:    parseRecordTime(row.SpaceUpdatedAtRaw),
			DocumentUpdatedAt: parseRecordTime(row.DocumentUpdatedAtRaw),
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
