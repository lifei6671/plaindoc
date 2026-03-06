package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

type gormDocumentShareRepository struct {
	db *gorm.DB
}

type documentShareRow struct {
	ID              int64   `gorm:"column:id"`
	ShareID         string  `gorm:"column:share_id"`
	DocumentID      string  `gorm:"column:document_id"`
	SpaceID         string  `gorm:"column:space_id"`
	Mode            string  `gorm:"column:mode"`
	PasswordHash    *string `gorm:"column:password_hash"`
	PasswordHint    string  `gorm:"column:password_hint"`
	ExpiresAtRaw    *string `gorm:"column:expires_at"`
	DisabledAtRaw   *string `gorm:"column:disabled_at"`
	AccessVersion   int     `gorm:"column:access_version"`
	CreatedByUserID *string `gorm:"column:created_by_user_id"`
	UpdatedByUserID *string `gorm:"column:updated_by_user_id"`
	CreatedAtRaw    string  `gorm:"column:created_at"`
	UpdatedAtRaw    string  `gorm:"column:updated_at"`
}

type documentShareAccessRow struct {
	ShareID          string  `gorm:"column:share_id"`
	DocumentID       string  `gorm:"column:document_id"`
	SpaceID          string  `gorm:"column:space_id"`
	DocumentFormat   string  `gorm:"column:document_format"`
	Mode             string  `gorm:"column:mode"`
	PasswordHash     *string `gorm:"column:password_hash"`
	PasswordHint     string  `gorm:"column:password_hint"`
	ExpiresAtRaw     *string `gorm:"column:expires_at"`
	DisabledAtRaw    *string `gorm:"column:disabled_at"`
	AccessVersion    int     `gorm:"column:access_version"`
	DocumentRouteKey string  `gorm:"column:document_route_key"`
}

