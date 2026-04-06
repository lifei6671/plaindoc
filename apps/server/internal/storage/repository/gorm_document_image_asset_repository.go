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

type gormDocumentImageAssetRepository struct {
	db *gorm.DB
}

type documentImageAssetRow = documentImageAssetRowDB

type adminDocumentImageAssetListRow = adminDocumentImageAssetListRowDB

type documentImageAssetReferenceRow = documentImageAssetReferenceRowDB

// NewGormDocumentImageAssetRepository 创建基于 GORM 的文档图片资源仓储实现。
func NewGormDocumentImageAssetRepository(db *gorm.DB) DocumentImageAssetRepository {
	return &gormDocumentImageAssetRepository{db: db}
}

func (r *gormDocumentImageAssetRepository) ListForAdmin(
	ctx context.Context,
	params ListAdminDocumentImageAssetsParams,
) ([]AdminDocumentImageAssetListRecord, int64, error) {
	if r == nil || r.db == nil {
		return nil, 0, fmt.Errorf("document image asset repository db is nil")
	}

	baseQuery := r.db.WithContext(ctx).
		Table(tableWithAlias(models.DocumentImageAsset{}, "dia")).
		Joins("JOIN " + tableName(models.Document{}) + " AS d ON d." + models.DocumentColumns.DocumentID + " = dia." + models.DocumentImageAssetColumns.DocumentID).
		Joins("JOIN " + tableName(models.Node{}) + " AS n ON n." + models.NodeColumns.NodeID + " = d." + models.DocumentColumns.NodeID).
		Joins("JOIN " + tableName(models.Space{}) + " AS s ON s." + models.SpaceColumns.SpaceID + " = n." + models.NodeColumns.SpaceID).
		Joins("JOIN " + tableName(models.User{}) + " AS uo ON uo." + models.UserColumns.UserID + " = s." + models.SpaceColumns.OwnerUserID)

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
			"LOWER("+qualifiedColumn("dia", models.DocumentImageAssetColumns.ImageAssetID)+") LIKE ? OR LOWER("+qualifiedColumn("dia", models.DocumentImageAssetColumns.DocumentID)+") LIKE ? OR LOWER("+qualifiedColumn("dia", models.DocumentImageAssetColumns.ObjectKey)+") LIKE ? OR LOWER("+qualifiedColumn("dia", models.DocumentImageAssetColumns.ObjectURL)+") LIKE ? OR LOWER(COALESCE("+qualifiedColumn("dia", models.DocumentImageAssetColumns.Status)+",'')) LIKE ? OR LOWER("+qualifiedColumn("d", models.DocumentColumns.Title)+") LIKE ? OR LOWER("+qualifiedColumn("s", models.SpaceColumns.SpaceID)+") LIKE ? OR LOWER("+qualifiedColumn("s", models.SpaceColumns.Name)+") LIKE ? OR LOWER("+qualifiedColumn("uo", models.UserColumns.UserID)+") LIKE ? OR LOWER("+qualifiedColumn("uo", models.UserColumns.Email)+") LIKE ? OR LOWER("+qualifiedColumn("uo", models.UserColumns.Name)+") LIKE ?",
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
		baseQuery = baseQuery.Where("s."+models.SpaceColumns.SpaceID+" = ?", spaceID)
	}

	documentID := strings.TrimSpace(params.DocumentID)
	if documentID != "" {
		baseQuery = baseQuery.Where("dia."+models.DocumentImageAssetColumns.DocumentID+" = ?", documentID)
	}

	statuses := normalizeImageAssetStatuses(params.Statuses)
	if len(statuses) > 0 {
		baseQuery = baseQuery.Where("LOWER(dia."+models.DocumentImageAssetColumns.Status+") IN ?", statuses)
	}

	storageProviders := normalizeImageAssetStorageProviders(params.StorageProviders)
	if len(storageProviders) > 0 {
		baseQuery = baseQuery.Where("LOWER(dia."+models.DocumentImageAssetColumns.StorageProvider+") IN ?", storageProviders)
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

	rows := make([]adminDocumentImageAssetListRow, 0, limit)
	if err := baseQuery.Session(&gorm.Session{}).
		Select(
			"dia."+models.DocumentImageAssetColumns.ID+" AS id",
			"dia."+models.DocumentImageAssetColumns.ImageAssetID+" AS image_asset_id",
			"dia."+models.DocumentImageAssetColumns.DocumentID+" AS document_id",
			"n."+models.NodeColumns.ReaderSlug+" AS reader_slug",
			"dia."+models.DocumentImageAssetColumns.SpaceID+" AS space_id",
			"dia."+models.DocumentImageAssetColumns.BlobID+" AS blob_id",
			"dia."+models.DocumentImageAssetColumns.StorageProvider+" AS storage_provider",
			"dia."+models.DocumentImageAssetColumns.ObjectKey+" AS object_key",
			"dia."+models.DocumentImageAssetColumns.ObjectURL+" AS object_url",
			"dia."+models.DocumentImageAssetColumns.Status+" AS status",
			"dia."+models.DocumentImageAssetColumns.PendingCleanupAt+" AS pending_cleanup_at_raw",
			"dia."+models.DocumentImageAssetColumns.DeletedAt+" AS deleted_at_raw",
			"dia."+models.DocumentImageAssetColumns.LastReferencedAt+" AS last_referenced_at_raw",
			"dia."+models.DocumentImageAssetColumns.CreatedAt+" AS created_at_raw",
			"dia."+models.DocumentImageAssetColumns.UpdatedAt+" AS updated_at_raw",
			"d."+models.DocumentColumns.Title+" AS document_title",
			"d."+models.DocumentColumns.Status+" AS document_status",
			"s."+models.SpaceColumns.Name+" AS space_name",
			"s."+models.SpaceColumns.OwnerUserID+" AS space_owner_id",
			"uo."+models.UserColumns.Name+" AS space_owner_name",
			"uo."+models.UserColumns.Email+" AS space_owner_mail",
		).
		Order("dia." + models.DocumentImageAssetColumns.CreatedAt + " DESC").
		Order("dia." + models.DocumentImageAssetColumns.ID + " DESC").
		Offset(offset).
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	result := make([]AdminDocumentImageAssetListRecord, 0, len(rows))
	for _, row := range rows {
		status := strings.ToLower(strings.TrimSpace(row.Status))
		if status == "" {
			status = "active"
		}
		documentStatus := row.DocumentStatus
		if !models.IsValidEntityStatus(documentStatus) {
			documentStatus = models.EntityStatusActive
		}
		result = append(result, AdminDocumentImageAssetListRecord{
			ImageAsset: models.DocumentImageAsset{
				ID:               row.ID,
				ImageAssetID:     strings.TrimSpace(row.ImageAssetID),
				DocumentID:       strings.TrimSpace(row.DocumentID),
				SpaceID:          strings.TrimSpace(row.SpaceID),
				BlobID:           trimOptionalString(row.BlobID),
				StorageProvider:  strings.TrimSpace(row.StorageProvider),
				ObjectKey:        strings.TrimSpace(row.ObjectKey),
				ObjectURL:        strings.TrimSpace(row.ObjectURL),
				Status:           status,
				PendingCleanupAt: recordtime.ParseNullable(row.PendingCleanupAtRaw),
				DeletedAt:        recordtime.ParseNullable(row.DeletedAtRaw),
				LastReferencedAt: recordtime.Parse(row.LastReferencedAtRaw),
				CreatedAt:        recordtime.Parse(row.CreatedAtRaw),
				UpdatedAt:        recordtime.Parse(row.UpdatedAtRaw),
			},
			DocumentRouteKey: resolveAdminDocumentRouteKey(row.DocumentID, row.ReaderSlug),
			DocumentTitle:    strings.TrimSpace(row.DocumentTitle),
			DocumentStatus:   documentStatus,
			SpaceName:        strings.TrimSpace(row.SpaceName),
			SpaceOwnerID:     strings.TrimSpace(row.SpaceOwnerID),
			SpaceOwnerName:   strings.TrimSpace(row.SpaceOwnerName),
			SpaceOwnerEmail:  strings.TrimSpace(row.SpaceOwnerMail),
		})
	}

	return result, total, nil
}

