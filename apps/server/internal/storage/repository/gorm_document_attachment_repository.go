package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

type gormDocumentAttachmentRepository struct {
	db *gorm.DB
}

type documentAttachmentRow struct {
	ID              int64   `gorm:"column:id"`
	AttachmentID    string  `gorm:"column:attachment_id"`
	DocumentID      string  `gorm:"column:document_id"`
	SpaceID         string  `gorm:"column:space_id"`
	StorageProvider string  `gorm:"column:storage_provider"`
	FileName        string  `gorm:"column:file_name"`
	ObjectKey       string  `gorm:"column:object_key"`
	ObjectURL       string  `gorm:"column:object_url"`
	MimeType        string  `gorm:"column:mime_type"`
	SizeBytes       int64   `gorm:"column:size_bytes"`
	PreviewKind     string  `gorm:"column:preview_kind"`
	Status          string  `gorm:"column:status"`
	DeletedAtRaw    *string `gorm:"column:deleted_at"`
	CreatedByUserID *string `gorm:"column:created_by_user_id"`
	CreatedAtRaw    string  `gorm:"column:created_at"`
	UpdatedAtRaw    string  `gorm:"column:updated_at"`
}

// NewGormDocumentAttachmentRepository 创建基于 GORM 的文档附件仓储实现。
func NewGormDocumentAttachmentRepository(db *gorm.DB) DocumentAttachmentRepository {
	return &gormDocumentAttachmentRepository{db: db}
}

func (r *gormDocumentAttachmentRepository) Create(
	ctx context.Context,
	attachment *models.DocumentAttachment,
) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("document attachment repository db is nil")
	}
	if attachment == nil {
		return fmt.Errorf("document attachment is nil")
	}
	if !models.IsValidEntityStatus(attachment.Status) {
		attachment.Status = models.EntityStatusActive
	}
	return r.db.WithContext(ctx).Create(attachment).Error
}

func (r *gormDocumentAttachmentRepository) ListByDocumentID(
	ctx context.Context,
	documentID string,
	includeDeleted bool,
) ([]models.DocumentAttachment, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("document attachment repository db is nil")
	}

	normalizedDocumentID := strings.TrimSpace(documentID)
	if normalizedDocumentID == "" {
		return []models.DocumentAttachment{}, nil
	}

	query := r.db.WithContext(ctx).
		Table("document_attachments").
		Select(
			"id, attachment_id, document_id, space_id, storage_provider, file_name, object_key, object_url, mime_type, "+
				"size_bytes, preview_kind, status, deleted_at, created_by_user_id, created_at, updated_at",
		).
		Where("document_id = ?", normalizedDocumentID)
	if !includeDeleted {
		query = query.Where("status = ? AND deleted_at IS NULL", models.EntityStatusActive)
	}

	var rows []documentAttachmentRow
	if err := query.
		Order("created_at ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	attachments := make([]models.DocumentAttachment, 0, len(rows))
	for _, row := range rows {
		attachments = append(attachments, mapDocumentAttachmentRow(row))
	}

	for index := range attachments {
		if !models.IsValidEntityStatus(attachments[index].Status) {
			attachments[index].Status = models.EntityStatusActive
		}
	}
	return attachments, nil
}

func (r *gormDocumentAttachmentRepository) GetByAttachmentID(
	ctx context.Context,
	attachmentID string,
) (*models.DocumentAttachment, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("document attachment repository db is nil")
	}

	normalizedAttachmentID := strings.TrimSpace(attachmentID)
	if normalizedAttachmentID == "" {
		return nil, gorm.ErrRecordNotFound
	}

	var row documentAttachmentRow
	if err := r.db.WithContext(ctx).
		Table("document_attachments").
		Select(
			"id, attachment_id, document_id, space_id, storage_provider, file_name, object_key, object_url, mime_type, "+
				"size_bytes, preview_kind, status, deleted_at, created_by_user_id, created_at, updated_at",
		).
		Where("attachment_id = ?", normalizedAttachmentID).
		Take(&row).Error; err != nil {
		return nil, err
	}
	attachment := mapDocumentAttachmentRow(row)
	if !models.IsValidEntityStatus(attachment.Status) {
		attachment.Status = models.EntityStatusActive
	}
	return &attachment, nil
}

func (r *gormDocumentAttachmentRepository) SoftDelete(
	ctx context.Context,
	attachmentID string,
	deletedAt time.Time,
) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("document attachment repository db is nil")
	}

	normalizedAttachmentID := strings.TrimSpace(attachmentID)
	if normalizedAttachmentID == "" {
		return false, nil
	}
	if deletedAt.IsZero() {
		deletedAt = time.Now().UTC()
	}

	updateResult := r.db.WithContext(ctx).
		Model(&models.DocumentAttachment{}).
		Where("attachment_id = ? AND status <> ?", normalizedAttachmentID, models.EntityStatusDeleted).
		Updates(map[string]any{
			"status":     models.EntityStatusDeleted,
			"deleted_at": deletedAt,
			"updated_at": deletedAt,
		})
	if updateResult.Error != nil {
		return false, updateResult.Error
	}
	return updateResult.RowsAffected > 0, nil
}

func mapDocumentAttachmentRow(row documentAttachmentRow) models.DocumentAttachment {
	return models.DocumentAttachment{
		ID:              row.ID,
		AttachmentID:    strings.TrimSpace(row.AttachmentID),
		DocumentID:      strings.TrimSpace(row.DocumentID),
		SpaceID:         strings.TrimSpace(row.SpaceID),
		StorageProvider: strings.TrimSpace(row.StorageProvider),
		FileName:        strings.TrimSpace(row.FileName),
		ObjectKey:       strings.TrimSpace(row.ObjectKey),
		ObjectURL:       strings.TrimSpace(row.ObjectURL),
		MimeType:        strings.TrimSpace(row.MimeType),
		SizeBytes:       row.SizeBytes,
		PreviewKind:     strings.TrimSpace(row.PreviewKind),
		Status:          models.EntityStatus(strings.TrimSpace(row.Status)),
		DeletedAt:       parseNullableRecordTime(row.DeletedAtRaw),
		CreatedByUserID: row.CreatedByUserID,
		CreatedAt:       parseRecordTime(row.CreatedAtRaw),
		UpdatedAt:       parseRecordTime(row.UpdatedAtRaw),
	}
}
