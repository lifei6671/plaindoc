package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

type gormDocumentRepository struct {
	db *gorm.DB
}

type documentAccessRow struct {
	ID                int64               `gorm:"column:id"`
	DocumentID        string              `gorm:"column:document_id"`
	NodeID            string              `gorm:"column:node_id"`
	ThemeID           string              `gorm:"column:theme_id"`
	DocumentVis       models.Visibility   `gorm:"column:document_visibility"`
	DocumentStatus    models.EntityStatus `gorm:"column:document_status"`
	DocumentBanReason string              `gorm:"column:document_banned_reason"`
	DocumentBannedAt  *time.Time          `gorm:"column:document_banned_at"`
	DocumentDeletedAt *time.Time          `gorm:"column:document_deleted_at"`
	Title             string              `gorm:"column:title"`
	ContentMD         string              `gorm:"column:content_md"`
	Version           int                 `gorm:"column:version"`
	CreatedByUserID   *string             `gorm:"column:created_by_user_id"`
	UpdatedByUserID   *string             `gorm:"column:updated_by_user_id"`
	SpaceID           string              `gorm:"column:space_id"`
	SpaceName         string              `gorm:"column:space_name"`
	SpaceVis          models.Visibility   `gorm:"column:space_visibility"`
	SpaceStatus       models.EntityStatus `gorm:"column:space_status"`
	SpaceBanReason    string              `gorm:"column:space_banned_reason"`
	SpaceBannedAt     *time.Time          `gorm:"column:space_banned_at"`
	SpaceDeletedAt    *time.Time          `gorm:"column:space_deleted_at"`
	SpaceOwnerUser    string              `gorm:"column:space_owner_user_id"`
}

// NewGormDocumentRepository 创建基于 GORM 的文档仓储实现。
func NewGormDocumentRepository(db *gorm.DB) DocumentRepository {
	return &gormDocumentRepository{db: db}
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
	}
	return r.db.WithContext(ctx).Create(document).Error
}