func (r *gormDocumentImageAssetRepository) GetByImageAssetID(
	ctx context.Context,
	imageAssetID string,
) (*models.DocumentImageAsset, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("document image asset repository db is nil")
	}

	normalizedImageAssetID := strings.TrimSpace(imageAssetID)
	if normalizedImageAssetID == "" {
		return nil, gorm.ErrRecordNotFound
	}

	var row documentImageAssetRow
	if err := r.db.WithContext(ctx).
		Model(&models.DocumentImageAsset{}).
		Select(
			models.DocumentImageAssetColumns.ID,
			models.DocumentImageAssetColumns.ImageAssetID,
			models.DocumentImageAssetColumns.DocumentID,
			models.DocumentImageAssetColumns.SpaceID,
			models.DocumentImageAssetColumns.BlobID,
			models.DocumentImageAssetColumns.StorageProvider,
			models.DocumentImageAssetColumns.ObjectKey,
			models.DocumentImageAssetColumns.ObjectURL,
			models.DocumentImageAssetColumns.Status,
			models.DocumentImageAssetColumns.PendingCleanupAt+" AS PendingCleanupAtRaw",
			models.DocumentImageAssetColumns.DeletedAt+" AS DeletedAtRaw",
			models.DocumentImageAssetColumns.LastReferencedAt+" AS LastReferencedAtRaw",
			models.DocumentImageAssetColumns.CreatedAt+" AS CreatedAtRaw",
			models.DocumentImageAssetColumns.UpdatedAt+" AS UpdatedAtRaw",
		).
		Where(models.DocumentImageAssetColumns.ImageAssetID+" = ?", normalizedImageAssetID).
		Take(&row).Error; err != nil {
		return nil, err
	}

	status := strings.ToLower(strings.TrimSpace(row.Status))
	if status == "" {
		status = "active"
	}
	asset := &models.DocumentImageAsset{
		ID:               row.ID,
		ImageAssetID:     strings.TrimSpace(row.ImageAssetID),
		DocumentID:       strings.TrimSpace(row.DocumentID),
		SpaceID:          strings.TrimSpace(row.SpaceID),
		BlobID:           trimOptionalString(row.BlobID),
		StorageProvider:  strings.TrimSpace(row.StorageProvider),
		ObjectKey:        strings.TrimSpace(row.ObjectKey),
		ObjectURL:        strings.TrimSpace(row.ObjectURL),
		Status:           status,
		PendingCleanupAt: recordtime.ParseNullable(row.PendingCleanupAtRaw),
		DeletedAt:        recordtime.ParseNullable(row.DeletedAtRaw),
		LastReferencedAt: recordtime.Parse(row.LastReferencedAtRaw),
		CreatedAt:        recordtime.Parse(row.CreatedAtRaw),
		UpdatedAt:        recordtime.Parse(row.UpdatedAtRaw),
	}
	return asset, nil
}

