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

type gormDocumentShareRepository struct {
	db *gorm.DB
}

type documentShareRow = documentShareRowDB

type documentShareAccessRow = documentShareAccessRowDB

type adminDocumentShareListRow = adminDocumentShareListRowDB

// NewGormDocumentShareRepository 创建基于 GORM 的文档分享仓储实现。
func NewGormDocumentShareRepository(db *gorm.DB) DocumentShareRepository {
	return &gormDocumentShareRepository{db: db}
}

func (r *gormDocumentShareRepository) Create(
	ctx context.Context,
	share *models.DocumentShare,
) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("document share repository db is nil")
	}
	if share == nil {
		return fmt.Errorf("document share is nil")
	}
	share.Mode = normalizeDocumentShareMode(share.Mode)
	if share.AccessVersion <= 0 {
		share.AccessVersion = 1
	}
	share.PasswordHint = strings.TrimSpace(share.PasswordHint)
	return r.db.WithContext(ctx).Create(share).Error
}

func (r *gormDocumentShareRepository) Update(
	ctx context.Context,
	share *models.DocumentShare,
) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("document share repository db is nil")
	}
	if share == nil {
		return false, fmt.Errorf("document share is nil")
	}

	shareID := strings.TrimSpace(share.ShareID)
	if shareID == "" {
		return false, gorm.ErrRecordNotFound
	}
	mode := normalizeDocumentShareMode(share.Mode)
	if mode == "" {
		mode = models.DocumentShareModePublic
	}
	accessVersion := share.AccessVersion
	if accessVersion <= 0 {
		accessVersion = 1
	}

	now := time.Now().UTC()
	if !share.UpdatedAt.IsZero() {
		now = share.UpdatedAt.UTC()
	}
	updates := map[string]any{
		models.DocumentShareColumns.Mode:            mode,
		models.DocumentShareColumns.PasswordHash:    share.PasswordHash,
		models.DocumentShareColumns.PasswordHint:    strings.TrimSpace(share.PasswordHint),
		models.DocumentShareColumns.ExpiresAt:       share.ExpiresAt,
		models.DocumentShareColumns.DisabledAt:      share.DisabledAt,
		models.DocumentShareColumns.AccessVersion:   accessVersion,
		models.DocumentShareColumns.UpdatedByUserID: trimOptionalString(share.UpdatedByUserID),
		models.DocumentShareColumns.UpdatedAt:       now,
	}
	result := r.db.WithContext(ctx).
		Model(&models.DocumentShare{}).
		Where(models.DocumentShareColumns.ShareID+" = ?", shareID).
		Updates(updates)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *gormDocumentShareRepository) GetByDocumentID(
	ctx context.Context,
	documentID string,
) (*models.DocumentShare, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("document share repository db is nil")
	}
	normalizedDocumentID := strings.TrimSpace(documentID)
	if normalizedDocumentID == "" {
		return nil, gorm.ErrRecordNotFound
	}

	var row documentShareRow
	if err := r.db.WithContext(ctx).
		Model(&models.DocumentShare{}).
		Select(
			models.DocumentShareColumns.ID,
			models.DocumentShareColumns.ShareID,
			models.DocumentShareColumns.DocumentID,
			models.DocumentShareColumns.SpaceID,
			models.DocumentShareColumns.Mode,
			models.DocumentShareColumns.PasswordHash,
			models.DocumentShareColumns.PasswordHint,
			models.DocumentShareColumns.ExpiresAt+" AS ExpiresAtRaw",
			models.DocumentShareColumns.DisabledAt+" AS DisabledAtRaw",
			models.DocumentShareColumns.AccessVersion,
			models.DocumentShareColumns.CreatedByUserID,
			models.DocumentShareColumns.UpdatedByUserID,
			models.DocumentShareColumns.CreatedAt+" AS CreatedAtRaw",
			models.DocumentShareColumns.UpdatedAt+" AS UpdatedAtRaw",
		).
		Where(models.DocumentShareColumns.DocumentID+" = ?", normalizedDocumentID).
		Take(&row).Error; err != nil {
		return nil, err
	}
	share := mapDocumentShareRow(row)
	return &share, nil
}

