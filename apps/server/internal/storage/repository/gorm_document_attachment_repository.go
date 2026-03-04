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
	BlobID          string  `gorm:"column:blob_id"`
	DocumentID      string  `gorm:"column:document_id"`
	SpaceID         string  `gorm:"column:space_id"`
	StorageProvider string  `gorm:"column:storage_provider"`
	FileName        string  `gorm:"column:file_name"`
	ObjectKey       string  `gorm:"column:object_key"`
	ObjectURL       string  `gorm:"column:object_url"`
	MimeType        string  `gorm:"column:mime_type"`
	SizeBytes       int64   `gorm:"column:size_bytes"`
	ContentHashAlgo string  `gorm:"column:content_hash_algo"`
	ContentHash     string  `gorm:"column:content_hash"`
	PreviewKind     string  `gorm:"column:preview_kind"`
	Status          string  `gorm:"column:status"`
	DeletedAtRaw    *string `gorm:"column:deleted_at"`
	CreatedByUserID *string `gorm:"column:created_by_user_id"`
	CreatedAtRaw    string  `gorm:"column:created_at"`
	UpdatedAtRaw    string  `gorm:"column:updated_at"`
}

type documentAttachmentBlobRow struct {
	ID              int64   `gorm:"column:id"`
	BlobID          string  `gorm:"column:blob_id"`
	StorageProvider string  `gorm:"column:storage_provider"`
	ObjectKey       string  `gorm:"column:object_key"`
	ObjectURL       string  `gorm:"column:object_url"`
	MimeType        string  `gorm:"column:mime_type"`
	SizeBytes       int64   `gorm:"column:size_bytes"`
	ContentHashAlgo string  `gorm:"column:content_hash_algo"`
	ContentHash     string  `gorm:"column:content_hash"`
	DeletedAtRaw    *string `gorm:"column:deleted_at"`
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
			"id, attachment_id, blob_id, document_id, space_id, storage_provider, file_name, object_key, object_url, mime_type, "+
				"size_bytes, content_hash_algo, content_hash, preview_kind, status, deleted_at, created_by_user_id, created_at, updated_at",
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

func (r *gormDocumentAttachmentRepository) ListForAdmin(
	ctx context.Context,
	params ListAdminDocumentAttachmentsParams,
) ([]AdminDocumentAttachmentListRecord, int64, error) {
	if r == nil || r.db == nil {
		return nil, 0, fmt.Errorf("document attachment repository db is nil")
	}

	baseQuery := r.db.WithContext(ctx).
		Table("document_attachments AS da").
		Joins("JOIN documents AS d ON d.document_id = da.document_id").
		Joins("JOIN nodes AS n ON n.node_id = d.node_id").
		Joins("JOIN spaces AS s ON s.space_id = n.space_id").
		Joins("JOIN users AS uo ON uo.user_id = s.owner_user_id").
		Joins("LEFT JOIN users AS uc ON uc.user_id = da.created_by_user_id")

	if params.RestrictToScopes {
		actorUserID := strings.TrimSpace(params.ActorUserID)
		baseQuery = baseQuery.Where(
			"(s.owner_user_id = ? OR EXISTS (SELECT 1 FROM space_admin_scopes AS sas WHERE sas.space_id = s.space_id AND sas.user_id = ?))",
			actorUserID,
			actorUserID,
		)
	}

	keyword := strings.ToLower(strings.TrimSpace(params.Keyword))
	if keyword != "" {
		likeKeyword := "%" + keyword + "%"
		baseQuery = baseQuery.Where(
			"LOWER(da.attachment_id) LIKE ? OR LOWER(da.document_id) LIKE ? OR LOWER(da.file_name) LIKE ? OR LOWER(da.object_key) LIKE ? OR LOWER(da.mime_type) LIKE ? OR LOWER(d.title) LIKE ? OR LOWER(s.space_id) LIKE ? OR LOWER(s.name) LIKE ? OR LOWER(uo.user_id) LIKE ? OR LOWER(uo.email) LIKE ? OR LOWER(uo.name) LIKE ? OR LOWER(COALESCE(uc.user_id,'')) LIKE ? OR LOWER(COALESCE(uc.email,'')) LIKE ? OR LOWER(COALESCE(uc.name,'')) LIKE ?",
			likeKeyword,
			likeKeyword,
			likeKeyword,
			likeKeyword,
			likeKeyword,
			likeKeyword,
			likeKeyword,
			likeKeyword,
			likeKeyword,
			likeKeyword,
			likeKeyword,
			likeKeyword,
			likeKeyword,
			likeKeyword,
		)
	}

	spaceID := strings.TrimSpace(params.SpaceID)
	if spaceID != "" {
		baseQuery = baseQuery.Where("s.space_id = ?", spaceID)
	}

	documentID := strings.TrimSpace(params.DocumentID)
	if documentID != "" {
		baseQuery = baseQuery.Where("da.document_id = ?", documentID)
	}

	statuses := normalizeAttachmentStatuses(params.Statuses)
	if len(statuses) > 0 {
		baseQuery = baseQuery.Where("da.status IN ?", statuses)
	}

	storageProviders := normalizeAttachmentStorageProviders(params.StorageProviders)
	if len(storageProviders) > 0 {
		baseQuery = baseQuery.Where("LOWER(da.storage_provider) IN ?", storageProviders)
	}

	var total int64
	if err := baseQuery.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := params.Offset
	if offset < 0 {
		offset = 0
	}

	type adminDocumentAttachmentListRow struct {
		ID              int64               `gorm:"column:id"`
		AttachmentID    string              `gorm:"column:attachment_id"`
		BlobID          string              `gorm:"column:blob_id"`
		DocumentID      string              `gorm:"column:document_id"`
		ReaderSlug      *string             `gorm:"column:reader_slug"`
		SpaceID         string              `gorm:"column:space_id"`
		StorageProvider string              `gorm:"column:storage_provider"`
		FileName        string              `gorm:"column:file_name"`
		ObjectKey       string              `gorm:"column:object_key"`
		ObjectURL       string              `gorm:"column:object_url"`
		MimeType        string              `gorm:"column:mime_type"`
		SizeBytes       int64               `gorm:"column:size_bytes"`
		ContentHashAlgo string              `gorm:"column:content_hash_algo"`
		ContentHash     string              `gorm:"column:content_hash"`
		PreviewKind     string              `gorm:"column:preview_kind"`
		Status          models.EntityStatus `gorm:"column:status"`
		DeletedAtRaw    *string             `gorm:"column:deleted_at"`
		CreatedByUserID *string             `gorm:"column:created_by_user_id"`
		CreatedAtRaw    string              `gorm:"column:created_at"`
		UpdatedAtRaw    string              `gorm:"column:updated_at"`

		DocumentTitle  string              `gorm:"column:document_title"`
		DocumentStatus models.EntityStatus `gorm:"column:document_status"`
		SpaceName      string              `gorm:"column:space_name"`
		SpaceOwnerID   string              `gorm:"column:space_owner_user_id"`
		SpaceOwnerName string              `gorm:"column:space_owner_name"`
		SpaceOwnerMail string              `gorm:"column:space_owner_email"`
		CreatedByName  string              `gorm:"column:created_by_name"`
		CreatedByEmail string              `gorm:"column:created_by_email"`
	}

	var rows []adminDocumentAttachmentListRow
	if err := baseQuery.Session(&gorm.Session{}).
		Select(
			"da.id",
			"da.attachment_id",
			"da.blob_id",
			"da.document_id",
			"n.reader_slug AS reader_slug",
			"da.space_id",
			"da.storage_provider",
			"da.file_name",
			"da.object_key",
			"da.object_url",
			"da.mime_type",
			"da.size_bytes",
			"da.content_hash_algo",
			"da.content_hash",
			"da.preview_kind",
			"da.status",
			"da.deleted_at",
			"da.created_by_user_id",
			"da.created_at",
			"da.updated_at",
			"d.title AS document_title",
			"d.status AS document_status",
			"s.name AS space_name",
			"s.owner_user_id AS space_owner_user_id",
			"uo.name AS space_owner_name",
			"uo.email AS space_owner_email",
			"COALESCE(uc.name,'') AS created_by_name",
			"COALESCE(uc.email,'') AS created_by_email",
		).
		Order("da.created_at DESC, da.id DESC").
		Offset(offset).
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	result := make([]AdminDocumentAttachmentListRecord, 0, len(rows))
	for _, row := range rows {
		attachment := models.DocumentAttachment{
			ID:              row.ID,
			AttachmentID:    strings.TrimSpace(row.AttachmentID),
			BlobID:          strings.TrimSpace(row.BlobID),
			DocumentID:      strings.TrimSpace(row.DocumentID),
			SpaceID:         strings.TrimSpace(row.SpaceID),
			StorageProvider: strings.TrimSpace(row.StorageProvider),
			FileName:        strings.TrimSpace(row.FileName),
			ObjectKey:       strings.TrimSpace(row.ObjectKey),
			ObjectURL:       strings.TrimSpace(row.ObjectURL),
			MimeType:        strings.TrimSpace(row.MimeType),
			SizeBytes:       row.SizeBytes,
			ContentHashAlgo: strings.TrimSpace(row.ContentHashAlgo),
			ContentHash:     strings.TrimSpace(row.ContentHash),
			PreviewKind:     strings.TrimSpace(row.PreviewKind),
			Status:          row.Status,
			DeletedAt:       parseNullableRecordTime(row.DeletedAtRaw),
			CreatedByUserID: row.CreatedByUserID,
			CreatedAt:       parseRecordTime(row.CreatedAtRaw),
			UpdatedAt:       parseRecordTime(row.UpdatedAtRaw),
		}
		if !models.IsValidEntityStatus(attachment.Status) {
			attachment.Status = models.EntityStatusActive
		}

		documentStatus := row.DocumentStatus
		if !models.IsValidEntityStatus(documentStatus) {
			documentStatus = models.EntityStatusActive
		}

		result = append(result, AdminDocumentAttachmentListRecord{
			Attachment:       attachment,
			DocumentRouteKey: resolveAdminDocumentRouteKey(row.DocumentID, row.ReaderSlug),
			DocumentTitle:    strings.TrimSpace(row.DocumentTitle),
			DocumentStatus:   documentStatus,
			SpaceName:        strings.TrimSpace(row.SpaceName),
			SpaceOwnerID:     strings.TrimSpace(row.SpaceOwnerID),
			SpaceOwnerName:   strings.TrimSpace(row.SpaceOwnerName),
			SpaceOwnerEmail:  strings.TrimSpace(row.SpaceOwnerMail),
			CreatedByName:    strings.TrimSpace(row.CreatedByName),
			CreatedByEmail:   strings.TrimSpace(row.CreatedByEmail),
		})
	}

	return result, total, nil
}

func (r *gormDocumentAttachmentRepository) FindBlobByHash(
	ctx context.Context,
	storageProvider string,
	contentHashAlgo string,
	contentHash string,
	sizeBytes int64,
) (*models.DocumentAttachmentBlob, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("document attachment repository db is nil")
	}

	normalizedStorageProvider := strings.ToLower(strings.TrimSpace(storageProvider))
	normalizedHashAlgo := strings.ToLower(strings.TrimSpace(contentHashAlgo))
	normalizedHash := strings.ToLower(strings.TrimSpace(contentHash))
	if normalizedStorageProvider == "" || normalizedHashAlgo == "" || normalizedHash == "" || sizeBytes < 0 {
		return nil, gorm.ErrRecordNotFound
	}

	var row documentAttachmentBlobRow
	if err := r.db.WithContext(ctx).
		Table("file_blobs").
		Select(
			"id, blob_id, storage_provider, object_key, object_url, mime_type, size_bytes, "+
				"content_hash_algo, content_hash, deleted_at, created_at, updated_at",
		).
		Where(
			"storage_provider = ? AND content_hash_algo = ? AND content_hash = ? AND size_bytes = ? AND deleted_at IS NULL",
			normalizedStorageProvider,
			normalizedHashAlgo,
			normalizedHash,
			sizeBytes,
		).
		Take(&row).Error; err != nil {
		return nil, err
	}

	blob := mapDocumentAttachmentBlobRow(row)
	return &blob, nil
}

