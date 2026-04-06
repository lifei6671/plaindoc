package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"
)

const (
	documentImageAssetLifecycleStatusActive         = "active"
	documentImageAssetLifecycleStatusPendingCleanup = "pending_cleanup"
	documentImageAssetLifecycleStatusDeleted        = "deleted"
)

type gormDocumentImageAssetLifecycleRepository struct {
	db *gorm.DB
}

type documentImageAssetLifecycleExistingRow = documentImageAssetLifecycleExistingRowDB

type documentImageAssetLifecyclePendingCandidateRow = documentImageAssetLifecyclePendingCandidateRowDB

type documentImageAssetLifecycleBlobRow = documentImageAssetLifecycleBlobRowDB

// NewGormDocumentImageAssetLifecycleRepository 创建文档图片生命周期仓储实现。
func NewGormDocumentImageAssetLifecycleRepository(db *gorm.DB) DocumentImageAssetLifecycleRepository {
	return &gormDocumentImageAssetLifecycleRepository{db: db}
}

func (r *gormDocumentImageAssetLifecycleRepository) SyncDocumentReferences(
	ctx context.Context,
	params SyncDocumentImageAssetReferencesParams,
) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("document image asset lifecycle repository db is nil")
	}

	documentID := strings.TrimSpace(params.DocumentID)
	spaceID := strings.TrimSpace(params.SpaceID)
	if documentID == "" || spaceID == "" {
		return fmt.Errorf("document image asset sync params are invalid")
	}

	referencedAt := params.ReferencedAt.UTC()
	if referencedAt.IsZero() {
		referencedAt = time.Now().UTC()
	}

	referencesWithBlobIDs, err := r.resolveBlobIDsByObject(ctx, params.References)
	if err != nil {
		return err
	}

	normalizedRefs := normalizeDocumentImageAssetReferenceInputs(referencesWithBlobIDs)
	existingAssets := make([]documentImageAssetLifecycleExistingRow, 0, len(normalizedRefs)+8)
	if err := r.db.WithContext(ctx).
		Model(&models.DocumentImageAsset{}).
		Select(
			models.DocumentImageAssetColumns.ID,
			models.DocumentImageAssetColumns.BlobID,
			models.DocumentImageAssetColumns.StorageProvider,
			models.DocumentImageAssetColumns.ObjectKey,
		).
		Where(models.DocumentImageAssetColumns.DocumentID+" = ?", documentID).
		Where(models.DocumentImageAssetColumns.Status+" IN ?", []string{
			documentImageAssetLifecycleStatusActive,
			documentImageAssetLifecycleStatusPendingCleanup,
		}).
		Find(&existingAssets).Error; err != nil {
		return err
	}

	existingByKey := make(map[string]documentImageAssetLifecycleExistingRow, len(existingAssets))
	for _, item := range existingAssets {
		identityKey := buildDocumentImageAssetLifecycleIdentityKey(item.StorageProvider, item.ObjectKey)
		if identityKey == "" {
			continue
		}
		existingByKey[identityKey] = item
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, reference := range normalizedRefs {
			identityKey := buildDocumentImageAssetLifecycleIdentityKey(reference.StorageProvider, reference.ObjectKey)
			if identityKey == "" {
				continue
			}
			if existing, exists := existingByKey[identityKey]; exists {
				if err := tx.Model(&models.DocumentImageAsset{}).
					Where(models.DocumentImageAssetColumns.ID+" = ?", existing.ID).
					Updates(map[string]any{
						models.DocumentImageAssetColumns.BlobID:           nullableTrimmedString(reference.BlobID),
						models.DocumentImageAssetColumns.SpaceID:          spaceID,
						models.DocumentImageAssetColumns.ObjectURL:        reference.ObjectURL,
						models.DocumentImageAssetColumns.Status:           documentImageAssetLifecycleStatusActive,
						models.DocumentImageAssetColumns.PendingCleanupAt: nil,
						models.DocumentImageAssetColumns.DeletedAt:        nil,
						models.DocumentImageAssetColumns.LastReferencedAt: referencedAt,
						models.DocumentImageAssetColumns.UpdatedAt:        referencedAt,
					}).Error; err != nil {
					return err
				}
				delete(existingByKey, identityKey)
				continue
			}

			newAsset := &models.DocumentImageAsset{
				ImageAssetID:     strings.ToLower(ulid.Make().String()),
				DocumentID:       documentID,
				SpaceID:          spaceID,
				BlobID:           nullableTrimmedString(reference.BlobID),
				StorageProvider:  reference.StorageProvider,
				ObjectKey:        reference.ObjectKey,
				ObjectURL:        reference.ObjectURL,
				Status:           documentImageAssetLifecycleStatusActive,
				LastReferencedAt: referencedAt,
				CreatedAt:        referencedAt,
				UpdatedAt:        referencedAt,
				PendingCleanupAt: nil,
				DeletedAt:        nil,
			}
			if err := tx.Create(newAsset).Error; err != nil {
				return err
			}
		}

		staleAssetIDs := make([]int64, 0, len(existingByKey))
		for _, item := range existingByKey {
			staleAssetIDs = append(staleAssetIDs, item.ID)
		}
		if len(staleAssetIDs) == 0 {
			return nil
		}

		return tx.Model(&models.DocumentImageAsset{}).
			Where(models.DocumentImageAssetColumns.ID+" IN ?", staleAssetIDs).
			Updates(map[string]any{
				models.DocumentImageAssetColumns.Status:           documentImageAssetLifecycleStatusPendingCleanup,
				models.DocumentImageAssetColumns.PendingCleanupAt: gorm.Expr("COALESCE("+models.DocumentImageAssetColumns.PendingCleanupAt+", ?)", referencedAt),
				models.DocumentImageAssetColumns.UpdatedAt:        referencedAt,
			}).Error
	})
}