func (r *gormDocumentImageAssetRepository) SoftDelete(
	ctx context.Context,
	imageAssetID string,
	deletedAt time.Time,
) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("document image asset repository db is nil")
	}

	normalizedImageAssetID := strings.TrimSpace(imageAssetID)
	if normalizedImageAssetID == "" {
		return false, nil
	}

	tx := r.db.WithContext(ctx).
		Model(&models.DocumentImageAsset{}).
		Where(models.DocumentImageAssetColumns.ImageAssetID+" = ?", normalizedImageAssetID).
		Where("LOWER("+models.DocumentImageAssetColumns.Status+") <> ?", "deleted").
		Updates(map[string]any{
			models.DocumentImageAssetColumns.Status:           "deleted",
			models.DocumentImageAssetColumns.DeletedAt:        deletedAt,
			models.DocumentImageAssetColumns.UpdatedAt:        deletedAt,
			models.DocumentImageAssetColumns.PendingCleanupAt: nil,
		})
	if tx.Error != nil {
		return false, tx.Error
	}
	return tx.RowsAffected > 0, nil
}

func (r *gormDocumentImageAssetRepository) HardDelete(
	ctx context.Context,
	imageAssetID string,
) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("document image asset repository db is nil")
	}

	normalizedImageAssetID := strings.TrimSpace(imageAssetID)
	if normalizedImageAssetID == "" {
		return false, nil
	}

	tx := r.db.WithContext(ctx).
		Where(models.DocumentImageAssetColumns.ImageAssetID+" = ?", normalizedImageAssetID).
		Delete(&models.DocumentImageAsset{})
	if tx.Error != nil {
		return false, tx.Error
	}
	return tx.RowsAffected > 0, nil
}