func (r *gormDocumentShareRepository) GetByShareID(
	ctx context.Context,
	shareID string,
) (*models.DocumentShare, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("document share repository db is nil")
	}
	normalizedShareID := strings.TrimSpace(shareID)
	if normalizedShareID == "" {
		return nil, gorm.ErrRecordNotFound
	}

	var row documentShareRow
	if err := r.db.WithContext(ctx).
		Model(&models.DocumentShare{}).
		Select(
			models.DocumentShareColumns.ID,
			models.DocumentShareColumns.ShareID,
			models.DocumentShareColumns.DocumentID,
			models.DocumentShareColumns.SpaceID,
			models.DocumentShareColumns.Mode,
			models.DocumentShareColumns.PasswordHash,
			models.DocumentShareColumns.PasswordHint,
			models.DocumentShareColumns.ExpiresAt+" AS ExpiresAtRaw",
			models.DocumentShareColumns.DisabledAt+" AS DisabledAtRaw",
			models.DocumentShareColumns.AccessVersion,
			models.DocumentShareColumns.CreatedByUserID,
			models.DocumentShareColumns.UpdatedByUserID,
			models.DocumentShareColumns.CreatedAt+" AS CreatedAtRaw",
			models.DocumentShareColumns.UpdatedAt+" AS UpdatedAtRaw",
		).
		Where(models.DocumentShareColumns.ShareID+" = ?", normalizedShareID).
		Take(&row).Error; err != nil {
		return nil, err
	}
	share := mapDocumentShareRow(row)
	return &share, nil
}