func (r *gormDocumentImageAssetLifecycleRepository) ListPendingCleanupCandidates(
	ctx context.Context,
	cutoff time.Time,
	limit int,
) ([]DocumentImageAssetCleanupCandidate, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("document image asset lifecycle repository db is nil")
	}
	if limit <= 0 {
		return []DocumentImageAssetCleanupCandidate{}, nil
	}

	rows := make([]documentImageAssetLifecyclePendingCandidateRow, 0, limit)
	if err := r.db.WithContext(ctx).
		Model(&models.DocumentImageAsset{}).
		Select(
			models.DocumentImageAssetColumns.ID,
			models.DocumentImageAssetColumns.BlobID,
			models.DocumentImageAssetColumns.StorageProvider,
			models.DocumentImageAssetColumns.ObjectKey,
		).
		Where(models.DocumentImageAssetColumns.Status+" = ?", documentImageAssetLifecycleStatusPendingCleanup).
		Where(models.DocumentImageAssetColumns.PendingCleanupAt+" IS NOT NULL").
		Where(models.DocumentImageAssetColumns.PendingCleanupAt+" <= ?", cutoff).
		Order(models.DocumentImageAssetColumns.PendingCleanupAt + " ASC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]DocumentImageAssetCleanupCandidate, 0, len(rows))
	for _, row := range rows {
		result = append(result, DocumentImageAssetCleanupCandidate{
			ID:              row.ID,
			StorageProvider: strings.TrimSpace(row.StorageProvider),
			ObjectKey:       strings.TrimSpace(row.ObjectKey),
		})
	}
	return result, nil
}

