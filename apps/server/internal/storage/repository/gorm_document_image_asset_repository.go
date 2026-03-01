package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

type gormDocumentImageAssetRepository struct {
	db *gorm.DB
}

type documentImageAssetRow struct {
	ID                  int64   `gorm:"column:id"`
	ImageAssetID        string  `gorm:"column:image_asset_id"`
	DocumentID          string  `gorm:"column:document_id"`
	SpaceID             string  `gorm:"column:space_id"`
	StorageProvider     string  `gorm:"column:storage_provider"`
	ObjectKey           string  `gorm:"column:object_key"`
	ObjectURL           string  `gorm:"column:object_url"`
	Status              string  `gorm:"column:status"`
	PendingCleanupAtRaw *string `gorm:"column:pending_cleanup_at"`
	DeletedAtRaw        *string `gorm:"column:deleted_at"`
	LastReferencedAtRaw string  `gorm:"column:last_referenced_at"`
	CreatedAtRaw        string  `gorm:"column:created_at"`
	UpdatedAtRaw        string  `gorm:"column:updated_at"`
}

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
		Table("document_image_assets AS dia").
		Joins("JOIN documents AS d ON d.document_id = dia.document_id").
		Joins("JOIN nodes AS n ON n.node_id = d.node_id").
		Joins("JOIN spaces AS s ON s.space_id = n.space_id").
		Joins("JOIN users AS uo ON uo.user_id = s.owner_user_id")

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
			"LOWER(dia.image_asset_id) LIKE ? OR LOWER(dia.document_id) LIKE ? OR LOWER(dia.object_key) LIKE ? OR LOWER(dia.object_url) LIKE ? OR LOWER(COALESCE(dia.status,'')) LIKE ? OR LOWER(d.title) LIKE ? OR LOWER(s.space_id) LIKE ? OR LOWER(s.name) LIKE ? OR LOWER(uo.user_id) LIKE ? OR LOWER(uo.email) LIKE ? OR LOWER(uo.name) LIKE ?",
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
		baseQuery = baseQuery.Where("dia.document_id = ?", documentID)
	}

	statuses := normalizeImageAssetStatuses(params.Statuses)
	if len(statuses) > 0 {
		baseQuery = baseQuery.Where("LOWER(dia.status) IN ?", statuses)
	}

	storageProviders := normalizeImageAssetStorageProviders(params.StorageProviders)
	if len(storageProviders) > 0 {
		baseQuery = baseQuery.Where("LOWER(dia.storage_provider) IN ?", storageProviders)
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

	type adminDocumentImageAssetListRow struct {
		ID                  int64   `gorm:"column:id"`
		ImageAssetID        string  `gorm:"column:image_asset_id"`
		DocumentID          string  `gorm:"column:document_id"`
		SpaceID             string  `gorm:"column:space_id"`
		StorageProvider     string  `gorm:"column:storage_provider"`
		ObjectKey           string  `gorm:"column:object_key"`
		ObjectURL           string  `gorm:"column:object_url"`
		Status              string  `gorm:"column:status"`
		PendingCleanupAtRaw *string `gorm:"column:pending_cleanup_at"`
		DeletedAtRaw        *string `gorm:"column:deleted_at"`
		LastReferencedAtRaw string  `gorm:"column:last_referenced_at"`
		CreatedAtRaw        string  `gorm:"column:created_at"`
		UpdatedAtRaw        string  `gorm:"column:updated_at"`

		DocumentTitle  string              `gorm:"column:document_title"`
		DocumentStatus models.EntityStatus `gorm:"column:document_status"`
		SpaceName      string              `gorm:"column:space_name"`
		SpaceOwnerID   string              `gorm:"column:space_owner_user_id"`
		SpaceOwnerName string              `gorm:"column:space_owner_name"`
		SpaceOwnerMail string              `gorm:"column:space_owner_email"`
	}

	rows := make([]adminDocumentImageAssetListRow, 0, limit)
	if err := baseQuery.Session(&gorm.Session{}).
		Select(
			"dia.id",
			"dia.image_asset_id",
			"dia.document_id",
			"dia.space_id",
			"dia.storage_provider",
			"dia.object_key",
			"dia.object_url",
			"dia.status",
			"dia.pending_cleanup_at",
			"dia.deleted_at",
			"dia.last_referenced_at",
			"dia.created_at",
			"dia.updated_at",
			"d.title AS document_title",
			"d.status AS document_status",
			"s.name AS space_name",
			"s.owner_user_id AS space_owner_user_id",
			"uo.name AS space_owner_name",
			"uo.email AS space_owner_email",
		).
		Order("dia.created_at DESC, dia.id DESC").
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
				StorageProvider:  strings.TrimSpace(row.StorageProvider),
				ObjectKey:        strings.TrimSpace(row.ObjectKey),
				ObjectURL:        strings.TrimSpace(row.ObjectURL),
				Status:           status,
				PendingCleanupAt: parseNullableRecordTime(row.PendingCleanupAtRaw),
				DeletedAt:        parseNullableRecordTime(row.DeletedAtRaw),
				LastReferencedAt: parseRecordTime(row.LastReferencedAtRaw),
				CreatedAt:        parseRecordTime(row.CreatedAtRaw),
				UpdatedAt:        parseRecordTime(row.UpdatedAtRaw),
			},
			DocumentTitle:   strings.TrimSpace(row.DocumentTitle),
			DocumentStatus:  documentStatus,
			SpaceName:       strings.TrimSpace(row.SpaceName),
			SpaceOwnerID:    strings.TrimSpace(row.SpaceOwnerID),
			SpaceOwnerName:  strings.TrimSpace(row.SpaceOwnerName),
			SpaceOwnerEmail: strings.TrimSpace(row.SpaceOwnerMail),
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
		Table("document_image_assets").
		Select(
			"id, image_asset_id, document_id, space_id, storage_provider, object_key, object_url, status, pending_cleanup_at, deleted_at, last_referenced_at, created_at, updated_at",
		).
		Where("image_asset_id = ?", normalizedImageAssetID).
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
		StorageProvider:  strings.TrimSpace(row.StorageProvider),
		ObjectKey:        strings.TrimSpace(row.ObjectKey),
		ObjectURL:        strings.TrimSpace(row.ObjectURL),
		Status:           status,
		PendingCleanupAt: parseNullableRecordTime(row.PendingCleanupAtRaw),
		DeletedAt:        parseNullableRecordTime(row.DeletedAtRaw),
		LastReferencedAt: parseRecordTime(row.LastReferencedAtRaw),
		CreatedAt:        parseRecordTime(row.CreatedAtRaw),
		UpdatedAt:        parseRecordTime(row.UpdatedAtRaw),
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
		Where("image_asset_id = ? AND LOWER(status) <> ?", normalizedImageAssetID, "deleted").
		Updates(map[string]any{
			"status":             "deleted",
			"deleted_at":         deletedAt,
			"updated_at":         deletedAt,
			"pending_cleanup_at": nil,
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
		Where("image_asset_id = ?", normalizedImageAssetID).
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
		Table("document_image_assets").
		Where("LOWER(storage_provider) = ? AND object_key = ? AND LOWER(status) = ?", normalizedProvider, normalizedObjectKey, "active").
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

	type referenceRow struct {
		ImageAssetID        string `gorm:"column:image_asset_id"`
		DocumentID          string `gorm:"column:document_id"`
		DocumentTitle       string `gorm:"column:document_title"`
		SpaceID             string `gorm:"column:space_id"`
		SpaceName           string `gorm:"column:space_name"`
		Status              string `gorm:"column:status"`
		LastReferencedAtRaw string `gorm:"column:last_referenced_at"`
	}

	rows := make([]referenceRow, 0, limit)
	if err := r.db.WithContext(ctx).
		Table("document_image_assets AS dia").
		Select(
			"dia.image_asset_id",
			"dia.document_id",
			"d.title AS document_title",
			"dia.space_id",
			"s.name AS space_name",
			"dia.status",
			"dia.last_referenced_at",
		).
		Joins("JOIN documents AS d ON d.document_id = dia.document_id").
		Joins("JOIN spaces AS s ON s.space_id = dia.space_id").
		Where(
			"LOWER(dia.storage_provider) = ? AND dia.object_key = ? AND LOWER(dia.status) = ?",
			normalizedProvider,
			normalizedObjectKey,
			"active",
		).
		Order("dia.last_referenced_at DESC, dia.id DESC").
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
			LastReferenced: parseRecordTime(row.LastReferencedAtRaw),
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