func (r *gormDocumentImageAssetRepository) CountActiveReferencesByObject(
	ctx context.Context,
	storageProvider string,
	objectKey string,
) (int64, error) {
	if r == nil || r.db == nil {
		return 0, fmt.Errorf("document image asset repository db is nil")
	}

	normalizedProvider := strings.ToLower(strings.TrimSpace(storageProvider))
	normalizedObjectKey := strings.TrimSpace(objectKey)
	if normalizedProvider == "" || normalizedObjectKey == "" {
		return 0, nil
	}

	var total int64
	if err := r.db.WithContext(ctx).
		Model(&models.DocumentImageAsset{}).
		Where("LOWER("+models.DocumentImageAssetColumns.StorageProvider+") = ?", normalizedProvider).
		Where(models.DocumentImageAssetColumns.ObjectKey+" = ?", normalizedObjectKey).
		Where("LOWER("+models.DocumentImageAssetColumns.Status+") = ?", "active").
		Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (r *gormDocumentImageAssetRepository) ListActiveReferencesByObject(
	ctx context.Context,
	storageProvider string,
	objectKey string,
	limit int,
) ([]DocumentImageAssetReferenceRecord, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("document image asset repository db is nil")
	}

	normalizedProvider := strings.ToLower(strings.TrimSpace(storageProvider))
	normalizedObjectKey := strings.TrimSpace(objectKey)
	if normalizedProvider == "" || normalizedObjectKey == "" {
		return []DocumentImageAssetReferenceRecord{}, nil
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	rows := make([]documentImageAssetReferenceRow, 0, limit)
	if err := r.db.WithContext(ctx).
		Table(tableWithAlias(models.DocumentImageAsset{}, "dia")).
		Select(
			"dia."+models.DocumentImageAssetColumns.ImageAssetID+" AS ImageAssetID",
			"dia."+models.DocumentImageAssetColumns.DocumentID+" AS DocumentID",
			"d."+models.DocumentColumns.Title+" AS DocumentTitle",
			"dia."+models.DocumentImageAssetColumns.SpaceID+" AS SpaceID",
			"s."+models.SpaceColumns.Name+" AS SpaceName",
			"dia."+models.DocumentImageAssetColumns.Status+" AS Status",
			"dia."+models.DocumentImageAssetColumns.LastReferencedAt+" AS LastReferencedAtRaw",
		).
		Joins("JOIN "+tableName(models.Document{})+" AS d ON d."+models.DocumentColumns.DocumentID+" = dia."+models.DocumentImageAssetColumns.DocumentID).
		Joins("JOIN "+tableName(models.Space{})+" AS s ON s."+models.SpaceColumns.SpaceID+" = dia."+models.DocumentImageAssetColumns.SpaceID).
		Where(
			"LOWER(dia."+models.DocumentImageAssetColumns.StorageProvider+") = ? AND dia."+models.DocumentImageAssetColumns.ObjectKey+" = ? AND LOWER(dia."+models.DocumentImageAssetColumns.Status+") = ?",
			normalizedProvider,
			normalizedObjectKey,
			"active",
		).
		Order("dia." + models.DocumentImageAssetColumns.LastReferencedAt + " DESC").
		Order("dia." + models.DocumentImageAssetColumns.ID + " DESC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]DocumentImageAssetReferenceRecord, 0, len(rows))
	for _, row := range rows {
		result = append(result, DocumentImageAssetReferenceRecord{
			ImageAssetID:   strings.TrimSpace(row.ImageAssetID),
			DocumentID:     strings.TrimSpace(row.DocumentID),
			DocumentTitle:  strings.TrimSpace(row.DocumentTitle),
			SpaceID:        strings.TrimSpace(row.SpaceID),
			SpaceName:      strings.TrimSpace(row.SpaceName),
			Status:         strings.ToLower(strings.TrimSpace(row.Status)),
			LastReferenced: recordtime.Parse(row.LastReferencedAtRaw),
		})
	}
	return result, nil
}

func normalizeImageAssetStatuses(input []string) []string {
	if len(input) == 0 {
		return nil
	}
	result := make([]string, 0, len(input))
	exists := make(map[string]struct{}, len(input))
	for _, item := range input {
		value := strings.ToLower(strings.TrimSpace(item))
		switch value {
		case "active", "pending_cleanup", "deleted":
		default:
			continue
		}
		if _, ok := exists[value]; ok {
			continue
		}
		exists[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func normalizeImageAssetStorageProviders(input []string) []string {
	if len(input) == 0 {
		return nil
	}
	result := make([]string, 0, len(input))
	exists := make(map[string]struct{}, len(input))
	for _, item := range input {
		value := strings.ToLower(strings.TrimSpace(item))
		switch value {
		case "local", "cloudflare-r2", "aliyun-oss":
		default:
			continue
		}
		if _, ok := exists[value]; ok {
			continue
		}
		exists[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