func (r *gormDocumentImageAssetLifecycleRepository) MarkDeletedDocumentReferencesPending(
	ctx context.Context,
	now time.Time,
) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("document image asset lifecycle repository db is nil")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	activeDocumentSubquery := r.db.WithContext(ctx).
		Model(&models.Document{}).
		Select(models.DocumentColumns.DocumentID).
		Where(models.DocumentColumns.Status+" = ?", models.EntityStatusActive).
		Where(models.DocumentColumns.DeletedAt + " IS NULL")

	return r.db.WithContext(ctx).
		Model(&models.DocumentImageAsset{}).
		Where(models.DocumentImageAssetColumns.Status+" = ?", documentImageAssetLifecycleStatusActive).
		Where(models.DocumentImageAssetColumns.DocumentID+" NOT IN (?)", activeDocumentSubquery).
		Updates(map[string]any{
			models.DocumentImageAssetColumns.Status:           documentImageAssetLifecycleStatusPendingCleanup,
			models.DocumentImageAssetColumns.PendingCleanupAt: gorm.Expr("COALESCE("+models.DocumentImageAssetColumns.PendingCleanupAt+", ?)", now),
			models.DocumentImageAssetColumns.UpdatedAt:        now,
		}).Error
}

func (r *gormDocumentImageAssetLifecycleRepository) CountActiveReferencesByObject(
	ctx context.Context,
	storageProvider string,
	objectKey string,
) (int64, error) {
	if r == nil || r.db == nil {
		return 0, fmt.Errorf("document image asset lifecycle repository db is nil")
	}

	normalizedStorageProvider := strings.ToLower(strings.TrimSpace(storageProvider))
	normalizedObjectKey := strings.TrimSpace(objectKey)
	if normalizedStorageProvider == "" || normalizedObjectKey == "" {
		return 0, nil
	}

	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.DocumentImageAsset{}).
		Where(models.DocumentImageAssetColumns.StorageProvider+" = ?", normalizedStorageProvider).
		Where(models.DocumentImageAssetColumns.ObjectKey+" = ?", normalizedObjectKey).
		Where(models.DocumentImageAssetColumns.Status+" = ?", documentImageAssetLifecycleStatusActive).
		Count(&count).Error
	return count, err
}

