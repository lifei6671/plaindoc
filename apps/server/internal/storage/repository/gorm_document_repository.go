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

type gormDocumentRepository struct {
	db                 *gorm.DB
	searchIndexJobRepo SearchIndexJobRepository
}

type documentAccessRow = documentAccessRowDB

type adminDocumentListRow = adminDocumentListRowDB

// NewGormDocumentRepository 创建基于 GORM 的文档仓储实现。
func NewGormDocumentRepository(
	db *gorm.DB,
	searchIndexJobRepos ...SearchIndexJobRepository,
) DocumentRepository {
	var searchIndexJobRepo SearchIndexJobRepository
	if len(searchIndexJobRepos) > 0 {
		searchIndexJobRepo = searchIndexJobRepos[0]
	}
	return &gormDocumentRepository{
		db:                 db,
		searchIndexJobRepo: searchIndexJobRepo,
	}
}

func (r *gormDocumentRepository) Create(ctx context.Context, document *models.Document) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("document repository db is nil")
	}
	if document != nil {
		if !models.IsValidVisibility(document.Visibility) {
			document.Visibility = models.VisibilityMember
		}
		if !models.IsValidEntityStatus(document.Status) {
			document.Status = models.EntityStatusActive
		}
		document.Format = models.NormalizeDocumentFormat(document.Format)
		document.ContentVersion = normalizeContentVersion(document.ContentVersion, document.Version)
		document.SourceBlobID = trimOptionalString(document.SourceBlobID)
		document.SourceFileName = trimOptionalString(document.SourceFileName)
		document.SourceMimeType = trimOptionalString(document.SourceMimeType)
		if document.Format == models.DocumentFormatMarkdown {
			document.SourceBlobID = nil
			document.SourceFileName = nil
			document.SourceMimeType = nil
		}
	}
	return r.db.WithContext(ctx).Create(document).Error
}

func (r *gormDocumentRepository) GetByDocumentID(ctx context.Context, documentID string) (*models.Document, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("document repository db is nil")
	}

	var document models.Document
	if err := r.db.WithContext(ctx).
		Model(&models.Document{}).
		Select(
			models.DocumentColumns.ID,
			models.DocumentColumns.DocumentID,
			models.DocumentColumns.NodeID,
			models.DocumentColumns.ThemeID,
			models.DocumentColumns.Format,
			models.DocumentColumns.Visibility,
			models.DocumentColumns.Status,
			models.DocumentColumns.BannedReason,
			models.DocumentColumns.BannedAt,
			models.DocumentColumns.DeletedAt,
			models.DocumentColumns.Title,
			models.DocumentColumns.ContentMD,
			models.DocumentColumns.Version,
			models.DocumentColumns.SourceBlobID,
			models.DocumentColumns.SourceFileName,
			models.DocumentColumns.SourceMimeType,
			models.DocumentColumns.ContentVersion,
			models.DocumentColumns.CreatedByUserID,
			models.DocumentColumns.UpdatedByUserID,
		).
		Where(models.DocumentColumns.DocumentID+" = ?", documentID).
		Take(&document).Error; err != nil {
		return nil, err
	}
	if !models.IsValidVisibility(document.Visibility) {
		document.Visibility = models.VisibilityMember
	}
	if !models.IsValidEntityStatus(document.Status) {
		document.Status = models.EntityStatusActive
	}
	document.Format = models.NormalizeDocumentFormat(document.Format)
	document.ContentVersion = normalizeContentVersion(document.ContentVersion, document.Version)
	document.SourceBlobID = trimOptionalString(document.SourceBlobID)
	document.SourceFileName = trimOptionalString(document.SourceFileName)
	document.SourceMimeType = trimOptionalString(document.SourceMimeType)
	return &document, nil
}