type adminDocumentShareListRow struct {
	ID              int64   `gorm:"column:id"`
	ShareID         string  `gorm:"column:share_id"`
	DocumentID      string  `gorm:"column:document_id"`
	SpaceID         string  `gorm:"column:space_id"`
	Mode            string  `gorm:"column:mode"`
	PasswordHash    *string `gorm:"column:password_hash"`
	PasswordHint    string  `gorm:"column:password_hint"`
	ExpiresAtRaw    *string `gorm:"column:expires_at"`
	DisabledAtRaw   *string `gorm:"column:disabled_at"`
	AccessVersion   int     `gorm:"column:access_version"`
	CreatedByUserID *string `gorm:"column:created_by_user_id"`
	UpdatedByUserID *string `gorm:"column:updated_by_user_id"`
	CreatedAtRaw    string  `gorm:"column:created_at"`
	UpdatedAtRaw    string  `gorm:"column:updated_at"`

	DocumentRouteKey string `gorm:"column:document_route_key"`
	DocumentTitle    string `gorm:"column:document_title"`
	DocumentFormat   string `gorm:"column:document_format"`
	SpaceName        string `gorm:"column:space_name"`
	SpaceOwnerID     string `gorm:"column:space_owner_user_id"`
	SpaceOwnerName   string `gorm:"column:space_owner_name"`
	SpaceOwnerEmail  string `gorm:"column:space_owner_email"`
	CreatedByName    string `gorm:"column:created_by_name"`
	CreatedByEmail   string `gorm:"column:created_by_email"`
	IsExpired        int    `gorm:"column:is_expired"`
}

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
		"mode":               mode,
		"password_hash":      share.PasswordHash,
		"password_hint":      strings.TrimSpace(share.PasswordHint),
		"expires_at":         share.ExpiresAt,
		"disabled_at":        share.DisabledAt,
		"access_version":     accessVersion,
		"updated_by_user_id": trimOptionalString(share.UpdatedByUserID),
		"updated_at":         now,
	}
	result := r.db.WithContext(ctx).
		Table("document_shares").
		Where("share_id = ?", shareID).
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
		Table("document_shares").
		Select(
			"id, share_id, document_id, space_id, mode, password_hash, password_hint, "+
				"expires_at, disabled_at, access_version, created_by_user_id, updated_by_user_id, created_at, updated_at",
		).
		Where("document_id = ?", normalizedDocumentID).
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
		Table("document_shares").
		Select(
			"id, share_id, document_id, space_id, mode, password_hash, password_hint, "+
				"expires_at, disabled_at, access_version, created_by_user_id, updated_by_user_id, created_at, updated_at",
		).
		Where("share_id = ?", normalizedShareID).
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
	var row documentShareAccessRow
	err := r.db.WithContext(ctx).
		Table("document_shares AS ds").
		Select(
			"ds.share_id",
			"ds.document_id",
			"ds.space_id",
			"d.format AS document_format",
			"ds.mode",
			"ds.password_hash",
			"ds.password_hint",
			"ds.expires_at",
			"ds.disabled_at",
			"ds.access_version",
			"COALESCE(NULLIF(TRIM(n.reader_slug), ''), d.document_id) AS document_route_key",
		).
		Joins("JOIN documents AS d ON d.document_id = ds.document_id").
		Joins("JOIN nodes AS n ON n.node_id = d.node_id").
		Joins("JOIN spaces AS s ON s.space_id = n.space_id").
		Where("ds.space_id = ?", normalizedSpaceID).
		Where("n.space_id = ?", normalizedSpaceID).
		Where("(d.document_id = ? OR d.node_id = ? OR n.reader_slug = ?)", normalizedDocKey, normalizedDocKey, docKeyLower).
		Where("ds.disabled_at IS NULL").
		Where("(ds.expires_at IS NULL OR ds.expires_at > ?)", now).
		Where("d.status = ? AND d.deleted_at IS NULL", models.EntityStatusActive).
		Where("s.status = ? AND s.deleted_at IS NULL", models.EntityStatusActive).
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
		ExpiresAt:        parseNullableRecordTime(row.ExpiresAtRaw),
		DisabledAt:       parseNullableRecordTime(row.DisabledAtRaw),
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

	applyCommonFilters := func(query *gorm.DB) *gorm.DB {
		query = query.Where("ds.disabled_at IS NULL")
		query = query.Where("d.deleted_at IS NULL")
		query = query.Where("s.deleted_at IS NULL")
		if params.RestrictToScopes {
			query = query.Where(
				"(s.owner_user_id = ? OR EXISTS (SELECT 1 FROM space_admin_scopes AS sas WHERE sas.space_id = s.space_id AND sas.user_id = ?))",
				actorUserID,
				actorUserID,
			)
		}
		if view == DocumentShareAdminViewMine {
			query = query.Where("ds.created_by_user_id = ?", actorUserID)
		}
		if keyword != "" {
			query = query.Where("d.title LIKE ?", "%"+keyword+"%")
		}
		if spaceID != "" {
			query = query.Where("s.space_id = ?", spaceID)
		}
		if mode != "" {
			query = query.Where("ds.mode = ?", mode)
		}
		switch expiredFilter {
		case "yes":
			query = query.Where("ds.expires_at IS NOT NULL AND ds.expires_at <= ?", now)
		case "no":
			query = query.Where("(ds.expires_at IS NULL OR ds.expires_at > ?)", now)
		}
		return query
	}

	var total int64
	countQuery := r.db.WithContext(ctx).
		Table("document_shares AS ds").
		Joins("JOIN documents AS d ON d.document_id = ds.document_id").
		Joins("JOIN nodes AS n ON n.node_id = d.node_id").
		Joins("JOIN spaces AS s ON s.space_id = n.space_id")
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
		Table("document_shares AS ds").
		Joins("JOIN documents AS d ON d.document_id = ds.document_id").
		Joins("JOIN nodes AS n ON n.node_id = d.node_id").
		Joins("JOIN spaces AS s ON s.space_id = n.space_id").
		Joins("JOIN users AS uo ON uo.user_id = s.owner_user_id").
		Joins("LEFT JOIN users AS uc ON uc.user_id = ds.created_by_user_id")
	listQuery = applyCommonFilters(listQuery)

	rows := make([]adminDocumentShareListRow, 0, limit)
	if err := listQuery.Session(&gorm.Session{}).
		Select(
			"ds.id, ds.share_id, ds.document_id, ds.space_id, ds.mode, ds.password_hash, ds.password_hint, "+
				"ds.expires_at, ds.disabled_at, ds.access_version, ds.created_by_user_id, ds.updated_by_user_id, "+
				"ds.created_at, ds.updated_at, COALESCE(NULLIF(TRIM(n.reader_slug), ''), d.document_id) AS document_route_key, "+
				"d.title AS document_title, d.format AS document_format, s.name AS space_name, s.owner_user_id AS space_owner_user_id, "+
				"uo.name AS space_owner_name, uo.email AS space_owner_email, "+
				"COALESCE(uc.name, '') AS created_by_name, COALESCE(uc.email, '') AS created_by_email, "+
				"CASE WHEN ds.expires_at IS NOT NULL AND ds.expires_at <= ? THEN 1 ELSE 0 END AS is_expired",
			now,
		).
		Order("ds.updated_at DESC, ds.id DESC").
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
			IsExpired:        row.IsExpired > 0,
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
		ExpiresAt:       parseNullableRecordTime(row.ExpiresAtRaw),
		DisabledAt:      parseNullableRecordTime(row.DisabledAtRaw),
		AccessVersion:   accessVersion,
		CreatedByUserID: trimOptionalString(row.CreatedByUserID),
		UpdatedByUserID: trimOptionalString(row.UpdatedByUserID),
		CreatedAt:       parseRecordTime(row.CreatedAtRaw),
		UpdatedAt:       parseRecordTime(row.UpdatedAtRaw),
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