func (r *gormDocumentAttachmentRepository) GetBlobByBlobID(
	ctx context.Context,
	blobID string,
) (*models.DocumentAttachmentBlob, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("document attachment repository db is nil")
	}

	normalizedBlobID := strings.TrimSpace(blobID)
	if normalizedBlobID == "" {
		return nil, gorm.ErrRecordNotFound
	}

	var row documentAttachmentBlobRow
	if err := r.db.WithContext(ctx).
		Table("file_blobs").
		Select(
			"id, blob_id, storage_provider, object_key, object_url, mime_type, size_bytes, "+
				"content_hash_algo, content_hash, deleted_at, created_at, updated_at",
		).
		Where("blob_id = ?", normalizedBlobID).
		Take(&row).Error; err != nil {
		return nil, err
	}

	blob := mapDocumentAttachmentBlobRow(row)
	return &blob, nil
}

func (r *gormDocumentAttachmentRepository) ListOrphanBlobs(
	ctx context.Context,
	limit int,
) ([]models.DocumentAttachmentBlob, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("document attachment repository db is nil")
	}

	if limit <= 0 {
		limit = 200
	}
	if limit > 2000 {
		limit = 2000
	}

	rows := make([]documentAttachmentBlobRow, 0, limit)
	if err := r.db.WithContext(ctx).
		Table("file_blobs").
		Select(
			"id, blob_id, storage_provider, object_key, object_url, mime_type, size_bytes, " +
				"content_hash_algo, content_hash, deleted_at, created_at, updated_at",
		).
		Where("deleted_at IS NULL").
		Where("NOT EXISTS (SELECT 1 FROM document_attachments WHERE document_attachments.blob_id = file_blobs.blob_id)").
		Order("created_at ASC, id ASC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	blobs := make([]models.DocumentAttachmentBlob, 0, len(rows))
	for _, row := range rows {
		blobs = append(blobs, mapDocumentAttachmentBlobRow(row))
	}
	return blobs, nil
}