func (r *gormDocumentRepository) GetAccessByDocumentID(
	ctx context.Context,
	documentID string,
) (*DocumentAccessInfo, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("document repository db is nil")
	}

	var row documentAccessRow
	if err := r.db.WithContext(ctx).
		Table(tableWithAlias(models.Document{}, "d")).
		Select(
			"d."+models.DocumentColumns.ID+" AS id",
			"d."+models.DocumentColumns.DocumentID+" AS document_id",
			"d."+models.DocumentColumns.NodeID+" AS node_id",
			"d."+models.DocumentColumns.ThemeID+" AS theme_id",
			"d."+models.DocumentColumns.Format+" AS format",
			"d."+models.DocumentColumns.Visibility+" AS document_vis",
			"d."+models.DocumentColumns.Status+" AS document_status",
			"d."+models.DocumentColumns.BannedReason+" AS document_ban_reason",
			"d."+models.DocumentColumns.BannedAt+" AS document_banned_at",
			"d."+models.DocumentColumns.DeletedAt+" AS document_deleted_at",
			"d."+models.DocumentColumns.Title+" AS title",
			"d."+models.DocumentColumns.ContentMD+" AS content_md",
			"d."+models.DocumentColumns.Version+" AS version",
			"d."+models.DocumentColumns.SourceBlobID+" AS source_blob_id",
			"d."+models.DocumentColumns.SourceFileName+" AS source_file_name",
			"d."+models.DocumentColumns.SourceMimeType+" AS source_mime_type",
			"d."+models.DocumentColumns.ContentVersion+" AS content_version",
			"d."+models.DocumentColumns.CreatedByUserID+" AS created_by_user_id",
			"d."+models.DocumentColumns.UpdatedByUserID+" AS updated_by_user_id",
			"s."+models.SpaceColumns.SpaceID+" AS space_id",
			"s."+models.SpaceColumns.Name+" AS space_name",
			"s."+models.SpaceColumns.Visibility+" AS space_vis",
			"s."+models.SpaceColumns.Status+" AS space_status",
			"s."+models.SpaceColumns.BannedReason+" AS space_ban_reason",
			"s."+models.SpaceColumns.BannedAt+" AS space_banned_at",
			"s."+models.SpaceColumns.DeletedAt+" AS space_deleted_at",
			"s."+models.SpaceColumns.OwnerUserID+" AS space_owner_user",
		).
		Joins("JOIN "+tableName(models.Node{})+" AS n ON n."+models.NodeColumns.NodeID+" = d."+models.DocumentColumns.NodeID).
		Joins("JOIN "+tableName(models.Space{})+" AS s ON s."+models.SpaceColumns.SpaceID+" = n."+models.NodeColumns.SpaceID).
		Where("d."+models.DocumentColumns.DocumentID+" = ?", documentID).
		Take(&row).Error; err != nil {
		return nil, err
	}

	if !models.IsValidVisibility(row.DocumentVis) {
		row.DocumentVis = models.VisibilityMember
	}
	if !models.IsValidVisibility(row.SpaceVis) {
		row.SpaceVis = models.VisibilityMember
	}
	if !models.IsValidEntityStatus(row.DocumentStatus) {
		row.DocumentStatus = models.EntityStatusActive
	}
	if !models.IsValidEntityStatus(row.SpaceStatus) {
		row.SpaceStatus = models.EntityStatusActive
	}

	return &DocumentAccessInfo{
		Document: models.Document{
			ID:              row.ID,
			DocumentID:      row.DocumentID,
			NodeID:          row.NodeID,
			ThemeID:         row.ThemeID,
			Format:          models.NormalizeDocumentFormat(row.Format),
			Visibility:      row.DocumentVis,
			Status:          row.DocumentStatus,
			BannedReason:    row.DocumentBanReason,
			BannedAt:        row.DocumentBannedAt,
			DeletedAt:       row.DocumentDeletedAt,
			Title:           row.Title,
			ContentMD:       row.ContentMD,
			Version:         row.Version,
			SourceBlobID:    trimOptionalString(row.SourceBlobID),
			SourceFileName:  trimOptionalString(row.SourceFileName),
			SourceMimeType:  trimOptionalString(row.SourceMimeType),
			ContentVersion:  normalizeContentVersion(row.ContentVersion, row.Version),
			CreatedByUserID: row.CreatedByUserID,
			UpdatedByUserID: row.UpdatedByUserID,
		},
		SpaceID:           row.SpaceID,
		SpaceName:         row.SpaceName,
		SpaceVisibility:   row.SpaceVis,
		SpaceStatus:       row.SpaceStatus,
		SpaceBannedAt:     row.SpaceBannedAt,
		SpaceDeletedAt:    row.SpaceDeletedAt,
		SpaceOwnerUserID:  row.SpaceOwnerUser,
		SpaceBannedReason: row.SpaceBanReason,
	}, nil
}