func (r *gormDocumentShareRepository) ResolveBySpaceAndDocKey(
	ctx context.Context,
	spaceID string,
	rawDocKey string,
) (*DocumentShareAccessRecord, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("document share repository db is nil")
	}
	normalizedSpaceID := strings.TrimSpace(spaceID)
	normalizedDocKey := strings.TrimSpace(rawDocKey)
	if normalizedSpaceID == "" || normalizedDocKey == "" {
		return nil, gorm.ErrRecordNotFound
	}

	now := time.Now().UTC()
	docKeyLower := strings.ToLower(normalizedDocKey)
	shareAlias := "ds"
	documentAlias := "d"
	nodeAlias := "n"
	spaceAlias := "s"
	var row documentShareAccessRow
	err := r.db.WithContext(ctx).
		Table(tableWithAlias(models.DocumentShare{}, shareAlias)).
		Select(
			qualifiedColumn(shareAlias, models.DocumentShareColumns.ShareID)+" AS share_id",
			qualifiedColumn(shareAlias, models.DocumentShareColumns.DocumentID)+" AS document_id",
			qualifiedColumn(shareAlias, models.DocumentShareColumns.SpaceID)+" AS space_id",
			qualifiedColumn(documentAlias, models.DocumentColumns.Format)+" AS document_format",
			qualifiedColumn(shareAlias, models.DocumentShareColumns.Mode)+" AS mode",
			qualifiedColumn(shareAlias, models.DocumentShareColumns.PasswordHash)+" AS password_hash",
			qualifiedColumn(shareAlias, models.DocumentShareColumns.PasswordHint)+" AS password_hint",
			qualifiedColumn(shareAlias, models.DocumentShareColumns.ExpiresAt)+" AS expires_at_raw",
			qualifiedColumn(shareAlias, models.DocumentShareColumns.DisabledAt)+" AS disabled_at_raw",
			qualifiedColumn(shareAlias, models.DocumentShareColumns.AccessVersion)+" AS access_version",
			"COALESCE(NULLIF(TRIM("+qualifiedColumn(nodeAlias, models.NodeColumns.ReaderSlug)+"), ''), "+qualifiedColumn(documentAlias, models.DocumentColumns.DocumentID)+") AS document_route_key",
		).
		Joins(
			"JOIN "+tableName(models.Document{})+" AS "+documentAlias+
				" ON "+qualifiedColumn(documentAlias, models.DocumentColumns.DocumentID)+" = "+qualifiedColumn(shareAlias, models.DocumentShareColumns.DocumentID),
		).
		Joins(
			"JOIN "+tableName(models.Node{})+" AS "+nodeAlias+
				" ON "+qualifiedColumn(nodeAlias, models.NodeColumns.NodeID)+" = "+qualifiedColumn(documentAlias, models.DocumentColumns.NodeID),
		).
		Joins(
			"JOIN "+tableName(models.Space{})+" AS "+spaceAlias+
				" ON "+qualifiedColumn(spaceAlias, models.SpaceColumns.SpaceID)+" = "+qualifiedColumn(nodeAlias, models.NodeColumns.SpaceID),
		).
		Where(qualifiedColumn(shareAlias, models.DocumentShareColumns.SpaceID)+" = ?", normalizedSpaceID).
		Where(qualifiedColumn(nodeAlias, models.NodeColumns.SpaceID)+" = ?", normalizedSpaceID).
		Where(
			"("+qualifiedColumn(documentAlias, models.DocumentColumns.DocumentID)+" = ? OR "+
				qualifiedColumn(documentAlias, models.DocumentColumns.NodeID)+" = ? OR "+
				qualifiedColumn(nodeAlias, models.NodeColumns.ReaderSlug)+" = ?)",
			normalizedDocKey,
			normalizedDocKey,
			docKeyLower,
		).
		Where(qualifiedColumn(shareAlias, models.DocumentShareColumns.DisabledAt)+" IS NULL").
		Where("("+qualifiedColumn(shareAlias, models.DocumentShareColumns.ExpiresAt)+" IS NULL OR "+qualifiedColumn(shareAlias, models.DocumentShareColumns.ExpiresAt)+" > ?)", now).
		Where(qualifiedColumn(documentAlias, models.DocumentColumns.Status)+" = ?", models.EntityStatusActive).
		Where(qualifiedColumn(documentAlias, models.DocumentColumns.DeletedAt)+" IS NULL").
		Where(qualifiedColumn(spaceAlias, models.SpaceColumns.Status)+" = ?", models.EntityStatusActive).
		Where(qualifiedColumn(spaceAlias, models.SpaceColumns.DeletedAt) + " IS NULL").
		Take(&row).Error
	if err != nil {
		return nil, err
	}

	mode := normalizeDocumentShareMode(models.DocumentShareMode(row.Mode))
	if mode == "" {
		mode = models.DocumentShareModePublic
	}
	accessVersion := row.AccessVersion
	if accessVersion <= 0 {
		accessVersion = 1
	}
	return &DocumentShareAccessRecord{
		ShareID:          strings.TrimSpace(row.ShareID),
		DocumentID:       strings.TrimSpace(row.DocumentID),
		SpaceID:          strings.TrimSpace(row.SpaceID),
		DocumentFormat:   models.NormalizeDocumentFormat(models.DocumentFormat(row.DocumentFormat)),
		Mode:             mode,
		PasswordHash:     trimOptionalString(row.PasswordHash),
		PasswordHint:     strings.TrimSpace(row.PasswordHint),
		ExpiresAt:        recordtime.ParseNullable(row.ExpiresAtRaw),
		DisabledAt:       recordtime.ParseNullable(row.DisabledAtRaw),
		AccessVersion:    accessVersion,
		DocumentRouteKey: strings.TrimSpace(row.DocumentRouteKey),
	}, nil
}

