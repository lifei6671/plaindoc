package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

type gormHomeSearchRepository struct {
	db *gorm.DB
}

type homeSearchMetadataRow struct {
	SpaceID      string `gorm:"column:space_id"`
	SpaceName    string `gorm:"column:space_name"`
	DocumentID   string `gorm:"column:document_id"`
	Title        string `gorm:"column:title"`
	UpdatedAtRaw string `gorm:"column:updated_at"`
}

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

	rows := make([]homeSearchMetadataRow, 0, len(normalizedDocumentIDs))
	if err := r.db.WithContext(ctx).
		Table("documents AS d").
		Select(
			"s.space_id AS space_id",
			"s.name AS space_name",
			"d.document_id AS document_id",
			"d.title AS title",
			"d.updated_at AS updated_at",
		).
		Joins("JOIN nodes AS n ON n.node_id = d.node_id").
		Joins("JOIN spaces AS s ON s.space_id = n.space_id").
		Where("d.document_id IN ?", normalizedDocumentIDs).
		Where("d.status = ? AND d.deleted_at IS NULL", models.EntityStatusActive).
		Where("s.status = ? AND s.deleted_at IS NULL", models.EntityStatusActive).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]HomeSearchDocumentMetadataRecord, 0, len(rows))
	for _, row := range rows {
		result = append(result, HomeSearchDocumentMetadataRecord{
			SpaceID:    strings.TrimSpace(row.SpaceID),
			SpaceName:  strings.TrimSpace(row.SpaceName),
			DocumentID: strings.TrimSpace(row.DocumentID),
			Title:      strings.TrimSpace(row.Title),
			UpdatedAt:  parseRecordTime(row.UpdatedAtRaw),
		})
	}
	return result, nil
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