func (r *gormDocumentRepository) ListForAdmin(
	ctx context.Context,
	params ListAdminDocumentsParams,
) ([]AdminDocumentListRecord, int64, error) {
	if r == nil || r.db == nil {
		return nil, 0, fmt.Errorf("document repository db is nil")
	}

	baseQuery := r.db.WithContext(ctx).
		Table(tableWithAlias(models.Document{}, "d")).
		Joins("JOIN " + tableName(models.Node{}) + " AS n ON n." + models.NodeColumns.NodeID + " = d." + models.DocumentColumns.NodeID).
		Joins("JOIN " + tableName(models.Space{}) + " AS s ON s." + models.SpaceColumns.SpaceID + " = n." + models.NodeColumns.SpaceID).
		Joins("JOIN " + tableName(models.User{}) + " AS u ON u." + models.UserColumns.UserID + " = s." + models.SpaceColumns.OwnerUserID)

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
			"LOWER("+qualifiedColumn("d", models.DocumentColumns.DocumentID)+") LIKE ? OR LOWER("+qualifiedColumn("d", models.DocumentColumns.NodeID)+") LIKE ? OR LOWER("+qualifiedColumn("d", models.DocumentColumns.Title)+") LIKE ? OR LOWER("+qualifiedColumn("s", models.SpaceColumns.SpaceID)+") LIKE ? OR LOWER("+qualifiedColumn("s", models.SpaceColumns.Name)+") LIKE ? OR LOWER("+qualifiedColumn("u", models.UserColumns.UserID)+") LIKE ? OR LOWER("+qualifiedColumn("u", models.UserColumns.Email)+") LIKE ? OR LOWER("+qualifiedColumn("u", models.UserColumns.Name)+") LIKE ?",
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

	statuses := normalizeDocumentStatuses(params.Statuses)
	if len(statuses) > 0 {
		baseQuery = baseQuery.Where("d."+models.DocumentColumns.Status+" IN ?", statuses)
	}

	visibilities := normalizeDocumentVisibilities(params.Visibilities)
	if len(visibilities) > 0 {
		baseQuery = baseQuery.Where("d."+models.DocumentColumns.Visibility+" IN ?", visibilities)
	}
	formats := normalizeDocumentFormats(params.Formats)
	if len(formats) > 0 {
		baseQuery = baseQuery.Where("d."+models.DocumentColumns.Format+" IN ?", formats)
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

	var rows []adminDocumentListRow
	if err := baseQuery.Session(&gorm.Session{}).
		Select(
			"d."+models.DocumentColumns.ID+" AS "+models.DocumentColumns.ID,
			"d."+models.DocumentColumns.DocumentID+" AS "+models.DocumentColumns.DocumentID,
			"d."+models.DocumentColumns.NodeID+" AS "+models.DocumentColumns.NodeID,
			"n."+models.NodeColumns.ReaderSlug+" AS "+models.NodeColumns.ReaderSlug,
			"d."+models.DocumentColumns.ThemeID+" AS "+models.DocumentColumns.ThemeID,
			"d."+models.DocumentColumns.Format+" AS "+models.DocumentColumns.Format,
			"d."+models.DocumentColumns.Visibility+" AS "+models.DocumentColumns.Visibility,
			"d."+models.DocumentColumns.Status+" AS "+models.DocumentColumns.Status,
			"d."+models.DocumentColumns.BannedReason+" AS "+models.DocumentColumns.BannedReason,
			"d."+models.DocumentColumns.BannedAt+" AS "+models.DocumentColumns.BannedAt,
			"d."+models.DocumentColumns.DeletedAt+" AS "+models.DocumentColumns.DeletedAt,
			"d."+models.DocumentColumns.Title+" AS "+models.DocumentColumns.Title,
			"d."+models.DocumentColumns.ContentMD+" AS "+models.DocumentColumns.ContentMD,
			"d."+models.DocumentColumns.Version+" AS "+models.DocumentColumns.Version,
			"d."+models.DocumentColumns.SourceBlobID+" AS "+models.DocumentColumns.SourceBlobID,
			"d."+models.DocumentColumns.SourceFileName+" AS "+models.DocumentColumns.SourceFileName,
			"d."+models.DocumentColumns.SourceMimeType+" AS "+models.DocumentColumns.SourceMimeType,
			"d."+models.DocumentColumns.ContentVersion+" AS "+models.DocumentColumns.ContentVersion,
			"d."+models.DocumentColumns.CreatedByUserID+" AS "+models.DocumentColumns.CreatedByUserID,
			"d."+models.DocumentColumns.UpdatedByUserID+" AS "+models.DocumentColumns.UpdatedByUserID,
			"d."+models.DocumentColumns.CreatedAt+" AS created_at_raw",
			"d."+models.DocumentColumns.UpdatedAt+" AS updated_at_raw",
			"s."+models.SpaceColumns.SpaceID+" AS "+models.SpaceColumns.SpaceID,
			"s."+models.SpaceColumns.Name+" AS space_name",
			"s."+models.SpaceColumns.OwnerUserID+" AS space_owner_id",
			"u."+models.UserColumns.Name+" AS space_owner_name",
			"u."+models.UserColumns.Email+" AS space_owner_email",
		).
		Order("d." + models.DocumentColumns.CreatedAt + " DESC").
		Offset(offset).
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	result := make([]AdminDocumentListRecord, 0, len(rows))
	for _, row := range rows {
		document := models.Document{
			ID:              row.ID,
			DocumentID:      row.DocumentID,
			NodeID:          row.NodeID,
			ThemeID:         row.ThemeID,
			Format:          models.NormalizeDocumentFormat(row.Format),
			Visibility:      row.Visibility,
			Status:          row.Status,
			BannedReason:    row.BannedReason,
			BannedAt:        row.BannedAt,
			DeletedAt:       row.DeletedAt,
			Title:           row.Title,
			ContentMD:       row.ContentMD,
			Version:         row.Version,
			SourceBlobID:    trimOptionalString(row.SourceBlobID),
			SourceFileName:  trimOptionalString(row.SourceFileName),
			SourceMimeType:  trimOptionalString(row.SourceMimeType),
			ContentVersion:  normalizeContentVersion(row.ContentVersion, row.Version),
			CreatedByUserID: row.CreatedByUserID,
			UpdatedByUserID: row.UpdatedByUserID,
			CreatedAt:       recordtime.Parse(row.CreatedAtRaw),
			UpdatedAt:       recordtime.Parse(row.UpdatedAtRaw),
		}
		if !models.IsValidVisibility(document.Visibility) {
			document.Visibility = models.VisibilityMember
		}
		if !models.IsValidEntityStatus(document.Status) {
			document.Status = models.EntityStatusActive
		}
		result = append(result, AdminDocumentListRecord{
			Document:         document,
			DocumentRouteKey: resolveAdminDocumentRouteKey(row.DocumentID, row.ReaderSlug),
			SpaceID:          row.SpaceID,
			SpaceName:        row.SpaceName,
			SpaceOwnerID:     row.SpaceOwnerID,
			SpaceOwnerName:   row.SpaceOwnerName,
			SpaceOwnerEmail:  row.SpaceOwnerEmail,
		})
	}

	return result, total, nil
}

func (r *gormDocumentRepository) UpdateTheme(
	ctx context.Context,
	documentID string,
	themeID string,
) (*models.Document, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("document repository db is nil")
	}

	updateTx := r.db.WithContext(ctx).
		Model(&models.Document{}).
		Where(models.DocumentColumns.DocumentID+" = ?", documentID).
		Updates(map[string]any{
			models.DocumentColumns.ThemeID:   themeID,
			models.DocumentColumns.UpdatedAt: gorm.Expr("CURRENT_TIMESTAMP"),
		})
	if updateTx.Error != nil {
		return nil, updateTx.Error
	}
	if updateTx.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	return r.GetByDocumentID(ctx, documentID)
}

