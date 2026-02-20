package repository

import (
	"context"
	"fmt"
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
	UpdatedByUserID   *string             `gorm:"column:updated_by_user_id"`
	SpaceID           string              `gorm:"column:space_id"`
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
			"d.updated_by_user_id AS updated_by_user_id",
			"s.space_id AS space_id",
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
			UpdatedByUserID: row.UpdatedByUserID,
		},
		SpaceID:           row.SpaceID,
		SpaceVisibility:   row.SpaceVis,
		SpaceStatus:       row.SpaceStatus,
		SpaceBannedAt:     row.SpaceBannedAt,
		SpaceDeletedAt:    row.SpaceDeletedAt,
		SpaceOwnerUserID:  row.SpaceOwnerUser,
		SpaceBannedReason: row.SpaceBanReason,
	}, nil
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
