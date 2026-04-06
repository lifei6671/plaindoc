package repository

import (
	"context"
	"fmt"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

type gormDocumentAttachmentCleanupRepository struct {
	db *gorm.DB
}

type deletedDocumentAttachmentCleanupCandidateRow = deletedDocumentAttachmentCleanupCandidateRowDB

// NewGormDocumentAttachmentCleanupRepository 创建文档附件清理仓储实现。
func NewGormDocumentAttachmentCleanupRepository(db *gorm.DB) DocumentAttachmentCleanupRepository {
	return &gormDocumentAttachmentCleanupRepository{db: db}
}

func (r *gormDocumentAttachmentCleanupRepository) ListDeletedDocumentAttachmentCandidates(
	ctx context.Context,
	batchSize int,
) ([]DeletedDocumentAttachmentCleanupCandidate, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("document attachment cleanup repository db is nil")
	}
	if batchSize <= 0 {
		return []DeletedDocumentAttachmentCleanupCandidate{}, nil
	}

	attachmentAlias := "da"
	documentAlias := "d"
	rows := make([]deletedDocumentAttachmentCleanupCandidateRow, 0, batchSize)
	if err := r.db.WithContext(ctx).
		Table(tableWithAlias(models.DocumentAttachment{}, attachmentAlias)).
		Select(
			qualifiedColumn(attachmentAlias, models.DocumentAttachmentColumns.AttachmentID)+" AS AttachmentID",
			qualifiedColumn(attachmentAlias, models.DocumentAttachmentColumns.BlobID)+" AS BlobID",
		).
		Joins(
			"JOIN "+tableName(models.Document{})+" AS "+documentAlias+
				" ON "+qualifiedColumn(documentAlias, models.DocumentColumns.DocumentID)+
				" = "+qualifiedColumn(attachmentAlias, models.DocumentAttachmentColumns.DocumentID),
		).
		Where(qualifiedColumn(documentAlias, models.DocumentColumns.Status)+" = ?", models.EntityStatusDeleted).
		Or(qualifiedColumn(documentAlias, models.DocumentColumns.DeletedAt) + " IS NOT NULL").
		Order(qualifiedColumn(attachmentAlias, models.DocumentAttachmentColumns.ID) + " ASC").
		Limit(batchSize).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]DeletedDocumentAttachmentCleanupCandidate, 0, len(rows))
	for _, row := range rows {
		result = append(result, DeletedDocumentAttachmentCleanupCandidate{
			AttachmentID: row.AttachmentID,
			BlobID:       row.BlobID,
		})
	}
	return result, nil
}