func (r *gormDocumentRepository) UpdateVisibility(
	ctx context.Context,
	documentID string,
	visibility models.Visibility,
) (*models.Document, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("document repository db is nil")
	}

	var updated models.Document
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updateTx := tx.Model(&models.Document{}).
			Where(models.DocumentColumns.DocumentID+" = ?", documentID).
			Updates(map[string]any{
				models.DocumentColumns.Visibility: visibility,
				models.DocumentColumns.UpdatedAt:  gorm.Expr("CURRENT_TIMESTAMP"),
			})
		if updateTx.Error != nil {
			return updateTx.Error
		}
		if updateTx.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		if err := r.enqueueDocumentUpsertInTx(ctx, tx, documentID); err != nil {
			return err
		}

		return tx.Select(
			models.DocumentColumns.ID,
			models.DocumentColumns.DocumentID,
			models.DocumentColumns.NodeID,
			models.DocumentColumns.ThemeID,
			models.DocumentColumns.Format,
			models.DocumentColumns.Visibility,
			models.DocumentColumns.Status,
			models.DocumentColumns.BannedReason,
			models.DocumentColumns.BannedAt,
			models.DocumentColumns.DeletedAt,
			models.DocumentColumns.Title,
			models.DocumentColumns.ContentMD,
			models.DocumentColumns.Version,
			models.DocumentColumns.SourceBlobID,
			models.DocumentColumns.SourceFileName,
			models.DocumentColumns.SourceMimeType,
			models.DocumentColumns.ContentVersion,
			models.DocumentColumns.CreatedByUserID,
			models.DocumentColumns.UpdatedByUserID,
		).
			Where(models.DocumentColumns.DocumentID+" = ?", documentID).
			Take(&updated).Error
	})
	if err != nil {
		return nil, err
	}
	if !models.IsValidVisibility(updated.Visibility) {
		updated.Visibility = models.VisibilityMember
	}
	if !models.IsValidEntityStatus(updated.Status) {
		updated.Status = models.EntityStatusActive
	}
	updated.Format = models.NormalizeDocumentFormat(updated.Format)
	updated.ContentVersion = normalizeContentVersion(updated.ContentVersion, updated.Version)
	updated.SourceBlobID = trimOptionalString(updated.SourceBlobID)
	updated.SourceFileName = trimOptionalString(updated.SourceFileName)
	updated.SourceMimeType = trimOptionalString(updated.SourceMimeType)
	return &updated, nil
}