func (r *gormDocumentShareRepository) ListForAdmin(
	ctx context.Context,
	params ListAdminDocumentSharesParams,
) ([]AdminDocumentShareListRecord, int64, error) {
	if r == nil || r.db == nil {
		return nil, 0, fmt.Errorf("document share repository db is nil")
	}

	actorUserID := strings.TrimSpace(params.ActorUserID)
	view := normalizeDocumentShareAdminView(params.View)
	keyword := strings.TrimSpace(params.Keyword)
	spaceID := strings.TrimSpace(params.SpaceID)
	mode := normalizeDocumentShareMode(params.Mode)
	now := time.Now().UTC()
	expiredFilter := normalizeDocumentShareExpiredFilter(params.Expired)
	shareAlias := "ds"
	documentAlias := "d"
	nodeAlias := "n"
	spaceAlias := "s"
	ownerAlias := "uo"
	creatorAlias := "uc"

	applyCommonFilters := func(query *gorm.DB) *gorm.DB {
		query = query.Where(qualifiedColumn(shareAlias, models.DocumentShareColumns.DisabledAt) + " IS NULL")
		query = query.Where(qualifiedColumn(documentAlias, models.DocumentColumns.DeletedAt) + " IS NULL")
		query = query.Where(qualifiedColumn(spaceAlias, models.SpaceColumns.DeletedAt) + " IS NULL")
		if params.RestrictToScopes {
			spaceAdminScopeQuery := r.db.WithContext(ctx).
				Model(&models.SpaceAdminScope{}).
				Select("1").
				Where(qualifiedColumn("", models.SpaceAdminScopeColumns.SpaceID)+" = "+qualifiedColumn(spaceAlias, models.SpaceColumns.SpaceID)).
				Where(qualifiedColumn("", models.SpaceAdminScopeColumns.UserID)+" = ?", actorUserID)
			query = query.Where(
				"("+qualifiedColumn(spaceAlias, models.SpaceColumns.OwnerUserID)+" = ? OR EXISTS (?))",
				actorUserID,
				spaceAdminScopeQuery,
			)
		}
		if view == DocumentShareAdminViewMine {
			query = query.Where(qualifiedColumn(shareAlias, models.DocumentShareColumns.CreatedByUserID)+" = ?", actorUserID)
		}
		if keyword != "" {
			query = query.Where(qualifiedColumn(documentAlias, models.DocumentColumns.Title)+" LIKE ?", "%"+keyword+"%")
		}
		if spaceID != "" {
			query = query.Where(qualifiedColumn(spaceAlias, models.SpaceColumns.SpaceID)+" = ?", spaceID)
		}
		if mode != "" {
			query = query.Where(qualifiedColumn(shareAlias, models.DocumentShareColumns.Mode)+" = ?", mode)
		}
		switch expiredFilter {
		case "yes":
			query = query.
				Where(qualifiedColumn(shareAlias, models.DocumentShareColumns.ExpiresAt)+" IS NOT NULL").
				Where(qualifiedColumn(shareAlias, models.DocumentShareColumns.ExpiresAt)+" <= ?", now)
		case "no":
			query = query.Where("("+qualifiedColumn(shareAlias, models.DocumentShareColumns.ExpiresAt)+" IS NULL OR "+
				qualifiedColumn(shareAlias, models.DocumentShareColumns.ExpiresAt)+" > ?)", now)
		}
		return query
	}

	var total int64
	countQuery := r.db.WithContext(ctx).
		Table(tableWithAlias(models.DocumentShare{}, shareAlias)).
		Joins(
			"JOIN " + tableName(models.Document{}) + " AS " + documentAlias +
				" ON " + qualifiedColumn(documentAlias, models.DocumentColumns.DocumentID) + " = " + qualifiedColumn(shareAlias, models.DocumentShareColumns.DocumentID),
		).
		Joins(
			"JOIN " + tableName(models.Node{}) + " AS " + nodeAlias +
				" ON " + qualifiedColumn(nodeAlias, models.NodeColumns.NodeID) + " = " + qualifiedColumn(documentAlias, models.DocumentColumns.NodeID),
		).
		Joins(
			"JOIN " + tableName(models.Space{}) + " AS " + spaceAlias +
				" ON " + qualifiedColumn(spaceAlias, models.SpaceColumns.SpaceID) + " = " + qualifiedColumn(nodeAlias, models.NodeColumns.SpaceID),
		)
	countQuery = applyCommonFilters(countQuery)
	if err := countQuery.Session(&gorm.Session{}).Count(&total).Error; err != nil {
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

	listQuery := r.db.WithContext(ctx).
		Table(tableWithAlias(models.DocumentShare{}, shareAlias)).
		Joins(
			"JOIN " + tableName(models.Document{}) + " AS " + documentAlias +
				" ON " + qualifiedColumn(documentAlias, models.DocumentColumns.DocumentID) + " = " + qualifiedColumn(shareAlias, models.DocumentShareColumns.DocumentID),
		).
		Joins(
			"JOIN " + tableName(models.Node{}) + " AS " + nodeAlias +
				" ON " + qualifiedColumn(nodeAlias, models.NodeColumns.NodeID) + " = " + qualifiedColumn(documentAlias, models.DocumentColumns.NodeID),
		).
		Joins(
			"JOIN " + tableName(models.Space{}) + " AS " + spaceAlias +
				" ON " + qualifiedColumn(spaceAlias, models.SpaceColumns.SpaceID) + " = " + qualifiedColumn(nodeAlias, models.NodeColumns.SpaceID),
		).
		Joins(
			"JOIN " + tableName(models.User{}) + " AS " + ownerAlias +
				" ON " + qualifiedColumn(ownerAlias, models.UserColumns.UserID) + " = " + qualifiedColumn(spaceAlias, models.SpaceColumns.OwnerUserID),
		).
		Joins(
			"LEFT JOIN " + tableName(models.User{}) + " AS " + creatorAlias +
				" ON " + qualifiedColumn(creatorAlias, models.UserColumns.UserID) + " = " + qualifiedColumn(shareAlias, models.DocumentShareColumns.CreatedByUserID),
		)
	listQuery = applyCommonFilters(listQuery)

	rows := make([]adminDocumentShareListRow, 0, limit)
	if err := listQuery.Session(&gorm.Session{}).
		Select(
			qualifiedColumn(shareAlias, models.DocumentShareColumns.ID)+" AS "+models.DocumentShareColumns.ID,
			qualifiedColumn(shareAlias, models.DocumentShareColumns.ShareID)+" AS "+models.DocumentShareColumns.ShareID,
			qualifiedColumn(shareAlias, models.DocumentShareColumns.DocumentID)+" AS "+models.DocumentShareColumns.DocumentID,
			qualifiedColumn(shareAlias, models.DocumentShareColumns.SpaceID)+" AS "+models.DocumentShareColumns.SpaceID,
			qualifiedColumn(shareAlias, models.DocumentShareColumns.Mode)+" AS "+models.DocumentShareColumns.Mode,
			qualifiedColumn(shareAlias, models.DocumentShareColumns.PasswordHash)+" AS "+models.DocumentShareColumns.PasswordHash,
			qualifiedColumn(shareAlias, models.DocumentShareColumns.PasswordHint)+" AS "+models.DocumentShareColumns.PasswordHint,
			qualifiedColumn(shareAlias, models.DocumentShareColumns.ExpiresAt)+" AS expires_at_raw",
			qualifiedColumn(shareAlias, models.DocumentShareColumns.DisabledAt)+" AS disabled_at_raw",
			qualifiedColumn(shareAlias, models.DocumentShareColumns.AccessVersion)+" AS "+models.DocumentShareColumns.AccessVersion,
			qualifiedColumn(shareAlias, models.DocumentShareColumns.CreatedByUserID)+" AS "+models.DocumentShareColumns.CreatedByUserID,
			qualifiedColumn(shareAlias, models.DocumentShareColumns.UpdatedByUserID)+" AS "+models.DocumentShareColumns.UpdatedByUserID,
			qualifiedColumn(shareAlias, models.DocumentShareColumns.CreatedAt)+" AS created_at_raw",
			qualifiedColumn(shareAlias, models.DocumentShareColumns.UpdatedAt)+" AS updated_at_raw",
			"COALESCE(NULLIF(TRIM("+qualifiedColumn(nodeAlias, models.NodeColumns.ReaderSlug)+"), ''), "+
				qualifiedColumn(documentAlias, models.DocumentColumns.DocumentID)+") AS document_route_key",
			qualifiedColumn(documentAlias, models.DocumentColumns.Title)+" AS document_title",
			qualifiedColumn(documentAlias, models.DocumentColumns.Format)+" AS document_format",
			qualifiedColumn(spaceAlias, models.SpaceColumns.Name)+" AS space_name",
			qualifiedColumn(spaceAlias, models.SpaceColumns.OwnerUserID)+" AS space_owner_id",
			qualifiedColumn(ownerAlias, models.UserColumns.Name)+" AS space_owner_name",
			qualifiedColumn(ownerAlias, models.UserColumns.Email)+" AS space_owner_email",
			"COALESCE("+qualifiedColumn(creatorAlias, models.UserColumns.Name)+", '') AS created_by_name",
			"COALESCE("+qualifiedColumn(creatorAlias, models.UserColumns.Email)+", '') AS created_by_email",
		).
		Order(qualifiedColumn(shareAlias, models.DocumentShareColumns.UpdatedAt) + " DESC").
		Order(qualifiedColumn(shareAlias, models.DocumentShareColumns.ID) + " DESC").
		Offset(offset).
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	result := make([]AdminDocumentShareListRecord, 0, len(rows))
	for _, row := range rows {
		share := mapDocumentShareRow(documentShareRow{
			ID:              row.ID,
			ShareID:         row.ShareID,
			DocumentID:      row.DocumentID,
			SpaceID:         row.SpaceID,
			Mode:            row.Mode,
			PasswordHash:    row.PasswordHash,
			PasswordHint:    row.PasswordHint,
			ExpiresAtRaw:    row.ExpiresAtRaw,
			DisabledAtRaw:   row.DisabledAtRaw,
			AccessVersion:   row.AccessVersion,
			CreatedByUserID: row.CreatedByUserID,
			UpdatedByUserID: row.UpdatedByUserID,
			CreatedAtRaw:    row.CreatedAtRaw,
			UpdatedAtRaw:    row.UpdatedAtRaw,
		})
		expiresAt := recordtime.ParseNullable(row.ExpiresAtRaw)
		result = append(result, AdminDocumentShareListRecord{
			Share:            share,
			DocumentRouteKey: strings.TrimSpace(row.DocumentRouteKey),
			DocumentTitle:    strings.TrimSpace(row.DocumentTitle),
			DocumentFormat:   models.NormalizeDocumentFormat(models.DocumentFormat(row.DocumentFormat)),
			SpaceName:        strings.TrimSpace(row.SpaceName),
			SpaceOwnerID:     strings.TrimSpace(row.SpaceOwnerID),
			SpaceOwnerName:   strings.TrimSpace(row.SpaceOwnerName),
			SpaceOwnerEmail:  strings.TrimSpace(row.SpaceOwnerEmail),
			CreatedByName:    strings.TrimSpace(row.CreatedByName),
			CreatedByEmail:   strings.TrimSpace(row.CreatedByEmail),
			IsExpired:        expiresAt != nil && !expiresAt.After(now),
		})
	}
	return result, total, nil
}

func mapDocumentShareRow(row documentShareRow) models.DocumentShare {
	mode := normalizeDocumentShareMode(models.DocumentShareMode(row.Mode))
	if mode == "" {
		mode = models.DocumentShareModePublic
	}
	accessVersion := row.AccessVersion
	if accessVersion <= 0 {
		accessVersion = 1
	}
	return models.DocumentShare{
		ID:              row.ID,
		ShareID:         strings.TrimSpace(row.ShareID),
		DocumentID:      strings.TrimSpace(row.DocumentID),
		SpaceID:         strings.TrimSpace(row.SpaceID),
		Mode:            mode,
		PasswordHash:    trimOptionalString(row.PasswordHash),
		PasswordHint:    strings.TrimSpace(row.PasswordHint),
		ExpiresAt:       recordtime.ParseNullable(row.ExpiresAtRaw),
		DisabledAt:      recordtime.ParseNullable(row.DisabledAtRaw),
		AccessVersion:   accessVersion,
		CreatedByUserID: trimOptionalString(row.CreatedByUserID),
		UpdatedByUserID: trimOptionalString(row.UpdatedByUserID),
		CreatedAt:       recordtime.Parse(row.CreatedAtRaw),
		UpdatedAt:       recordtime.Parse(row.UpdatedAtRaw),
	}
}

func normalizeDocumentShareMode(value models.DocumentShareMode) models.DocumentShareMode {
	mode := models.DocumentShareMode(strings.ToLower(strings.TrimSpace(string(value))))
	if !models.IsValidDocumentShareMode(mode) {
		return ""
	}
	return mode
}

func normalizeDocumentShareAdminView(value DocumentShareAdminView) DocumentShareAdminView {
	view := DocumentShareAdminView(strings.ToLower(strings.TrimSpace(string(value))))
	switch view {
	case DocumentShareAdminViewMine:
		return DocumentShareAdminViewMine
	default:
		return DocumentShareAdminViewAll
	}
}

func normalizeDocumentShareExpiredFilter(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "yes", "expired", "true":
		return "yes"
	case "no", "active", "false":
		return "no"
	default:
		return "all"
	}
}
