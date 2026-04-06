package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/recordtime"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

type gormDocumentAttachmentRepository struct {
	db *gorm.DB
}

type documentAttachmentRow = documentAttachmentRowDB

type documentAttachmentBlobRow = documentAttachmentBlobRowDB

type adminDocumentAttachmentListRow = adminDocumentAttachmentListRowDB

type documentAttachmentReferenceRow = documentAttachmentReferenceRowDB

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
		Model(&models.DocumentAttachment{}).
		Select(
			models.DocumentAttachmentColumns.ID,
			models.DocumentAttachmentColumns.AttachmentID,
			models.DocumentAttachmentColumns.BlobID,
			models.DocumentAttachmentColumns.DocumentID,
			models.DocumentAttachmentColumns.SpaceID,
			models.DocumentAttachmentColumns.StorageProvider,
			models.DocumentAttachmentColumns.FileName,
			models.DocumentAttachmentColumns.ObjectKey,
			models.DocumentAttachmentColumns.ObjectURL,
			models.DocumentAttachmentColumns.MimeType,
			models.DocumentAttachmentColumns.SizeBytes,
			models.DocumentAttachmentColumns.ContentHashAlgo,
			models.DocumentAttachmentColumns.ContentHash,
			models.DocumentAttachmentColumns.PreviewKind,
			models.DocumentAttachmentColumns.Status,
			models.DocumentAttachmentColumns.DeletedAt+" AS DeletedAtRaw",
			models.DocumentAttachmentColumns.CreatedByUserID,
			models.DocumentAttachmentColumns.CreatedAt+" AS CreatedAtRaw",
			models.DocumentAttachmentColumns.UpdatedAt+" AS UpdatedAtRaw",
		).
		Where(models.DocumentAttachmentColumns.DocumentID+" = ?", normalizedDocumentID)
	if !includeDeleted {
		query = query.Where(models.DocumentAttachmentColumns.Status+" = ?", models.EntityStatusActive).
			Where(models.DocumentAttachmentColumns.DeletedAt + " IS NULL")
	}

	var rows []documentAttachmentRow
	if err := query.
		Order(models.DocumentAttachmentColumns.CreatedAt + " ASC").
		Order(models.DocumentAttachmentColumns.ID + " ASC").
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
		Table(tableWithAlias(models.DocumentAttachment{}, "da")).
		Joins("JOIN " + tableName(models.Document{}) + " AS d ON d." + models.DocumentColumns.DocumentID + " = da." + models.DocumentAttachmentColumns.DocumentID).
		Joins("JOIN " + tableName(models.Node{}) + " AS n ON n." + models.NodeColumns.NodeID + " = d." + models.DocumentColumns.NodeID).
		Joins("JOIN " + tableName(models.Space{}) + " AS s ON s." + models.SpaceColumns.SpaceID + " = n." + models.NodeColumns.SpaceID).
		Joins("JOIN " + tableName(models.User{}) + " AS uo ON uo." + models.UserColumns.UserID + " = s." + models.SpaceColumns.OwnerUserID).
		Joins("LEFT JOIN " + tableName(models.User{}) + " AS uc ON uc." + models.UserColumns.UserID + " = da." + models.DocumentAttachmentColumns.CreatedByUserID)

	if params.RestrictToScopes {
		actorUserID := strings.TrimSpace(params.ActorUserID)
		spaceAdminScopeQuery := r.db.WithContext(ctx).
			Model(&models.SpaceAdminScope{}).
			Select("1").
			Where(qualifiedColumn("", models.SpaceAdminScopeColumns.SpaceID)+" = "+qualifiedColumn("s", models.SpaceColumns.SpaceID)).
			Where(qualifiedColumn("", models.SpaceAdminScopeColumns.UserID)+" = ?", actorUserID)
		baseQuery = baseQuery.Where(
			"("+qualifiedColumn("s", models.SpaceColumns.OwnerUserID)+" = ? OR EXISTS (?))",
			actorUserID,
			spaceAdminScopeQuery,
		)
	}

	keyword := strings.ToLower(strings.TrimSpace(params.Keyword))
	if keyword != "" {
		likeKeyword := "%" + keyword + "%"
		baseQuery = baseQuery.Where(
			"LOWER("+qualifiedColumn("da", models.DocumentAttachmentColumns.AttachmentID)+") LIKE ? OR LOWER("+qualifiedColumn("da", models.DocumentAttachmentColumns.DocumentID)+") LIKE ? OR LOWER("+qualifiedColumn("da", models.DocumentAttachmentColumns.FileName)+") LIKE ? OR LOWER("+qualifiedColumn("da", models.DocumentAttachmentColumns.ObjectKey)+") LIKE ? OR LOWER("+qualifiedColumn("da", models.DocumentAttachmentColumns.MimeType)+") LIKE ? OR LOWER("+qualifiedColumn("d", models.DocumentColumns.Title)+") LIKE ? OR LOWER("+qualifiedColumn("s", models.SpaceColumns.SpaceID)+") LIKE ? OR LOWER("+qualifiedColumn("s", models.SpaceColumns.Name)+") LIKE ? OR LOWER("+qualifiedColumn("uo", models.UserColumns.UserID)+") LIKE ? OR LOWER("+qualifiedColumn("uo", models.UserColumns.Email)+") LIKE ? OR LOWER("+qualifiedColumn("uo", models.UserColumns.Name)+") LIKE ? OR LOWER(COALESCE("+qualifiedColumn("uc", models.UserColumns.UserID)+",'')) LIKE ? OR LOWER(COALESCE("+qualifiedColumn("uc", models.UserColumns.Email)+",'')) LIKE ? OR LOWER(COALESCE("+qualifiedColumn("uc", models.UserColumns.Name)+",'')) LIKE ?",
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
		baseQuery = baseQuery.Where(qualifiedColumn("s", models.SpaceColumns.SpaceID)+" = ?", spaceID)
	}

	documentID := strings.TrimSpace(params.DocumentID)
	if documentID != "" {
		baseQuery = baseQuery.Where("da."+models.DocumentAttachmentColumns.DocumentID+" = ?", documentID)
	}

	statuses := normalizeAttachmentStatuses(params.Statuses)
	if len(statuses) > 0 {
		baseQuery = baseQuery.Where("da."+models.DocumentAttachmentColumns.Status+" IN ?", statuses)
	}

	storageProviders := normalizeAttachmentStorageProviders(params.StorageProviders)
	if len(storageProviders) > 0 {
		baseQuery = baseQuery.Where("LOWER(da."+models.DocumentAttachmentColumns.StorageProvider+") IN ?", storageProviders)
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

	var rows []adminDocumentAttachmentListRow
	if err := baseQuery.Session(&gorm.Session{}).
		Select(
			"da."+models.DocumentAttachmentColumns.ID+" AS "+models.DocumentAttachmentColumns.ID,
			"da."+models.DocumentAttachmentColumns.AttachmentID+" AS "+models.DocumentAttachmentColumns.AttachmentID,
			"da."+models.DocumentAttachmentColumns.BlobID+" AS "+models.DocumentAttachmentColumns.BlobID,
			"da."+models.DocumentAttachmentColumns.DocumentID+" AS "+models.DocumentAttachmentColumns.DocumentID,
			"n."+models.NodeColumns.ReaderSlug+" AS "+models.NodeColumns.ReaderSlug,
			"da."+models.DocumentAttachmentColumns.SpaceID+" AS "+models.DocumentAttachmentColumns.SpaceID,
			"da."+models.DocumentAttachmentColumns.StorageProvider+" AS "+models.DocumentAttachmentColumns.StorageProvider,
			"da."+models.DocumentAttachmentColumns.FileName+" AS "+models.DocumentAttachmentColumns.FileName,
			"da."+models.DocumentAttachmentColumns.ObjectKey+" AS "+models.DocumentAttachmentColumns.ObjectKey,
			"da."+models.DocumentAttachmentColumns.ObjectURL+" AS "+models.DocumentAttachmentColumns.ObjectURL,
			"da."+models.DocumentAttachmentColumns.MimeType+" AS "+models.DocumentAttachmentColumns.MimeType,
			"da."+models.DocumentAttachmentColumns.SizeBytes+" AS "+models.DocumentAttachmentColumns.SizeBytes,
			"da."+models.DocumentAttachmentColumns.ContentHashAlgo+" AS "+models.DocumentAttachmentColumns.ContentHashAlgo,
			"da."+models.DocumentAttachmentColumns.ContentHash+" AS "+models.DocumentAttachmentColumns.ContentHash,
			"da."+models.DocumentAttachmentColumns.PreviewKind+" AS "+models.DocumentAttachmentColumns.PreviewKind,
			"da."+models.DocumentAttachmentColumns.Status+" AS "+models.DocumentAttachmentColumns.Status,
			"da."+models.DocumentAttachmentColumns.DeletedAt+" AS deleted_at_raw",
			"da."+models.DocumentAttachmentColumns.CreatedByUserID+" AS "+models.DocumentAttachmentColumns.CreatedByUserID,
			"da."+models.DocumentAttachmentColumns.CreatedAt+" AS created_at_raw",
			"da."+models.DocumentAttachmentColumns.UpdatedAt+" AS updated_at_raw",
			"d."+models.DocumentColumns.Title+" AS document_title",
			"d."+models.DocumentColumns.Status+" AS document_status",
			"s."+models.SpaceColumns.Name+" AS space_name",
			"s."+models.SpaceColumns.OwnerUserID+" AS space_owner_id",
			"uo."+models.UserColumns.Name+" AS space_owner_name",
			"uo."+models.UserColumns.Email+" AS space_owner_mail",
			"COALESCE(uc."+models.UserColumns.Name+",'') AS created_by_name",
			"COALESCE(uc."+models.UserColumns.Email+",'') AS created_by_email",
		).
		Order("da." + models.DocumentAttachmentColumns.CreatedAt + " DESC").
		Order("da." + models.DocumentAttachmentColumns.ID + " DESC").
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
			DeletedAt:       recordtime.ParseNullable(row.DeletedAtRaw),
			CreatedByUserID: row.CreatedByUserID,
			CreatedAt:       recordtime.Parse(row.CreatedAtRaw),
			UpdatedAt:       recordtime.Parse(row.UpdatedAtRaw),
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
		Model(&models.DocumentAttachmentBlob{}).
		Select(
			models.DocumentAttachmentBlobColumns.ID,
			models.DocumentAttachmentBlobColumns.BlobID,
			models.DocumentAttachmentBlobColumns.StorageProvider,
			models.DocumentAttachmentBlobColumns.ObjectKey,
			models.DocumentAttachmentBlobColumns.ObjectURL,
			models.DocumentAttachmentBlobColumns.MimeType,
			models.DocumentAttachmentBlobColumns.SizeBytes,
			models.DocumentAttachmentBlobColumns.ContentHashAlgo,
			models.DocumentAttachmentBlobColumns.ContentHash,
			models.DocumentAttachmentBlobColumns.DeletedAt+" AS DeletedAtRaw",
			models.DocumentAttachmentBlobColumns.CreatedAt+" AS CreatedAtRaw",
			models.DocumentAttachmentBlobColumns.UpdatedAt+" AS UpdatedAtRaw",
		).
		Where(
			models.DocumentAttachmentBlobColumns.StorageProvider+" = ? AND "+
				models.DocumentAttachmentBlobColumns.ContentHashAlgo+" = ? AND "+
				models.DocumentAttachmentBlobColumns.ContentHash+" = ? AND "+
				models.DocumentAttachmentBlobColumns.SizeBytes+" = ? AND "+
				models.DocumentAttachmentBlobColumns.DeletedAt+" IS NULL",
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
		Model(&models.DocumentAttachmentBlob{}).
		Select(
			models.DocumentAttachmentBlobColumns.ID,
			models.DocumentAttachmentBlobColumns.BlobID,
			models.DocumentAttachmentBlobColumns.StorageProvider,
			models.DocumentAttachmentBlobColumns.ObjectKey,
			models.DocumentAttachmentBlobColumns.ObjectURL,
			models.DocumentAttachmentBlobColumns.MimeType,
			models.DocumentAttachmentBlobColumns.SizeBytes,
			models.DocumentAttachmentBlobColumns.ContentHashAlgo,
			models.DocumentAttachmentBlobColumns.ContentHash,
			models.DocumentAttachmentBlobColumns.DeletedAt+" AS DeletedAtRaw",
			models.DocumentAttachmentBlobColumns.CreatedAt+" AS CreatedAtRaw",
			models.DocumentAttachmentBlobColumns.UpdatedAt+" AS UpdatedAtRaw",
		).
		Where(models.DocumentAttachmentBlobColumns.BlobID+" = ?", normalizedBlobID).
		Take(&row).Error; err != nil {
		return nil, err
	}

	blob := mapDocumentAttachmentBlobRow(row)
	return &blob, nil
}

func (r *gormDocumentAttachmentRepository) FindBlobByObject(
	ctx context.Context,
	storageProvider string,
	objectKey string,
) (*models.DocumentAttachmentBlob, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("document attachment repository db is nil")
	}

	normalizedStorageProvider := strings.ToLower(strings.TrimSpace(storageProvider))
	normalizedObjectKey := strings.TrimSpace(objectKey)
	if normalizedStorageProvider == "" || normalizedObjectKey == "" {
		return nil, gorm.ErrRecordNotFound
	}

	var row documentAttachmentBlobRow
	if err := r.db.WithContext(ctx).
		Model(&models.DocumentAttachmentBlob{}).
		Select(
			models.DocumentAttachmentBlobColumns.ID,
			models.DocumentAttachmentBlobColumns.BlobID,
			models.DocumentAttachmentBlobColumns.StorageProvider,
			models.DocumentAttachmentBlobColumns.ObjectKey,
			models.DocumentAttachmentBlobColumns.ObjectURL,
			models.DocumentAttachmentBlobColumns.MimeType,
			models.DocumentAttachmentBlobColumns.SizeBytes,
			models.DocumentAttachmentBlobColumns.ContentHashAlgo,
			models.DocumentAttachmentBlobColumns.ContentHash,
			models.DocumentAttachmentBlobColumns.DeletedAt+" AS DeletedAtRaw",
			models.DocumentAttachmentBlobColumns.CreatedAt+" AS CreatedAtRaw",
			models.DocumentAttachmentBlobColumns.UpdatedAt+" AS UpdatedAtRaw",
		).
		Where(
			models.DocumentAttachmentBlobColumns.StorageProvider+" = ? AND "+
				models.DocumentAttachmentBlobColumns.ObjectKey+" = ? AND "+
				models.DocumentAttachmentBlobColumns.DeletedAt+" IS NULL",
			normalizedStorageProvider,
			normalizedObjectKey,
		).
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
	imageAssetReferenceQuery := r.db.WithContext(ctx).
		Model(&models.DocumentImageAsset{}).
		Select("1").
		Where(tableName(models.DocumentImageAsset{})+"."+models.DocumentImageAssetColumns.BlobID+" = "+tableName(models.DocumentAttachmentBlob{})+"."+models.DocumentAttachmentBlobColumns.BlobID).
		Where(models.DocumentImageAssetColumns.Status+" = ?", documentImageAssetLifecycleStatusActive).
		Where(models.DocumentImageAssetColumns.DeletedAt + " IS NULL")
	attachmentReferenceQuery := r.db.WithContext(ctx).
		Model(&models.DocumentAttachment{}).
		Select("1").
		Where(qualifiedColumn("", models.DocumentAttachmentColumns.BlobID) + " = " + tableName(models.DocumentAttachmentBlob{}) + "." + models.DocumentAttachmentBlobColumns.BlobID)
	documentReferenceQuery := r.db.WithContext(ctx).
		Model(&models.Document{}).
		Select("1").
		Where(qualifiedColumn("", models.DocumentColumns.SourceBlobID) + " = " + tableName(models.DocumentAttachmentBlob{}) + "." + models.DocumentAttachmentBlobColumns.BlobID)
	fileRevisionReferenceQuery := r.db.WithContext(ctx).
		Model(&models.DocumentFileRevision{}).
		Select("1").
		Where(qualifiedColumn("", models.DocumentFileRevisionColumns.BlobID) + " = " + tableName(models.DocumentAttachmentBlob{}) + "." + models.DocumentAttachmentBlobColumns.BlobID)
	if err := r.db.WithContext(ctx).
		Model(&models.DocumentAttachmentBlob{}).
		Select(
			models.DocumentAttachmentBlobColumns.ID,
			models.DocumentAttachmentBlobColumns.BlobID,
			models.DocumentAttachmentBlobColumns.StorageProvider,
			models.DocumentAttachmentBlobColumns.ObjectKey,
			models.DocumentAttachmentBlobColumns.ObjectURL,
			models.DocumentAttachmentBlobColumns.MimeType,
			models.DocumentAttachmentBlobColumns.SizeBytes,
			models.DocumentAttachmentBlobColumns.ContentHashAlgo,
			models.DocumentAttachmentBlobColumns.ContentHash,
			models.DocumentAttachmentBlobColumns.DeletedAt+" AS DeletedAtRaw",
			models.DocumentAttachmentBlobColumns.CreatedAt+" AS CreatedAtRaw",
			models.DocumentAttachmentBlobColumns.UpdatedAt+" AS UpdatedAtRaw",
		).
		Where(models.DocumentAttachmentBlobColumns.DeletedAt+" IS NULL").
		Where("NOT EXISTS (?)", attachmentReferenceQuery).
		Where("NOT EXISTS (?)", documentReferenceQuery).
		Where("NOT EXISTS (?)", fileRevisionReferenceQuery).
		Where("NOT EXISTS (?)", imageAssetReferenceQuery).
		Order(models.DocumentAttachmentBlobColumns.CreatedAt + " ASC").
		Order(models.DocumentAttachmentBlobColumns.ID + " ASC").
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
		Where(models.DocumentAttachmentBlobColumns.BlobID+" = ?", normalizedBlobID).
		Where("NOT EXISTS (?)", r.db.WithContext(ctx).
			Model(&models.DocumentAttachment{}).
			Select("1").
			Where(qualifiedColumn("", models.DocumentAttachmentColumns.BlobID)+" = "+tableName(models.DocumentAttachmentBlob{})+"."+models.DocumentAttachmentBlobColumns.BlobID)).
		Where("NOT EXISTS (?)", r.db.WithContext(ctx).
			Model(&models.Document{}).
			Select("1").
			Where(qualifiedColumn("", models.DocumentColumns.SourceBlobID)+" = "+tableName(models.DocumentAttachmentBlob{})+"."+models.DocumentAttachmentBlobColumns.BlobID)).
		Where("NOT EXISTS (?)", r.db.WithContext(ctx).
			Model(&models.DocumentFileRevision{}).
			Select("1").
			Where(qualifiedColumn("", models.DocumentFileRevisionColumns.BlobID)+" = "+tableName(models.DocumentAttachmentBlob{})+"."+models.DocumentAttachmentBlobColumns.BlobID)).
		Where("NOT EXISTS (?)", r.db.WithContext(ctx).
			Model(&models.DocumentImageAsset{}).
			Select("1").
			Where(qualifiedColumn("", models.DocumentImageAssetColumns.BlobID)+" = "+tableName(models.DocumentAttachmentBlob{})+"."+models.DocumentAttachmentBlobColumns.BlobID).
			Where(qualifiedColumn("", models.DocumentImageAssetColumns.Status)+" = ?", documentImageAssetLifecycleStatusActive).
			Where(qualifiedColumn("", models.DocumentImageAssetColumns.DeletedAt)+" IS NULL")).
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

	countQuery := func(query *gorm.DB) (int64, error) {
		var total int64
		if err := query.Count(&total).Error; err != nil {
			return 0, err
		}
		return total, nil
	}

	attachmentCount, err := countQuery(
		r.db.WithContext(ctx).
			Model(&models.DocumentAttachment{}).
			Where(qualifiedColumn("", models.DocumentAttachmentColumns.BlobID)+" = ?", normalizedBlobID),
	)
	if err != nil {
		return 0, err
	}

	documentCount, err := countQuery(
		r.db.WithContext(ctx).
			Model(&models.Document{}).
			Where(qualifiedColumn("", models.DocumentColumns.SourceBlobID)+" = ?", normalizedBlobID),
	)
	if err != nil {
		return 0, err
	}

	revisionCount, err := countQuery(
		r.db.WithContext(ctx).
			Model(&models.DocumentFileRevision{}).
			Where(qualifiedColumn("", models.DocumentFileRevisionColumns.BlobID)+" = ?", normalizedBlobID),
	)
	if err != nil {
		return 0, err
	}

	imageAssetCount, err := countQuery(
		r.db.WithContext(ctx).
			Model(&models.DocumentImageAsset{}).
			Where(qualifiedColumn("", models.DocumentImageAssetColumns.BlobID)+" = ?", normalizedBlobID).
			Where(qualifiedColumn("", models.DocumentImageAssetColumns.Status)+" = ?", documentImageAssetLifecycleStatusActive).
			Where(qualifiedColumn("", models.DocumentImageAssetColumns.DeletedAt) + " IS NULL"),
	)
	if err != nil {
		return 0, err
	}

	return attachmentCount + documentCount + revisionCount + imageAssetCount, nil
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

	rows := make([]documentAttachmentReferenceRow, 0, limit)
	if err := r.db.WithContext(ctx).
		Table(tableWithAlias(models.DocumentAttachment{}, "da")).
		Select(
			"da."+models.DocumentAttachmentColumns.AttachmentID+" AS AttachmentID",
			"da."+models.DocumentAttachmentColumns.DocumentID+" AS DocumentID",
			"d."+models.DocumentColumns.Title+" AS DocumentTitle",
			"da."+models.DocumentAttachmentColumns.SpaceID+" AS SpaceID",
			"s."+models.SpaceColumns.Name+" AS SpaceName",
			"da."+models.DocumentAttachmentColumns.FileName+" AS FileName",
			"da."+models.DocumentAttachmentColumns.Status+" AS Status",
		).
		Joins("JOIN "+tableName(models.Document{})+" AS d ON d."+models.DocumentColumns.DocumentID+" = da."+models.DocumentAttachmentColumns.DocumentID).
		Joins("JOIN "+tableName(models.Space{})+" AS s ON s."+models.SpaceColumns.SpaceID+" = da."+models.DocumentAttachmentColumns.SpaceID).
		Where("da."+models.DocumentAttachmentColumns.BlobID+" = ?", normalizedBlobID).
		Where("da."+models.DocumentAttachmentColumns.Status+" = ?", models.EntityStatusActive).
		Where("da." + models.DocumentAttachmentColumns.DeletedAt + " IS NULL").
		Order("da." + models.DocumentAttachmentColumns.CreatedAt + " ASC").
		Order("da." + models.DocumentAttachmentColumns.ID + " ASC").
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
		Model(&models.DocumentAttachment{}).
		Select(
			models.DocumentAttachmentColumns.ID,
			models.DocumentAttachmentColumns.AttachmentID,
			models.DocumentAttachmentColumns.BlobID,
			models.DocumentAttachmentColumns.DocumentID,
			models.DocumentAttachmentColumns.SpaceID,
			models.DocumentAttachmentColumns.StorageProvider,
			models.DocumentAttachmentColumns.FileName,
			models.DocumentAttachmentColumns.ObjectKey,
			models.DocumentAttachmentColumns.ObjectURL,
			models.DocumentAttachmentColumns.MimeType,
			models.DocumentAttachmentColumns.SizeBytes,
			models.DocumentAttachmentColumns.ContentHashAlgo,
			models.DocumentAttachmentColumns.ContentHash,
			models.DocumentAttachmentColumns.PreviewKind,
			models.DocumentAttachmentColumns.Status,
			models.DocumentAttachmentColumns.DeletedAt+" AS DeletedAtRaw",
			models.DocumentAttachmentColumns.CreatedByUserID,
			models.DocumentAttachmentColumns.CreatedAt+" AS CreatedAtRaw",
			models.DocumentAttachmentColumns.UpdatedAt+" AS UpdatedAtRaw",
		).
		Where(models.DocumentAttachmentColumns.AttachmentID+" = ?", normalizedAttachmentID).
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
		Where(models.DocumentAttachmentColumns.AttachmentID+" = ?", normalizedAttachmentID).
		Where(models.DocumentAttachmentColumns.Status+" <> ?", models.EntityStatusDeleted).
		Updates(map[string]any{
			models.DocumentAttachmentColumns.Status:    models.EntityStatusDeleted,
			models.DocumentAttachmentColumns.DeletedAt: deletedAt,
			models.DocumentAttachmentColumns.UpdatedAt: deletedAt,
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
		Where(models.DocumentAttachmentColumns.AttachmentID+" = ?", normalizedAttachmentID).
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
		DeletedAt:       recordtime.ParseNullable(row.DeletedAtRaw),
		CreatedByUserID: row.CreatedByUserID,
		CreatedAt:       recordtime.Parse(row.CreatedAtRaw),
		UpdatedAt:       recordtime.Parse(row.UpdatedAtRaw),
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
		DeletedAt:       recordtime.ParseNullable(row.DeletedAtRaw),
		CreatedAt:       recordtime.Parse(row.CreatedAtRaw),
		UpdatedAt:       recordtime.Parse(row.UpdatedAtRaw),
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