func (r *gormDocumentRepository) UpdateStatus(ctx context.Context, params UpdateDocumentStatusParams) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("document repository db is nil")
	}
	if strings.TrimSpace(params.DocumentID) == "" {
		return false, nil
	}
	if !models.IsValidEntityStatus(params.Status) {
		return false, nil
	}

	updatedAt := params.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}

	updateValues := map[string]any{
		models.DocumentColumns.Status:       params.Status,
		models.DocumentColumns.UpdatedAt:    updatedAt,
		models.DocumentColumns.BannedReason: "",
		models.DocumentColumns.BannedAt:     nil,
	}
	if params.Status == models.EntityStatusBanned {
		updateValues[models.DocumentColumns.BannedReason] = strings.TrimSpace(params.BannedReason)
		updateValues[models.DocumentColumns.BannedAt] = params.BannedAt
	}

	updated := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updateTx := tx.Model(&models.Document{}).
			Where(models.DocumentColumns.DocumentID+" = ?", params.DocumentID).
			Where(models.DocumentColumns.Status+" <> ?", models.EntityStatusDeleted).
			Updates(updateValues)
		if updateTx.Error != nil {
			return updateTx.Error
		}
		if updateTx.RowsAffected == 0 {
			return nil
		}
		updated = true

		var enqueueErr error
		if params.Status == models.EntityStatusBanned || params.Status == models.EntityStatusDeleted {
			enqueueErr = r.enqueueDocumentDeleteInTx(ctx, tx, params.DocumentID)
		} else {
			enqueueErr = r.enqueueDocumentUpsertInTx(ctx, tx, params.DocumentID)
		}
		return enqueueErr
	})
	if err != nil {
		return false, err
	}
	return updated, nil
}