func (r *gormDocumentAttachmentRepository) CreateBlob(
	ctx context.Context,
	blob *models.DocumentAttachmentBlob,
) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("document attachment repository db is nil")
	}
	if blob == nil {
		return fmt.Errorf("document attachment blob is nil")
	}
	return r.db.WithContext(ctx).Create(blob).Error
}

func (r *gormDocumentAttachmentRepository) HardDeleteBlobIfUnreferenced(
	ctx context.Context,
	blobID string,
) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("document attachment repository db is nil")
	}

	normalizedBlobID := strings.TrimSpace(blobID)
	if normalizedBlobID == "" {
		return false, nil
	}

	deleteResult := r.db.WithContext(ctx).
		Where(
			"blob_id = ? AND NOT EXISTS (SELECT 1 FROM document_attachments WHERE blob_id = ?)",
			normalizedBlobID,
			normalizedBlobID,
		).
		Delete(&models.DocumentAttachmentBlob{})
	if deleteResult.Error != nil {
		return false, deleteResult.Error
	}
	return deleteResult.RowsAffected > 0, nil
}

func (r *gormDocumentAttachmentRepository) CountActiveReferencesByBlobID(
	ctx context.Context,
	blobID string,
) (int64, error) {
	if r == nil || r.db == nil {
		return 0, fmt.Errorf("document attachment repository db is nil")
	}

	normalizedBlobID := strings.TrimSpace(blobID)
	if normalizedBlobID == "" {
		return 0, nil
	}

	var total int64
	if err := r.db.WithContext(ctx).
		Table("document_attachments").
		Where("blob_id = ? AND status = ? AND deleted_at IS NULL", normalizedBlobID, models.EntityStatusActive).
		Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (r *gormDocumentAttachmentRepository) ListActiveReferencesByBlobID(
	ctx context.Context,
	blobID string,
	limit int,
) ([]DocumentAttachmentReferenceRecord, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("document attachment repository db is nil")
	}

	normalizedBlobID := strings.TrimSpace(blobID)
	if normalizedBlobID == "" {
		return []DocumentAttachmentReferenceRecord{}, nil
	}

	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	type referenceRow struct {
		AttachmentID  string              `gorm:"column:attachment_id"`
		DocumentID    string              `gorm:"column:document_id"`
		DocumentTitle string              `gorm:"column:document_title"`
		SpaceID       string              `gorm:"column:space_id"`
		SpaceName     string              `gorm:"column:space_name"`
		FileName      string              `gorm:"column:file_name"`
		Status        models.EntityStatus `gorm:"column:status"`
	}

	rows := make([]referenceRow, 0, limit)
	if err := r.db.WithContext(ctx).
		Table("document_attachments AS da").
		Select(
			"da.attachment_id",
			"da.document_id",
			"d.title AS document_title",
			"da.space_id",
			"s.name AS space_name",
			"da.file_name",
			"da.status",
		).
		Joins("JOIN documents AS d ON d.document_id = da.document_id").
		Joins("JOIN spaces AS s ON s.space_id = da.space_id").
		Where("da.blob_id = ? AND da.status = ? AND da.deleted_at IS NULL", normalizedBlobID, models.EntityStatusActive).
		Order("da.created_at ASC, da.id ASC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]DocumentAttachmentReferenceRecord, 0, len(rows))
	for _, row := range rows {
		status := row.Status
		if !models.IsValidEntityStatus(status) {
			status = models.EntityStatusActive
		}
		result = append(result, DocumentAttachmentReferenceRecord{
			AttachmentID:  strings.TrimSpace(row.AttachmentID),
			DocumentID:    strings.TrimSpace(row.DocumentID),
			DocumentTitle: strings.TrimSpace(row.DocumentTitle),
			SpaceID:       strings.TrimSpace(row.SpaceID),
			SpaceName:     strings.TrimSpace(row.SpaceName),
			FileName:      strings.TrimSpace(row.FileName),
			Status:        status,
		})
	}
	return result, nil
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
			"id, attachment_id, blob_id, document_id, space_id, storage_provider, file_name, object_key, object_url, mime_type, "+
				"size_bytes, content_hash_algo, content_hash, preview_kind, status, deleted_at, created_by_user_id, created_at, updated_at",
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

func (r *gormDocumentAttachmentRepository) HardDelete(
	ctx context.Context,
	attachmentID string,
) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("document attachment repository db is nil")
	}

	normalizedAttachmentID := strings.TrimSpace(attachmentID)
	if normalizedAttachmentID == "" {
		return false, nil
	}

	deleteResult := r.db.WithContext(ctx).
		Where("attachment_id = ?", normalizedAttachmentID).
		Delete(&models.DocumentAttachment{})
	if deleteResult.Error != nil {
		return false, deleteResult.Error
	}
	return deleteResult.RowsAffected > 0, nil
}