func (r *gormDocumentRepository) GetByDocumentID(ctx context.Context, documentID string) (*models.Document, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("document repository db is nil")
	}

	var document models.Document
	if err := r.db.WithContext(ctx).
		Select(
			"id",
			"document_id",
			"node_id",
			"theme_id",
			"visibility",
			"status",
			"banned_reason",
			"banned_at",
			"deleted_at",
			"title",
			"content_md",
			"version",
			"created_by_user_id",
			"updated_by_user_id",
		).
		Where("document_id = ?", documentID).
		Take(&document).Error; err != nil {
		return nil, err
	}
	if !models.IsValidVisibility(document.Visibility) {
		document.Visibility = models.VisibilityMember
	}
	if !models.IsValidEntityStatus(document.Status) {
		document.Status = models.EntityStatusActive
	}
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
		Table("documents AS d").
		Select(
			"d.id AS id",
			"d.document_id AS document_id",
			"d.node_id AS node_id",
			"d.theme_id AS theme_id",
			"d.visibility AS document_visibility",
			"d.status AS document_status",
			"d.banned_reason AS document_banned_reason",
			"d.banned_at AS document_banned_at",
			"d.deleted_at AS document_deleted_at",
			"d.title AS title",
			"d.content_md AS content_md",
			"d.version AS version",
			"d.created_by_user_id AS created_by_user_id",
			"d.updated_by_user_id AS updated_by_user_id",
			"s.space_id AS space_id",
			"s.name AS space_name",
			"s.visibility AS space_visibility",
			"s.status AS space_status",
			"s.banned_reason AS space_banned_reason",
			"s.banned_at AS space_banned_at",
			"s.deleted_at AS space_deleted_at",
			"s.owner_user_id AS space_owner_user_id",
		).
		Joins("JOIN nodes AS n ON n.node_id = d.node_id").
		Joins("JOIN spaces AS s ON s.space_id = n.space_id").
		Where("d.document_id = ?", documentID).
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
			Visibility:      row.DocumentVis,
			Status:          row.DocumentStatus,
			BannedReason:    row.DocumentBanReason,
			BannedAt:        row.DocumentBannedAt,
			DeletedAt:       row.DocumentDeletedAt,
			Title:           row.Title,
			ContentMD:       row.ContentMD,
			Version:         row.Version,
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
		Table("documents AS d").
		Joins("JOIN nodes AS n ON n.node_id = d.node_id").
		Joins("JOIN spaces AS s ON s.space_id = n.space_id").
		Joins("JOIN users AS u ON u.user_id = s.owner_user_id")

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
			"LOWER(d.document_id) LIKE ? OR LOWER(d.node_id) LIKE ? OR LOWER(d.title) LIKE ? OR LOWER(s.space_id) LIKE ? OR LOWER(s.name) LIKE ? OR LOWER(u.user_id) LIKE ? OR LOWER(u.email) LIKE ? OR LOWER(u.name) LIKE ?",
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

	statuses := normalizeDocumentStatuses(params.Statuses)
	if len(statuses) > 0 {
		baseQuery = baseQuery.Where("d.status IN ?", statuses)
	}

	visibilities := normalizeDocumentVisibilities(params.Visibilities)
	if len(visibilities) > 0 {
		baseQuery = baseQuery.Where("d.visibility IN ?", visibilities)
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

	type adminDocumentListRow struct {
		ID              int64               `gorm:"column:id"`
		DocumentID      string              `gorm:"column:document_id"`
		NodeID          string              `gorm:"column:node_id"`
		ThemeID         string              `gorm:"column:theme_id"`
		Visibility      models.Visibility   `gorm:"column:visibility"`
		Status          models.EntityStatus `gorm:"column:status"`
		BannedReason    string              `gorm:"column:banned_reason"`
		BannedAt        *time.Time          `gorm:"column:banned_at"`
		DeletedAt       *time.Time          `gorm:"column:deleted_at"`
		Title           string              `gorm:"column:title"`
		ContentMD       string              `gorm:"column:content_md"`
		Version         int                 `gorm:"column:version"`
		CreatedByUserID *string             `gorm:"column:created_by_user_id"`
		UpdatedByUserID *string             `gorm:"column:updated_by_user_id"`
		CreatedAtRaw    string              `gorm:"column:created_at"`
		UpdatedAtRaw    string              `gorm:"column:updated_at"`
		SpaceID         string              `gorm:"column:space_id"`
		SpaceName       string              `gorm:"column:space_name"`
		SpaceOwnerID    string              `gorm:"column:space_owner_user_id"`
		SpaceOwnerName  string              `gorm:"column:space_owner_name"`
		SpaceOwnerEmail string              `gorm:"column:space_owner_email"`
	}

	var rows []adminDocumentListRow
	if err := baseQuery.Session(&gorm.Session{}).
		Select(
			"d.id",
			"d.document_id",
			"d.node_id",
			"d.theme_id",
			"d.visibility",
			"d.status",
			"d.banned_reason",
			"d.banned_at",
			"d.deleted_at",
			"d.title",
			"d.content_md",
			"d.version",
			"d.created_by_user_id",
			"d.updated_by_user_id",
			"d.created_at",
			"d.updated_at",
			"s.space_id AS space_id",
			"s.name AS space_name",
			"s.owner_user_id AS space_owner_user_id",
			"u.name AS space_owner_name",
			"u.email AS space_owner_email",
		).
		Order("d.created_at DESC").
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
			Visibility:      row.Visibility,
			Status:          row.Status,
			BannedReason:    row.BannedReason,
			BannedAt:        row.BannedAt,
			DeletedAt:       row.DeletedAt,
			Title:           row.Title,
			ContentMD:       row.ContentMD,
			Version:         row.Version,
			CreatedByUserID: row.CreatedByUserID,
			UpdatedByUserID: row.UpdatedByUserID,
			CreatedAt:       parseDocumentRecordTime(row.CreatedAtRaw),
			UpdatedAt:       parseDocumentRecordTime(row.UpdatedAtRaw),
		}
		if !models.IsValidVisibility(document.Visibility) {
			document.Visibility = models.VisibilityMember
		}
		if !models.IsValidEntityStatus(document.Status) {
			document.Status = models.EntityStatusActive
		}
		result = append(result, AdminDocumentListRecord{
			Document:        document,
			SpaceID:         row.SpaceID,
			SpaceName:       row.SpaceName,
			SpaceOwnerID:    row.SpaceOwnerID,
			SpaceOwnerName:  row.SpaceOwnerName,
			SpaceOwnerEmail: row.SpaceOwnerEmail,
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
		Where("document_id = ?", documentID).
		Updates(map[string]any{
			"theme_id":   themeID,
			"updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
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

	updateTx := r.db.WithContext(ctx).
		Model(&models.Document{}).
		Where("document_id = ?", documentID).
		Updates(map[string]any{
			"visibility": visibility,
			"updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
		})
	if updateTx.Error != nil {
		return nil, updateTx.Error
	}
	if updateTx.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	return r.GetByDocumentID(ctx, documentID)
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
		"status":        params.Status,
		"updated_at":    updatedAt,
		"banned_reason": "",
		"banned_at":     nil,
	}
	if params.Status == models.EntityStatusBanned {
		updateValues["banned_reason"] = strings.TrimSpace(params.BannedReason)
		updateValues["banned_at"] = params.BannedAt
	}

	tx := r.db.WithContext(ctx).
		Model(&models.Document{}).
		Where("document_id = ? AND status <> ?", params.DocumentID, models.EntityStatusDeleted).
		Updates(updateValues)
	if tx.Error != nil {
		return false, tx.Error
	}
	return tx.RowsAffected > 0, nil
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

	tx := r.db.WithContext(ctx).
		Model(&models.Document{}).
		Where("document_id = ? AND status <> ?", documentID, models.EntityStatusDeleted).
		Updates(map[string]any{
			"status":        models.EntityStatusDeleted,
			"deleted_at":    deletedAt,
			"banned_reason": "",
			"banned_at":     nil,
			"updated_at":    deletedAt,
		})
	if tx.Error != nil {
		return false, tx.Error
	}
	return tx.RowsAffected > 0, nil
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

	updateTx := r.db.WithContext(ctx).
		Model(&models.Document{}).
		Where("document_id = ? AND version = ?", document.DocumentID, baseVersion).
		Updates(map[string]any{
			"title":              document.Title,
			"content_md":         document.ContentMD,
			"theme_id":           document.ThemeID,
			"visibility":         visibility,
			"status":             status,
			"version":            document.Version,
			"updated_by_user_id": document.UpdatedByUserID,
			"updated_at":         gorm.Expr("CURRENT_TIMESTAMP"),
		})
	if updateTx.Error != nil {
		return false, updateTx.Error
	}
	return updateTx.RowsAffected == 1, nil
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

func parseDocumentRecordTime(raw string) time.Time {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05.999999-07:00",
		"2006-01-02T15:04:05.999-07:00",
		"2006-01-02T15:04:05.999999999-07:00",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05.999999-07:00",
		"2006-01-02 15:04:05.999-07:00",
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05 -0700 MST",
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05.999999999 -0700",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05",
	}
	for _, layout := range layouts {
		if parsedAt, err := time.Parse(layout, value); err == nil {
			return parsedAt.UTC()
		}
	}
	// 兼容数据库返回的 "YYYY-MM-DD HH:MM:SS+00:00" 等格式，统一转为 RFC3339 再解析。
	normalized := strings.Replace(value, " ", "T", 1)
	timePart := normalized
	if index := strings.IndexByte(normalized, 'T'); index >= 0 && index < len(normalized)-1 {
		timePart = normalized[index+1:]
	}
	if !strings.ContainsAny(timePart, "Zz+-") {
		normalized += "Z"
	}
	if parsedAt, err := time.Parse(time.RFC3339Nano, normalized); err == nil {
		return parsedAt.UTC()
	}
	if parsedAt, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.UTC); err == nil {
		return parsedAt.UTC()
	}
	return time.Time{}
}