func (r *gormDocumentImageAssetLifecycleRepository) MarkDeletedByID(
	ctx context.Context,
	id int64,
	now time.Time,
) (int64, error) {
	if r == nil || r.db == nil {
		return 0, fmt.Errorf("document image asset lifecycle repository db is nil")
	}
	if id <= 0 {
		return 0, nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	updateTx := r.db.WithContext(ctx).
		Model(&models.DocumentImageAsset{}).
		Where(models.DocumentImageAssetColumns.ID+" = ?", id).
		Where(models.DocumentImageAssetColumns.Status+" <> ?", documentImageAssetLifecycleStatusDeleted).
		Updates(map[string]any{
			models.DocumentImageAssetColumns.Status:    documentImageAssetLifecycleStatusDeleted,
			models.DocumentImageAssetColumns.DeletedAt: now,
			models.DocumentImageAssetColumns.UpdatedAt: now,
		})
	return updateTx.RowsAffected, updateTx.Error
}

func (r *gormDocumentImageAssetLifecycleRepository) MarkDeletedByObject(
	ctx context.Context,
	storageProvider string,
	objectKey string,
	now time.Time,
) (int64, error) {
	if r == nil || r.db == nil {
		return 0, fmt.Errorf("document image asset lifecycle repository db is nil")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	updateTx := r.db.WithContext(ctx).
		Model(&models.DocumentImageAsset{}).
		Where(models.DocumentImageAssetColumns.StorageProvider+" = ?", strings.ToLower(strings.TrimSpace(storageProvider))).
		Where(models.DocumentImageAssetColumns.ObjectKey+" = ?", strings.TrimSpace(objectKey)).
		Where(models.DocumentImageAssetColumns.Status+" <> ?", documentImageAssetLifecycleStatusDeleted).
		Updates(map[string]any{
			models.DocumentImageAssetColumns.Status:    documentImageAssetLifecycleStatusDeleted,
			models.DocumentImageAssetColumns.DeletedAt: now,
			models.DocumentImageAssetColumns.UpdatedAt: now,
		})
	return updateTx.RowsAffected, updateTx.Error
}

func buildDocumentImageAssetLifecycleIdentityKey(storageProvider string, objectKey string) string {
	normalizedStorageProvider := strings.ToLower(strings.TrimSpace(storageProvider))
	normalizedObjectKey := strings.TrimSpace(objectKey)
	if normalizedStorageProvider == "" || normalizedObjectKey == "" {
		return ""
	}
	return normalizedStorageProvider + "::" + normalizedObjectKey
}

func normalizeDocumentImageAssetReferenceInputs(
	references []DocumentImageAssetReferenceInput,
) []DocumentImageAssetReferenceInput {
	if len(references) == 0 {
		return []DocumentImageAssetReferenceInput{}
	}
	result := make([]DocumentImageAssetReferenceInput, 0, len(references))
	seen := make(map[string]struct{}, len(references))
	for _, item := range references {
		normalized := DocumentImageAssetReferenceInput{
			BlobID:          strings.TrimSpace(item.BlobID),
			StorageProvider: strings.ToLower(strings.TrimSpace(item.StorageProvider)),
			ObjectKey:       strings.TrimSpace(item.ObjectKey),
			ObjectURL:       strings.TrimSpace(item.ObjectURL),
		}
		identityKey := buildDocumentImageAssetLifecycleIdentityKey(
			normalized.StorageProvider,
			normalized.ObjectKey,
		)
		if identityKey == "" {
			continue
		}
		if _, exists := seen[identityKey]; exists {
			continue
		}
		seen[identityKey] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func (r *gormDocumentImageAssetLifecycleRepository) resolveBlobIDsByObject(
	ctx context.Context,
	references []DocumentImageAssetReferenceInput,
) ([]DocumentImageAssetReferenceInput, error) {
	if len(references) == 0 {
		return []DocumentImageAssetReferenceInput{}, nil
	}

	groupedObjectKeys := make(map[string][]string)
	for _, item := range references {
		storageProvider := strings.ToLower(strings.TrimSpace(item.StorageProvider))
		objectKey := strings.TrimSpace(item.ObjectKey)
		if storageProvider == "" || objectKey == "" {
			continue
		}
		groupedObjectKeys[storageProvider] = append(groupedObjectKeys[storageProvider], objectKey)
	}

	blobIDByIdentity := make(map[string]string, len(references))
	for storageProvider, objectKeys := range groupedObjectKeys {
		if len(objectKeys) == 0 {
			continue
		}
		rows := make([]documentImageAssetLifecycleBlobRow, 0, len(objectKeys))
		if err := r.db.WithContext(ctx).
			Model(&models.DocumentAttachmentBlob{}).
			Select(
				models.DocumentAttachmentBlobColumns.BlobID,
				models.DocumentAttachmentBlobColumns.StorageProvider,
				models.DocumentAttachmentBlobColumns.ObjectKey,
			).
			Where(models.DocumentAttachmentBlobColumns.DeletedAt+" IS NULL").
			Where(models.DocumentAttachmentBlobColumns.StorageProvider+" = ?", storageProvider).
			Where(models.DocumentAttachmentBlobColumns.ObjectKey+" IN ?", objectKeys).
			Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			identityKey := buildDocumentImageAssetLifecycleIdentityKey(row.StorageProvider, row.ObjectKey)
			if identityKey == "" {
				continue
			}
			blobIDByIdentity[identityKey] = strings.TrimSpace(row.BlobID)
		}
	}

	result := make([]DocumentImageAssetReferenceInput, 0, len(references))
	for _, item := range references {
		normalized := DocumentImageAssetReferenceInput{
			StorageProvider: strings.ToLower(strings.TrimSpace(item.StorageProvider)),
			ObjectKey:       strings.TrimSpace(item.ObjectKey),
			ObjectURL:       strings.TrimSpace(item.ObjectURL),
		}
		identityKey := buildDocumentImageAssetLifecycleIdentityKey(
			normalized.StorageProvider,
			normalized.ObjectKey,
		)
		if blobID, ok := blobIDByIdentity[identityKey]; ok {
			normalized.BlobID = blobID
		}
		result = append(result, normalized)
	}
	return result, nil
}

func nullableTrimmedString(value string) *string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return nil
	}
	return &normalized
}