func mapDocumentAttachmentRow(row documentAttachmentRow) models.DocumentAttachment {
	return models.DocumentAttachment{
		ID:              row.ID,
		AttachmentID:    strings.TrimSpace(row.AttachmentID),
		BlobID:          strings.TrimSpace(row.BlobID),
		DocumentID:      strings.TrimSpace(row.DocumentID),
		SpaceID:         strings.TrimSpace(row.SpaceID),
		StorageProvider: strings.TrimSpace(row.StorageProvider),
		FileName:        strings.TrimSpace(row.FileName),
		ObjectKey:       strings.TrimSpace(row.ObjectKey),
		ObjectURL:       strings.TrimSpace(row.ObjectURL),
		MimeType:        strings.TrimSpace(row.MimeType),
		SizeBytes:       row.SizeBytes,
		ContentHashAlgo: strings.TrimSpace(row.ContentHashAlgo),
		ContentHash:     strings.TrimSpace(row.ContentHash),
		PreviewKind:     strings.TrimSpace(row.PreviewKind),
		Status:          models.EntityStatus(strings.TrimSpace(row.Status)),
		DeletedAt:       parseNullableRecordTime(row.DeletedAtRaw),
		CreatedByUserID: row.CreatedByUserID,
		CreatedAt:       parseRecordTime(row.CreatedAtRaw),
		UpdatedAt:       parseRecordTime(row.UpdatedAtRaw),
	}
}