func (r *gormDocumentRepository) SoftDelete(ctx context.Context, documentID string, deletedAt time.Time) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("document repository db is nil")
	}
	if strings.TrimSpace(documentID) == "" {
		return false, nil
	}
	if deletedAt.IsZero() {
		deletedAt = time.Now().UTC()
	}

	deleted := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updateTx := tx.Model(&models.Document{}).
			Where(models.DocumentColumns.DocumentID+" = ?", documentID).
			Where(models.DocumentColumns.Status+" <> ?", models.EntityStatusDeleted).
			Updates(map[string]any{
				models.DocumentColumns.Status:       models.EntityStatusDeleted,
				models.DocumentColumns.DeletedAt:    deletedAt,
				models.DocumentColumns.BannedReason: "",
				models.DocumentColumns.BannedAt:     nil,
				models.DocumentColumns.UpdatedAt:    deletedAt,
			})
		if updateTx.Error != nil {
			return updateTx.Error
		}
		if updateTx.RowsAffected == 0 {
			return nil
		}
		deleted = true
		return r.enqueueDocumentDeleteInTx(ctx, tx, documentID)
	})
	if err != nil {
		return false, err
	}
	return deleted, nil
}

func (r *gormDocumentRepository) HardDelete(ctx context.Context, documentID string) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("document repository db is nil")
	}
	normalizedDocumentID := strings.TrimSpace(documentID)
	if normalizedDocumentID == "" {
		return false, nil
	}

	var documentDeleted bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		deletedDocumentCount, err := DeleteDocumentsCascadeInTx(tx, []string{normalizedDocumentID})
		if err != nil {
			return err
		}
		documentDeleted = deletedDocumentCount > 0
		if !documentDeleted {
			return nil
		}
		return r.enqueueDocumentDeleteInTx(ctx, tx, normalizedDocumentID)
	})
	if err != nil {
		return false, err
	}
	return documentDeleted, nil
}

