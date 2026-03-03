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

type deletedDocumentAttachmentCleanupCandidateRow struct {
	AttachmentID string `gorm:"column:attachment_id"`
	BlobID       string `gorm:"column:blob_id"`
}

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

	rows := make([]deletedDocumentAttachmentCleanupCandidateRow, 0, batchSize)
	if err := r.db.WithContext(ctx).
		Table("document_attachments AS da").
		Select("da.attachment_id, da.blob_id").
		Joins("JOIN documents AS d ON d.document_id = da.document_id").
		Where("d.status = ? OR d.deleted_at IS NOT NULL", models.EntityStatusDeleted).
		Order("da.id ASC").
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