func mapDocumentAttachmentBlobRow(row documentAttachmentBlobRow) models.DocumentAttachmentBlob {
	return models.DocumentAttachmentBlob{
		ID:              row.ID,
		BlobID:          strings.TrimSpace(row.BlobID),
		StorageProvider: strings.TrimSpace(row.StorageProvider),
		ObjectKey:       strings.TrimSpace(row.ObjectKey),
		ObjectURL:       strings.TrimSpace(row.ObjectURL),
		MimeType:        strings.TrimSpace(row.MimeType),
		SizeBytes:       row.SizeBytes,
		ContentHashAlgo: strings.TrimSpace(row.ContentHashAlgo),
		ContentHash:     strings.TrimSpace(row.ContentHash),
		DeletedAt:       parseNullableRecordTime(row.DeletedAtRaw),
		CreatedAt:       parseRecordTime(row.CreatedAtRaw),
		UpdatedAt:       parseRecordTime(row.UpdatedAtRaw),
	}
}

func normalizeAttachmentStatuses(statuses []models.EntityStatus) []models.EntityStatus {
	if len(statuses) == 0 {
		return nil
	}
	result := make([]models.EntityStatus, 0, len(statuses))
	seen := make(map[models.EntityStatus]struct{}, len(statuses))
	for _, status := range statuses {
		if !models.IsValidEntityStatus(status) {
			continue
		}
		if _, exists := seen[status]; exists {
			continue
		}
		seen[status] = struct{}{}
		result = append(result, status)
	}
	return result
}

func normalizeAttachmentStorageProviders(storageProviders []string) []string {
	if len(storageProviders) == 0 {
		return nil
	}
	result := make([]string, 0, len(storageProviders))
	seen := make(map[string]struct{}, len(storageProviders))
	for _, storageProvider := range storageProviders {
		value := strings.ToLower(strings.TrimSpace(storageProvider))
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