func (r *gormDocumentRepository) UpdateWithVersion(
	ctx context.Context,
	document *models.Document,
	baseVersion int,
) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("document repository db is nil")
	}
	if document == nil {
		return false, fmt.Errorf("document must not be nil")
	}

	visibility := document.Visibility
	if !models.IsValidVisibility(visibility) {
		visibility = models.VisibilityMember
	}
	status := document.Status
	if !models.IsValidEntityStatus(status) {
		status = models.EntityStatusActive
	}
	documentFormat := models.NormalizeDocumentFormat(document.Format)
	contentVersion := normalizeContentVersion(document.ContentVersion, document.Version)

	updated := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updateTx := tx.Model(&models.Document{}).
			Where(models.DocumentColumns.DocumentID+" = ?", document.DocumentID).
			Where(models.DocumentColumns.Version+" = ?", baseVersion).
			Updates(map[string]any{
				models.DocumentColumns.Title:           document.Title,
				models.DocumentColumns.ContentMD:       document.ContentMD,
				models.DocumentColumns.ThemeID:         document.ThemeID,
				models.DocumentColumns.Format:          documentFormat,
				models.DocumentColumns.Visibility:      visibility,
				models.DocumentColumns.Status:          status,
				models.DocumentColumns.Version:         document.Version,
				models.DocumentColumns.SourceBlobID:    trimOptionalString(document.SourceBlobID),
				models.DocumentColumns.SourceFileName:  trimOptionalString(document.SourceFileName),
				models.DocumentColumns.SourceMimeType:  trimOptionalString(document.SourceMimeType),
				models.DocumentColumns.ContentVersion:  contentVersion,
				models.DocumentColumns.UpdatedByUserID: document.UpdatedByUserID,
				models.DocumentColumns.UpdatedAt:       gorm.Expr("CURRENT_TIMESTAMP"),
			})
		if updateTx.Error != nil {
			return updateTx.Error
		}
		if updateTx.RowsAffected != 1 {
			return nil
		}
		updated = true
		return r.enqueueDocumentUpsertInTx(ctx, tx, document.DocumentID)
	})
	if err != nil {
		return false, err
	}
	return updated, nil
}

func (r *gormDocumentRepository) enqueueDocumentUpsertInTx(
	ctx context.Context,
	tx *gorm.DB,
	documentID string,
) error {
	if r == nil || r.searchIndexJobRepo == nil {
		return nil
	}
	params, err := BuildSearchIndexDocUpsertJob(documentID)
	if err != nil {
		return err
	}
	return r.searchIndexJobRepo.EnqueueInTx(ctx, tx, params)
}

func (r *gormDocumentRepository) enqueueDocumentDeleteInTx(
	ctx context.Context,
	tx *gorm.DB,
	documentID string,
) error {
	if r == nil || r.searchIndexJobRepo == nil {
		return nil
	}
	params, err := BuildSearchIndexDocDeleteJob(documentID)
	if err != nil {
		return err
	}
	return r.searchIndexJobRepo.EnqueueInTx(ctx, tx, params)
}

func normalizeDocumentStatuses(input []models.EntityStatus) []models.EntityStatus {
	if len(input) == 0 {
		return nil
	}
	statuses := make([]models.EntityStatus, 0, len(input))
	seen := make(map[models.EntityStatus]struct{}, len(input))
	for _, status := range input {
		if !models.IsValidEntityStatus(status) {
			continue
		}
		if _, ok := seen[status]; ok {
			continue
		}
		seen[status] = struct{}{}
		statuses = append(statuses, status)
	}
	return statuses
}

func normalizeDocumentVisibilities(input []models.Visibility) []models.Visibility {
	if len(input) == 0 {
		return nil
	}
	visibilities := make([]models.Visibility, 0, len(input))
	seen := make(map[models.Visibility]struct{}, len(input))
	for _, visibility := range input {
		if !models.IsValidVisibility(visibility) {
			continue
		}
		if _, ok := seen[visibility]; ok {
			continue
		}
		seen[visibility] = struct{}{}
		visibilities = append(visibilities, visibility)
	}
	return visibilities
}

func normalizeDocumentFormats(input []models.DocumentFormat) []models.DocumentFormat {
	if len(input) == 0 {
		return nil
	}
	formats := make([]models.DocumentFormat, 0, len(input))
	seen := make(map[models.DocumentFormat]struct{}, len(input))
	for _, item := range input {
		format := models.NormalizeDocumentFormat(item)
		if !models.IsValidDocumentFormat(format) {
			continue
		}
		if _, ok := seen[format]; ok {
			continue
		}
		seen[format] = struct{}{}
		formats = append(formats, format)
	}
	return formats
}

func resolveAdminDocumentRouteKey(documentID string, readerSlug *string) string {
	normalizedReaderSlug := strings.TrimSpace(strings.ToLower(derefOptionalString(readerSlug)))
	if normalizedReaderSlug != "" {
		return normalizedReaderSlug
	}
	return strings.TrimSpace(documentID)
}
