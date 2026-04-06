package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type gormSpaceRepository struct {
	db                 *gorm.DB
	searchIndexJobRepo SearchIndexJobRepository
}

// NewGormSpaceRepository 创建基于 GORM 的空间仓储实现。
func NewGormSpaceRepository(
	db *gorm.DB,
	searchIndexJobRepos ...SearchIndexJobRepository,
) SpaceRepository {
	var searchIndexJobRepo SearchIndexJobRepository
	if len(searchIndexJobRepos) > 0 {
		searchIndexJobRepo = searchIndexJobRepos[0]
	}
	return &gormSpaceRepository{
		db:                 db,
		searchIndexJobRepo: searchIndexJobRepo,
	}
}

func (r *gormSpaceRepository) Create(ctx context.Context, space *models.Space) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("space repository db is nil")
	}
	if space != nil {
		space.Name = strings.TrimSpace(space.Name)
		space.Description = strings.TrimSpace(space.Description)
		space.CategoryID = strings.TrimSpace(space.CategoryID)
		space.Category = strings.TrimSpace(space.Category)
		if !models.IsValidVisibility(space.Visibility) {
			space.Visibility = models.VisibilityMember
		}
		if !models.IsValidEntityStatus(space.Status) {
			space.Status = models.EntityStatusActive
		}
		space.CoverKey = strings.TrimSpace(space.CoverKey)
		space.CoverURL = strings.TrimSpace(space.CoverURL)
		space.CoverSource = strings.TrimSpace(space.CoverSource)
		if space.CoverWidth < 0 {
			space.CoverWidth = 0
		}
		if space.CoverHeight < 0 {
			space.CoverHeight = 0
		}
		if space.CoverAssetID != nil {
			trimmedAssetID := strings.TrimSpace(*space.CoverAssetID)
			if trimmedAssetID == "" {
				space.CoverAssetID = nil
			} else {
				space.CoverAssetID = &trimmedAssetID
			}
		}
	}
	return r.db.WithContext(ctx).Create(space).Error
}

func (r *gormSpaceRepository) GetBySpaceID(ctx context.Context, spaceID string) (*models.Space, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("space repository db is nil")
	}

	type spaceRecordRow struct {
		ID           int64               `gorm:"column:id"`
		SpaceID      string              `gorm:"column:space_id"`
		Name         string              `gorm:"column:name"`
		Description  string              `gorm:"column:description"`
		CategoryID   string              `gorm:"column:category_id"`
		Category     string              `gorm:"column:category"`
		OwnerUserID  string              `gorm:"column:owner_user_id"`
		Visibility   models.Visibility   `gorm:"column:visibility"`
		CoverAssetID *string             `gorm:"column:cover_asset_id"`
		CoverKey     string              `gorm:"column:cover_key"`
		CoverURL     string              `gorm:"column:cover_url"`
		CoverWidth   int                 `gorm:"column:cover_width"`
		CoverHeight  int                 `gorm:"column:cover_height"`
		CoverSource  string              `gorm:"column:cover_source"`
		Status       models.EntityStatus `gorm:"column:status"`
		BannedReason string              `gorm:"column:banned_reason"`
		BannedAt     *time.Time          `gorm:"column:banned_at"`
		DeletedAt    *time.Time          `gorm:"column:deleted_at"`
		CreatedAtRaw string              `gorm:"column:created_at"`
		UpdatedAtRaw string              `gorm:"column:updated_at"`
	}

	var row spaceRecordRow
	if err := r.db.WithContext(ctx).
		Model(&models.Space{}).
		Select(selectColumns(
			qualifiedColumn("", models.SpaceColumns.ID),
			qualifiedColumn("", models.SpaceColumns.SpaceID),
			qualifiedColumn("", models.SpaceColumns.Name),
			qualifiedColumn("", models.SpaceColumns.Description),
			qualifiedColumn("", models.SpaceColumns.CategoryID),
			qualifiedColumn("", models.SpaceColumns.Category),
			qualifiedColumn("", models.SpaceColumns.OwnerUserID),
			qualifiedColumn("", models.SpaceColumns.Visibility),
			qualifiedColumn("", models.SpaceColumns.CoverAssetID),
			qualifiedColumn("", models.SpaceColumns.CoverKey),
			qualifiedColumn("", models.SpaceColumns.CoverURL),
			qualifiedColumn("", models.SpaceColumns.CoverWidth),
			qualifiedColumn("", models.SpaceColumns.CoverHeight),
			qualifiedColumn("", models.SpaceColumns.CoverSource),
			qualifiedColumn("", models.SpaceColumns.Status),
			qualifiedColumn("", models.SpaceColumns.BannedReason),
			qualifiedColumn("", models.SpaceColumns.BannedAt),
			qualifiedColumn("", models.SpaceColumns.DeletedAt),
			qualifiedColumn("", models.SpaceColumns.CreatedAt),
			qualifiedColumn("", models.SpaceColumns.UpdatedAt),
		)).
		Where(qualifiedColumn("", models.SpaceColumns.SpaceID)+" = ?", spaceID).
		Take(&row).Error; err != nil {
		return nil, err
	}

	space := models.Space{
		ID:           row.ID,
		SpaceID:      row.SpaceID,
		Name:         row.Name,
		Description:  row.Description,
		CategoryID:   row.CategoryID,
		Category:     row.Category,
		OwnerUserID:  row.OwnerUserID,
		Visibility:   row.Visibility,
		CoverAssetID: row.CoverAssetID,
		CoverKey:     row.CoverKey,
		CoverURL:     row.CoverURL,
		CoverWidth:   row.CoverWidth,
		CoverHeight:  row.CoverHeight,
		CoverSource:  row.CoverSource,
		Status:       row.Status,
		BannedReason: row.BannedReason,
		BannedAt:     row.BannedAt,
		DeletedAt:    row.DeletedAt,
		CreatedAt:    parseSpaceRecordTime(row.CreatedAtRaw),
		UpdatedAt:    parseSpaceRecordTime(row.UpdatedAtRaw),
	}
	if !models.IsValidVisibility(space.Visibility) {
		space.Visibility = models.VisibilityMember
	}
	if !models.IsValidEntityStatus(space.Status) {
		space.Status = models.EntityStatusActive
	}
	space.Description = strings.TrimSpace(space.Description)
	space.CategoryID = strings.TrimSpace(space.CategoryID)
	space.Category = strings.TrimSpace(space.Category)
	space.CoverKey = strings.TrimSpace(space.CoverKey)
	space.CoverURL = strings.TrimSpace(space.CoverURL)
	space.CoverSource = strings.TrimSpace(space.CoverSource)
	if space.CoverWidth < 0 {
		space.CoverWidth = 0
	}
	if space.CoverHeight < 0 {
		space.CoverHeight = 0
	}
	if space.CoverAssetID != nil {
		trimmedAssetID := strings.TrimSpace(*space.CoverAssetID)
		if trimmedAssetID == "" {
			space.CoverAssetID = nil
		} else {
			space.CoverAssetID = &trimmedAssetID
		}
	}
	return &space, nil
}

func (r *gormSpaceRepository) GetCoverAssetByAssetID(
	ctx context.Context,
	assetID string,
) (*models.SpaceCoverAsset, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("space repository db is nil")
	}

	var asset models.SpaceCoverAsset
	if err := r.db.WithContext(ctx).
		Select(selectColumns(
			qualifiedColumn("", models.SpaceCoverAssetColumns.ID),
			qualifiedColumn("", models.SpaceCoverAssetColumns.AssetID),
			qualifiedColumn("", models.SpaceCoverAssetColumns.Source),
			qualifiedColumn("", models.SpaceCoverAssetColumns.ObjectKey),
			qualifiedColumn("", models.SpaceCoverAssetColumns.ObjectURL),
			qualifiedColumn("", models.SpaceCoverAssetColumns.MimeType),
			qualifiedColumn("", models.SpaceCoverAssetColumns.Width),
			qualifiedColumn("", models.SpaceCoverAssetColumns.Height),
			qualifiedColumn("", models.SpaceCoverAssetColumns.SizeBytes),
			qualifiedColumn("", models.SpaceCoverAssetColumns.Normalized),
			qualifiedColumn("", models.SpaceCoverAssetColumns.CreatedByUserID),
			qualifiedColumn("", models.SpaceCoverAssetColumns.CreatedAt),
			qualifiedColumn("", models.SpaceCoverAssetColumns.UpdatedAt),
		)).
		Where(qualifiedColumn("", models.SpaceCoverAssetColumns.AssetID)+" = ?", strings.TrimSpace(assetID)).
		Take(&asset).Error; err != nil {
		return nil, err
	}

	asset.AssetID = strings.TrimSpace(asset.AssetID)
	asset.Source = strings.TrimSpace(asset.Source)
	asset.ObjectKey = strings.TrimSpace(asset.ObjectKey)
	asset.ObjectURL = strings.TrimSpace(asset.ObjectURL)
	asset.MimeType = strings.TrimSpace(asset.MimeType)
	asset.CreatedByUserID = strings.TrimSpace(asset.CreatedByUserID)
	if asset.Width < 0 {
		asset.Width = 0
	}
	if asset.Height < 0 {
		asset.Height = 0
	}
	if asset.SizeBytes < 0 {
		asset.SizeBytes = 0
	}
	return &asset, nil
}

func (r *gormSpaceRepository) ListByUserID(ctx context.Context, userID string) ([]models.Space, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("space repository db is nil")
	}

	var spaces []models.Space
	if err := r.db.WithContext(ctx).
		Table(tableWithAlias(models.Space{}, "s")).
		Select(selectColumns(
			qualifiedColumn("s", models.SpaceColumns.ID),
			qualifiedColumn("s", models.SpaceColumns.SpaceID),
			qualifiedColumn("s", models.SpaceColumns.Name),
			qualifiedColumn("s", models.SpaceColumns.Description),
			qualifiedColumn("s", models.SpaceColumns.CategoryID),
			qualifiedColumn("s", models.SpaceColumns.Category),
			qualifiedColumn("s", models.SpaceColumns.OwnerUserID),
			qualifiedColumn("s", models.SpaceColumns.Visibility),
			qualifiedColumn("s", models.SpaceColumns.CoverAssetID),
			qualifiedColumn("s", models.SpaceColumns.CoverKey),
			qualifiedColumn("s", models.SpaceColumns.CoverURL),
			qualifiedColumn("s", models.SpaceColumns.CoverWidth),
			qualifiedColumn("s", models.SpaceColumns.CoverHeight),
			qualifiedColumn("s", models.SpaceColumns.CoverSource),
			qualifiedColumn("s", models.SpaceColumns.Status),
			qualifiedColumn("s", models.SpaceColumns.BannedReason),
			qualifiedColumn("s", models.SpaceColumns.BannedAt),
			qualifiedColumn("s", models.SpaceColumns.DeletedAt),
		)).
		Joins("LEFT JOIN "+tableName(models.SpaceMember{})+" AS sm ON sm."+models.SpaceMemberColumns.SpaceID+" = "+qualifiedColumn("s", models.SpaceColumns.SpaceID)+" AND sm."+models.SpaceMemberColumns.UserID+" = ?", userID).
		Where(qualifiedColumn("s", models.SpaceColumns.OwnerUserID)+" = ? OR sm.id IS NOT NULL", userID).
		Order(qualifiedColumn("s", models.SpaceColumns.ID) + " DESC").
		Find(&spaces).Error; err != nil {
		return nil, err
	}

	for i := range spaces {
		spaces[i].Name = strings.TrimSpace(spaces[i].Name)
		spaces[i].Description = strings.TrimSpace(spaces[i].Description)
		spaces[i].CategoryID = strings.TrimSpace(spaces[i].CategoryID)
		spaces[i].Category = strings.TrimSpace(spaces[i].Category)
		if !models.IsValidVisibility(spaces[i].Visibility) {
			spaces[i].Visibility = models.VisibilityMember
		}
		if !models.IsValidEntityStatus(spaces[i].Status) {
			spaces[i].Status = models.EntityStatusActive
		}
		spaces[i].CoverKey = strings.TrimSpace(spaces[i].CoverKey)
		spaces[i].CoverURL = strings.TrimSpace(spaces[i].CoverURL)
		spaces[i].CoverSource = strings.TrimSpace(spaces[i].CoverSource)
		if spaces[i].CoverWidth < 0 {
			spaces[i].CoverWidth = 0
		}
		if spaces[i].CoverHeight < 0 {
			spaces[i].CoverHeight = 0
		}
		if spaces[i].CoverAssetID != nil {
			trimmedAssetID := strings.TrimSpace(*spaces[i].CoverAssetID)
			if trimmedAssetID == "" {
				spaces[i].CoverAssetID = nil
			} else {
				spaces[i].CoverAssetID = &trimmedAssetID
			}
		}
	}

	return spaces, nil
}

func (r *gormSpaceRepository) ListVisibleForHomepage(
	ctx context.Context,
	params ListVisibleHomepageSpacesParams,
) ([]HomepageVisibleSpaceRecord, int64, error) {
	if r == nil || r.db == nil {
		return nil, 0, fmt.Errorf("space repository db is nil")
	}

	viewerUserID := strings.TrimSpace(params.ViewerUserID)
	categoryID := strings.TrimSpace(params.CategoryID)

	// 首页空间展示与阅读页权限必须一致：
	// 至少存在一篇当前访问者可读（有效状态）的文档时，空间才进入首页/分类列表。
	normalizedSpaceVisibilityExpr := "CASE WHEN " + qualifiedColumn("s", models.SpaceColumns.Visibility) + " IN ('public','authenticated','member') THEN " + qualifiedColumn("s", models.SpaceColumns.Visibility) + " ELSE 'member' END"
	normalizedDocumentVisibilityExpr := "CASE WHEN " + qualifiedColumn("d", models.DocumentColumns.Visibility) + " IN ('public','authenticated','member') THEN " + qualifiedColumn("d", models.DocumentColumns.Visibility) + " ELSE 'member' END"
	normalizedDocumentStatusExpr := "CASE WHEN " + qualifiedColumn("d", models.DocumentColumns.Status) + " IN ('active','banned','deleted') THEN " + qualifiedColumn("d", models.DocumentColumns.Status) + " ELSE 'active' END"

	// 统一首页空间可见性基础过滤，供 count/list 复用。
	// 这样 count 可以只保留必要 JOIN，避免和列表查询一样携带展示字段 JOIN。
	applyHomepageCommonFilters := func(query *gorm.DB) *gorm.DB {
		query = query.Where(qualifiedColumn("s", models.SpaceColumns.Status)+" = ?", models.EntityStatusActive)
		if categoryID != "" {
			query = query.Where(qualifiedColumn("s", models.SpaceColumns.CategoryID)+" = ?", categoryID)
		}
		if viewerUserID == "" {
			return query.Where(qualifiedColumn("s", models.SpaceColumns.Visibility)+" = ?", models.VisibilityPublic)
		}
		return query.
			Joins("LEFT JOIN "+tableName(models.SpaceMember{})+" AS sm ON sm."+models.SpaceMemberColumns.SpaceID+" = "+qualifiedColumn("s", models.SpaceColumns.SpaceID)+" AND sm."+models.SpaceMemberColumns.UserID+" = ?", viewerUserID).
			Where(
				"("+qualifiedColumn("s", models.SpaceColumns.Visibility)+" IN ? OR ("+qualifiedColumn("s", models.SpaceColumns.Visibility)+" = ? AND ("+qualifiedColumn("s", models.SpaceColumns.OwnerUserID)+" = ? OR sm.id IS NOT NULL)))",
				[]models.Visibility{models.VisibilityPublic, models.VisibilityAuthenticated},
				models.VisibilityMember,
				viewerUserID,
			)
	}

	buildDocumentVisibilityQuery := func() *gorm.DB {
		query := r.db.WithContext(ctx).
			Table(tableWithAlias(models.Document{}, "d")).
			Select("1").
			Joins("JOIN "+tableName(models.Node{})+" AS n ON "+qualifiedColumn("n", models.NodeColumns.NodeID)+" = "+qualifiedColumn("d", models.DocumentColumns.NodeID)).
			Where(qualifiedColumn("n", models.NodeColumns.SpaceID)+" = "+qualifiedColumn("s", models.SpaceColumns.SpaceID)).
			Where(normalizedDocumentStatusExpr+" = ?", models.EntityStatusActive)

		if viewerUserID == "" {
			return query.Where(
				normalizedSpaceVisibilityExpr+" = ? AND "+normalizedDocumentVisibilityExpr+" = ?",
				models.VisibilityPublic,
				models.VisibilityPublic,
			)
		}

		// 已登录场景复用外层 space_members 别名 `sm`，避免子查询重复 LEFT JOIN 成员表。
		// 可读条件化简为：
		// 1) 空间 owner；
		// 2) 空间成员（member 空间/文档均可读）；
		// 3) 非成员时仅允许 public/authenticated 组合。
		return query.Where(
			"("+
				qualifiedColumn("s", models.SpaceColumns.OwnerUserID)+" = ? OR "+
				"sm.id IS NOT NULL OR "+
				"(("+normalizedSpaceVisibilityExpr+" IN (?,?)) AND ("+normalizedDocumentVisibilityExpr+" IN (?,?)))"+
				")",
			viewerUserID,
			models.VisibilityPublic,
			models.VisibilityAuthenticated,
			models.VisibilityPublic,
			models.VisibilityAuthenticated,
		)
	}

	buildSpaceHasAnyDocumentQuery := func() *gorm.DB {
		// 空间无文档时也允许展示（点击后会进入“无可读文档”的友好提示），
		// 仅在存在文档时要求至少有一篇可读文档。
		return r.db.WithContext(ctx).
			Table(tableWithAlias(models.Document{}, "d_any")).
			Select("1").
			Joins("JOIN " + tableName(models.Node{}) + " AS n_any ON " + qualifiedColumn("n_any", models.NodeColumns.NodeID) + " = " + qualifiedColumn("d_any", models.DocumentColumns.NodeID)).
			Where(qualifiedColumn("n_any", models.NodeColumns.SpaceID) + " = " + qualifiedColumn("s", models.SpaceColumns.SpaceID)).
			Where(qualifiedColumn("d_any", models.DocumentColumns.DeletedAt) + " IS NULL")
	}

	var total int64
	countQuery := r.db.WithContext(ctx).Table(tableWithAlias(models.Space{}, "s"))
	countQuery = applyHomepageCommonFilters(countQuery)
	countQuery = countQuery.Where(
		"(NOT EXISTS (?) OR EXISTS (?))",
		buildSpaceHasAnyDocumentQuery(),
		buildDocumentVisibilityQuery(),
	)
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
		Table(tableWithAlias(models.Space{}, "s")).
		Joins("LEFT JOIN " + tableName(models.User{}) + " AS u ON u." + models.UserColumns.UserID + " = " + qualifiedColumn("s", models.SpaceColumns.OwnerUserID))
	listQuery = applyHomepageCommonFilters(listQuery)
	listQuery = listQuery.Where(
		"(NOT EXISTS (?) OR EXISTS (?))",
		buildSpaceHasAnyDocumentQuery(),
		buildDocumentVisibilityQuery(),
	)

	type homepageSpaceRow struct {
		ID           int64               `gorm:"column:id"`
		SpaceID      string              `gorm:"column:space_id"`
		Name         string              `gorm:"column:name"`
		Description  string              `gorm:"column:description"`
		CategoryID   string              `gorm:"column:category_id"`
		Category     string              `gorm:"column:category"`
		OwnerUserID  string              `gorm:"column:owner_user_id"`
		Visibility   models.Visibility   `gorm:"column:visibility"`
		CoverAssetID *string             `gorm:"column:cover_asset_id"`
		CoverKey     string              `gorm:"column:cover_key"`
		CoverURL     string              `gorm:"column:cover_url"`
		CoverWidth   int                 `gorm:"column:cover_width"`
		CoverHeight  int                 `gorm:"column:cover_height"`
		CoverSource  string              `gorm:"column:cover_source"`
		Status       models.EntityStatus `gorm:"column:status"`
		BannedReason string              `gorm:"column:banned_reason"`
		BannedAt     *time.Time          `gorm:"column:banned_at"`
		DeletedAt    *time.Time          `gorm:"column:deleted_at"`
		CreatedAtRaw string              `gorm:"column:created_at"`
		UpdatedAtRaw string              `gorm:"column:updated_at"`
		OwnerName    string              `gorm:"column:owner_name"`
		OwnerAvatar  string              `gorm:"column:owner_avatar_url"`
	}

	var rows []homepageSpaceRow
	if err := listQuery.Session(&gorm.Session{}).
		Select(selectColumns(
			qualifiedColumn("s", models.SpaceColumns.ID),
			qualifiedColumn("s", models.SpaceColumns.SpaceID),
			qualifiedColumn("s", models.SpaceColumns.Name),
			qualifiedColumn("s", models.SpaceColumns.Description),
			qualifiedColumn("s", models.SpaceColumns.CategoryID),
			qualifiedColumn("s", models.SpaceColumns.Category),
			qualifiedColumn("s", models.SpaceColumns.OwnerUserID),
			qualifiedColumn("s", models.SpaceColumns.Visibility),
			qualifiedColumn("s", models.SpaceColumns.CoverAssetID),
			qualifiedColumn("s", models.SpaceColumns.CoverKey),
			qualifiedColumn("s", models.SpaceColumns.CoverURL),
			qualifiedColumn("s", models.SpaceColumns.CoverWidth),
			qualifiedColumn("s", models.SpaceColumns.CoverHeight),
			qualifiedColumn("s", models.SpaceColumns.CoverSource),
			qualifiedColumn("s", models.SpaceColumns.Status),
			qualifiedColumn("s", models.SpaceColumns.BannedReason),
			qualifiedColumn("s", models.SpaceColumns.BannedAt),
			qualifiedColumn("s", models.SpaceColumns.DeletedAt),
			qualifiedColumn("s", models.SpaceColumns.CreatedAt),
			qualifiedColumn("s", models.SpaceColumns.UpdatedAt),
			qualifiedColumn("u", models.UserColumns.Name)+" AS owner_name",
			qualifiedColumn("u", models.UserColumns.AvatarURL)+" AS owner_avatar_url",
		)).
		Order(qualifiedColumn("s", models.SpaceColumns.CreatedAt) + " DESC, " + qualifiedColumn("s", models.SpaceColumns.ID) + " DESC").
		Offset(offset).
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	result := make([]HomepageVisibleSpaceRecord, 0, len(rows))
	for _, row := range rows {
		space := models.Space{
			ID:           row.ID,
			SpaceID:      row.SpaceID,
			Name:         row.Name,
			Description:  row.Description,
			CategoryID:   row.CategoryID,
			Category:     row.Category,
			OwnerUserID:  row.OwnerUserID,
			Visibility:   row.Visibility,
			CoverAssetID: row.CoverAssetID,
			CoverKey:     row.CoverKey,
			CoverURL:     row.CoverURL,
			CoverWidth:   row.CoverWidth,
			CoverHeight:  row.CoverHeight,
			CoverSource:  row.CoverSource,
			Status:       row.Status,
			BannedReason: row.BannedReason,
			BannedAt:     row.BannedAt,
			DeletedAt:    row.DeletedAt,
			CreatedAt:    parseSpaceRecordTime(row.CreatedAtRaw),
			UpdatedAt:    parseSpaceRecordTime(row.UpdatedAtRaw),
		}
		if !models.IsValidVisibility(space.Visibility) {
			space.Visibility = models.VisibilityMember
		}
		if !models.IsValidEntityStatus(space.Status) {
			space.Status = models.EntityStatusActive
		}
		space.Name = strings.TrimSpace(space.Name)
		space.Description = strings.TrimSpace(space.Description)
		space.CategoryID = strings.TrimSpace(space.CategoryID)
		space.Category = strings.TrimSpace(space.Category)
		space.CoverKey = strings.TrimSpace(space.CoverKey)
		space.CoverURL = strings.TrimSpace(space.CoverURL)
		space.CoverSource = strings.TrimSpace(space.CoverSource)
		if space.CoverWidth < 0 {
			space.CoverWidth = 0
		}
		if space.CoverHeight < 0 {
			space.CoverHeight = 0
		}
		if space.CoverAssetID != nil {
			trimmedAssetID := strings.TrimSpace(*space.CoverAssetID)
			if trimmedAssetID == "" {
				space.CoverAssetID = nil
			} else {
				space.CoverAssetID = &trimmedAssetID
			}
		}
		result = append(result, HomepageVisibleSpaceRecord{
			Space:          space,
			OwnerName:      strings.TrimSpace(row.OwnerName),
			OwnerAvatarURL: strings.TrimSpace(row.OwnerAvatar),
		})
	}

	return result, total, nil
}

func (r *gormSpaceRepository) ListForAdmin(
	ctx context.Context,
	params ListAdminSpacesParams,
) ([]AdminSpaceListRecord, int64, error) {
	if r == nil || r.db == nil {
		return nil, 0, fmt.Errorf("space repository db is nil")
	}

	baseQuery := r.db.WithContext(ctx).
		Table(tableWithAlias(models.Space{}, "s")).
		Joins("JOIN " + tableName(models.User{}) + " AS u ON u." + models.UserColumns.UserID + " = " + qualifiedColumn("s", models.SpaceColumns.OwnerUserID)).
		Joins("LEFT JOIN " + tableName(models.SpaceCategory{}) + " AS sc ON sc." + models.SpaceCategoryColumns.CategoryID + " = " + qualifiedColumn("s", models.SpaceColumns.CategoryID))

	switch {
	case params.RestrictToMembers:
		actorUserID := strings.TrimSpace(params.ActorUserID)
		baseQuery = baseQuery.Joins("LEFT JOIN "+tableName(models.SpaceMember{})+" AS sm ON sm."+models.SpaceMemberColumns.SpaceID+" = "+qualifiedColumn("s", models.SpaceColumns.SpaceID)+" AND sm."+models.SpaceMemberColumns.UserID+" = ?", actorUserID)
		baseQuery = baseQuery.Where("("+qualifiedColumn("s", models.SpaceColumns.OwnerUserID)+" = ? OR sm.id IS NOT NULL)", actorUserID)
	case params.RestrictToScopes:
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
			"LOWER("+qualifiedColumn("s", models.SpaceColumns.SpaceID)+") LIKE ? OR LOWER("+qualifiedColumn("s", models.SpaceColumns.Name)+") LIKE ? OR LOWER("+qualifiedColumn("s", models.SpaceColumns.Category)+") LIKE ? OR LOWER("+qualifiedColumn("sc", models.SpaceCategoryColumns.Name)+") LIKE ? OR LOWER("+qualifiedColumn("u", models.UserColumns.UserID)+") LIKE ? OR LOWER("+qualifiedColumn("u", models.UserColumns.Email)+") LIKE ? OR LOWER("+qualifiedColumn("u", models.UserColumns.Name)+") LIKE ?",
			likeKeyword,
			likeKeyword,
			likeKeyword,
			likeKeyword,
			likeKeyword,
			likeKeyword,
			likeKeyword,
		)
	}

	statuses := normalizeSpaceStatuses(params.Statuses)
	if len(statuses) > 0 {
		baseQuery = baseQuery.Where(qualifiedColumn("s", models.SpaceColumns.Status)+" IN ?", statuses)
	}

	visibilities := normalizeSpaceVisibilities(params.Visibilities)
	if len(visibilities) > 0 {
		baseQuery = baseQuery.Where(qualifiedColumn("s", models.SpaceColumns.Visibility)+" IN ?", visibilities)
	}

	var total int64
	if err := baseQuery.Session(&gorm.Session{}).
		Count(&total).Error; err != nil {
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

	type adminSpaceListRow struct {
		ID           int64               `gorm:"column:id"`
		SpaceID      string              `gorm:"column:space_id"`
		Name         string              `gorm:"column:name"`
		Description  string              `gorm:"column:description"`
		CategoryID   string              `gorm:"column:category_id"`
		Category     string              `gorm:"column:category"`
		CategoryName string              `gorm:"column:category_name"`
		CategoryDef  bool                `gorm:"column:category_is_default"`
		OwnerUserID  string              `gorm:"column:owner_user_id"`
		Visibility   models.Visibility   `gorm:"column:visibility"`
		CoverAssetID *string             `gorm:"column:cover_asset_id"`
		CoverKey     string              `gorm:"column:cover_key"`
		CoverURL     string              `gorm:"column:cover_url"`
		CoverWidth   int                 `gorm:"column:cover_width"`
		CoverHeight  int                 `gorm:"column:cover_height"`
		CoverSource  string              `gorm:"column:cover_source"`
		Status       models.EntityStatus `gorm:"column:status"`
		BannedReason string              `gorm:"column:banned_reason"`
		BannedAt     *time.Time          `gorm:"column:banned_at"`
		DeletedAt    *time.Time          `gorm:"column:deleted_at"`
		CreatedAtRaw string              `gorm:"column:created_at"`
		UpdatedAtRaw string              `gorm:"column:updated_at"`
		OwnerName    string              `gorm:"column:owner_name"`
		OwnerEmail   string              `gorm:"column:owner_email"`
	}

	var rows []adminSpaceListRow
	if err := baseQuery.Session(&gorm.Session{}).
		Select(
			selectColumns(
				qualifiedColumn("s", models.SpaceColumns.ID),
				qualifiedColumn("s", models.SpaceColumns.SpaceID),
				qualifiedColumn("s", models.SpaceColumns.Name),
				qualifiedColumn("s", models.SpaceColumns.Description),
				qualifiedColumn("s", models.SpaceColumns.CategoryID),
				qualifiedColumn("s", models.SpaceColumns.Category),
				"COALESCE("+qualifiedColumn("sc", models.SpaceCategoryColumns.Name)+", "+qualifiedColumn("s", models.SpaceColumns.Category)+") AS category_name",
				"COALESCE("+qualifiedColumn("sc", models.SpaceCategoryColumns.IsDefault)+", FALSE) AS category_is_default",
				qualifiedColumn("s", models.SpaceColumns.OwnerUserID),
				qualifiedColumn("s", models.SpaceColumns.Visibility),
				qualifiedColumn("s", models.SpaceColumns.CoverAssetID),
				qualifiedColumn("s", models.SpaceColumns.CoverKey),
				qualifiedColumn("s", models.SpaceColumns.CoverURL),
				qualifiedColumn("s", models.SpaceColumns.CoverWidth),
				qualifiedColumn("s", models.SpaceColumns.CoverHeight),
				qualifiedColumn("s", models.SpaceColumns.CoverSource),
				qualifiedColumn("s", models.SpaceColumns.Status),
				qualifiedColumn("s", models.SpaceColumns.BannedReason),
				qualifiedColumn("s", models.SpaceColumns.BannedAt),
				qualifiedColumn("s", models.SpaceColumns.DeletedAt),
				qualifiedColumn("s", models.SpaceColumns.CreatedAt),
				qualifiedColumn("s", models.SpaceColumns.UpdatedAt),
				qualifiedColumn("u", models.UserColumns.Name)+" AS owner_name",
				qualifiedColumn("u", models.UserColumns.Email)+" AS owner_email",
			),
		).
		Order(qualifiedColumn("s", models.SpaceColumns.CreatedAt) + " DESC").
		Offset(offset).
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	result := make([]AdminSpaceListRecord, 0, len(rows))
	for _, row := range rows {
		space := models.Space{
			ID:           row.ID,
			SpaceID:      row.SpaceID,
			Name:         row.Name,
			Description:  row.Description,
			CategoryID:   row.CategoryID,
			Category:     row.Category,
			OwnerUserID:  row.OwnerUserID,
			Visibility:   row.Visibility,
			CoverAssetID: row.CoverAssetID,
			CoverKey:     row.CoverKey,
			CoverURL:     row.CoverURL,
			CoverWidth:   row.CoverWidth,
			CoverHeight:  row.CoverHeight,
			CoverSource:  row.CoverSource,
			Status:       row.Status,
			BannedReason: row.BannedReason,
			BannedAt:     row.BannedAt,
			DeletedAt:    row.DeletedAt,
			CreatedAt:    parseSpaceRecordTime(row.CreatedAtRaw),
			UpdatedAt:    parseSpaceRecordTime(row.UpdatedAtRaw),
		}
		if !models.IsValidVisibility(space.Visibility) {
			space.Visibility = models.VisibilityMember
		}
		if !models.IsValidEntityStatus(space.Status) {
			space.Status = models.EntityStatusActive
		}
		space.Name = strings.TrimSpace(space.Name)
		space.Description = strings.TrimSpace(space.Description)
		space.CategoryID = strings.TrimSpace(space.CategoryID)
		space.Category = strings.TrimSpace(row.CategoryName)
		if space.Category == "" {
			space.Category = strings.TrimSpace(row.Category)
		}
		space.CoverKey = strings.TrimSpace(space.CoverKey)
		space.CoverURL = strings.TrimSpace(space.CoverURL)
		space.CoverSource = strings.TrimSpace(space.CoverSource)
		if space.CoverWidth < 0 {
			space.CoverWidth = 0
		}
		if space.CoverHeight < 0 {
			space.CoverHeight = 0
		}
		if space.CoverAssetID != nil {
			trimmedAssetID := strings.TrimSpace(*space.CoverAssetID)
			if trimmedAssetID == "" {
				space.CoverAssetID = nil
			} else {
				space.CoverAssetID = &trimmedAssetID
			}
		}
		result = append(result, AdminSpaceListRecord{
			Space:         space,
			CategoryID:    space.CategoryID,
			CategoryName:  space.Category,
			CategoryIsDef: row.CategoryDef,
			OwnerName:     row.OwnerName,
			OwnerEmail:    row.OwnerEmail,
		})
	}

	return result, total, nil
}

func (r *gormSpaceRepository) ListMembers(ctx context.Context, spaceID string) ([]SpaceMemberListRecord, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("space repository db is nil")
	}
	normalizedSpaceID := strings.TrimSpace(spaceID)
	if normalizedSpaceID == "" {
		return []SpaceMemberListRecord{}, nil
	}

	type spaceMemberRow struct {
		UserID       string      `gorm:"column:user_id"`
		Email        string      `gorm:"column:email"`
		Name         string      `gorm:"column:name"`
		Role         models.Role `gorm:"column:role"`
		CreatedAtRaw string      `gorm:"column:created_at"`
		UpdatedAtRaw string      `gorm:"column:updated_at"`
	}

	var rows []spaceMemberRow
	if err := r.db.WithContext(ctx).
		Table(tableWithAlias(models.SpaceMember{}, "sm")).
		Select(selectColumns(
			qualifiedColumn("sm", models.SpaceMemberColumns.UserID),
			qualifiedColumn("sm", models.SpaceMemberColumns.Role),
			qualifiedColumn("sm", models.SpaceMemberColumns.CreatedAt),
			qualifiedColumn("sm", models.SpaceMemberColumns.UpdatedAt),
			qualifiedColumn("u", models.UserColumns.Email),
			qualifiedColumn("u", models.UserColumns.Name),
		)).
		Joins("JOIN "+tableName(models.User{})+" AS u ON u."+models.UserColumns.UserID+" = "+qualifiedColumn("sm", models.SpaceMemberColumns.UserID)).
		Where(qualifiedColumn("sm", models.SpaceMemberColumns.SpaceID)+" = ?", normalizedSpaceID).
		Order(qualifiedColumn("sm", models.SpaceMemberColumns.CreatedAt) + " ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]SpaceMemberListRecord, 0, len(rows))
	for _, row := range rows {
		result = append(result, SpaceMemberListRecord{
			UserID:    strings.TrimSpace(row.UserID),
			Email:     strings.TrimSpace(row.Email),
			Name:      strings.TrimSpace(row.Name),
			Role:      normalizeSpaceMemberRole(row.Role),
			CreatedAt: parseSpaceRecordTime(row.CreatedAtRaw),
			UpdatedAt: parseSpaceRecordTime(row.UpdatedAtRaw),
		})
	}
	return result, nil
}

func (r *gormSpaceRepository) UpsertMember(ctx context.Context, params UpsertSpaceMemberParams) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("space repository db is nil")
	}

	spaceID := strings.TrimSpace(params.SpaceID)
	userID := strings.TrimSpace(params.UserID)
	if spaceID == "" || userID == "" {
		return nil
	}

	role := normalizeSpaceMemberRole(params.Role)
	if role == "" {
		return nil
	}

	updatedAt := params.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}

	return r.db.WithContext(ctx).
		Model(&models.SpaceMember{}).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: models.SpaceMemberColumns.SpaceID}, {Name: models.SpaceMemberColumns.UserID}},
			DoUpdates: clause.Assignments(map[string]any{
				models.SpaceMemberColumns.Role:      role,
				models.SpaceMemberColumns.UpdatedAt: updatedAt,
			}),
		}).
		Create(map[string]any{
			models.SpaceMemberColumns.SpaceID:   spaceID,
			models.SpaceMemberColumns.UserID:    userID,
			models.SpaceMemberColumns.Role:      role,
			models.SpaceMemberColumns.CreatedAt: updatedAt,
			models.SpaceMemberColumns.UpdatedAt: updatedAt,
		}).Error
}

func (r *gormSpaceRepository) UpdateMemberRole(ctx context.Context, params UpdateSpaceMemberRoleParams) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("space repository db is nil")
	}

	spaceID := strings.TrimSpace(params.SpaceID)
	userID := strings.TrimSpace(params.UserID)
	if spaceID == "" || userID == "" {
		return false, nil
	}

	role := normalizeSpaceMemberRole(params.Role)
	if role == "" {
		return false, nil
	}

	updatedAt := params.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}

	tx := r.db.WithContext(ctx).
		Model(&models.SpaceMember{}).
		Where(qualifiedColumn("", models.SpaceMemberColumns.SpaceID)+" = ?", spaceID).
		Where(qualifiedColumn("", models.SpaceMemberColumns.UserID)+" = ?", userID).
		Updates(map[string]any{
			models.SpaceMemberColumns.Role:      role,
			models.SpaceMemberColumns.UpdatedAt: updatedAt,
		})
	if tx.Error != nil {
		return false, tx.Error
	}
	return tx.RowsAffected > 0, nil
}

func (r *gormSpaceRepository) DeleteMember(ctx context.Context, spaceID string, userID string) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("space repository db is nil")
	}

	normalizedSpaceID := strings.TrimSpace(spaceID)
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedSpaceID == "" || normalizedUserID == "" {
		return false, nil
	}

	tx := r.db.WithContext(ctx).
		Where(qualifiedColumn("", models.SpaceMemberColumns.SpaceID)+" = ?", normalizedSpaceID).
		Where(qualifiedColumn("", models.SpaceMemberColumns.UserID)+" = ?", normalizedUserID).
		Delete(&models.SpaceMember{})
	if tx.Error != nil {
		return false, tx.Error
	}
	return tx.RowsAffected > 0, nil
}

func (r *gormSpaceRepository) UpdateVisibility(
	ctx context.Context,
	spaceID string,
	visibility models.Visibility,
) (*models.Space, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("space repository db is nil")
	}

	var updatedSpace *models.Space
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updateTx := tx.Model(&models.Space{}).
			Where(qualifiedColumn("", models.SpaceColumns.SpaceID)+" = ?", spaceID).
			Updates(map[string]any{
				models.SpaceColumns.Visibility: visibility,
				models.SpaceColumns.UpdatedAt:  gorm.Expr("CURRENT_TIMESTAMP"),
			})
		if updateTx.Error != nil {
			return updateTx.Error
		}
		if updateTx.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		if err := r.enqueueSpaceRebuildInTx(ctx, tx, spaceID); err != nil {
			return err
		}
		space, err := r.getBySpaceIDWithTx(ctx, tx, spaceID)
		if err != nil {
			return err
		}
		updatedSpace = space
		return nil
	})
	if err != nil {
		return nil, err
	}
	if updatedSpace == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return updatedSpace, nil
}

func (r *gormSpaceRepository) CreateCoverAsset(ctx context.Context, asset *models.SpaceCoverAsset) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("space repository db is nil")
	}
	if asset != nil {
		asset.AssetID = strings.TrimSpace(asset.AssetID)
		asset.Source = strings.TrimSpace(asset.Source)
		asset.ObjectKey = strings.TrimSpace(asset.ObjectKey)
		asset.ObjectURL = strings.TrimSpace(asset.ObjectURL)
		asset.MimeType = strings.TrimSpace(asset.MimeType)
		asset.CreatedByUserID = strings.TrimSpace(asset.CreatedByUserID)
		if asset.Width < 0 {
			asset.Width = 0
		}
		if asset.Height < 0 {
			asset.Height = 0
		}
		if asset.SizeBytes < 0 {
			asset.SizeBytes = 0
		}
	}
	return r.db.WithContext(ctx).Create(asset).Error
}

func (r *gormSpaceRepository) UpdateStatus(ctx context.Context, params UpdateSpaceStatusParams) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("space repository db is nil")
	}
	if strings.TrimSpace(params.SpaceID) == "" {
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
		models.SpaceColumns.Status:       params.Status,
		models.SpaceColumns.UpdatedAt:    updatedAt,
		models.SpaceColumns.BannedReason: "",
		models.SpaceColumns.BannedAt:     nil,
	}
	if params.Status == models.EntityStatusBanned {
		updateValues[models.SpaceColumns.BannedReason] = strings.TrimSpace(params.BannedReason)
		updateValues[models.SpaceColumns.BannedAt] = params.BannedAt
	}

	updated := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updateTx := tx.Model(&models.Space{}).
			Where(qualifiedColumn("", models.SpaceColumns.SpaceID)+" = ?", params.SpaceID).
			Where(qualifiedColumn("", models.SpaceColumns.Status)+" <> ?", models.EntityStatusDeleted).
			Updates(updateValues)
		if updateTx.Error != nil {
			return updateTx.Error
		}
		if updateTx.RowsAffected == 0 {
			return nil
		}
		updated = true

		if params.Status == models.EntityStatusBanned || params.Status == models.EntityStatusDeleted {
			return r.enqueueSpacePurgeInTx(ctx, tx, params.SpaceID)
		}
		return r.enqueueSpaceRebuildInTx(ctx, tx, params.SpaceID)
	})
	if err != nil {
		return false, err
	}
	return updated, nil
}

func (r *gormSpaceRepository) UpdateMetadata(ctx context.Context, params UpdateSpaceMetadataParams) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("space repository db is nil")
	}
	if strings.TrimSpace(params.SpaceID) == "" {
		return false, nil
	}

	updatedAt := params.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}

	updateValues := map[string]any{
		models.SpaceColumns.UpdatedAt: updatedAt,
	}
	if params.Name != nil {
		updateValues[models.SpaceColumns.Name] = strings.TrimSpace(*params.Name)
	}
	if params.Description != nil {
		updateValues[models.SpaceColumns.Description] = strings.TrimSpace(*params.Description)
	}
	if params.CategoryID != nil {
		updateValues[models.SpaceColumns.CategoryID] = strings.TrimSpace(*params.CategoryID)
	}
	if params.Category != nil {
		updateValues[models.SpaceColumns.Category] = strings.TrimSpace(*params.Category)
	}
	if params.Visibility != nil {
		if !models.IsValidVisibility(*params.Visibility) {
			return false, nil
		}
		updateValues[models.SpaceColumns.Visibility] = *params.Visibility
	}
	if params.CoverAssetID != nil {
		// 关键分支：封面更新允许清空（传空字符串时转为 NULL）。
		trimmed := strings.TrimSpace(*params.CoverAssetID)
		if trimmed == "" {
			updateValues[models.SpaceColumns.CoverAssetID] = nil
		} else {
			updateValues[models.SpaceColumns.CoverAssetID] = trimmed
		}
	}
	if params.CoverKey != nil {
		updateValues[models.SpaceColumns.CoverKey] = strings.TrimSpace(*params.CoverKey)
	}
	if params.CoverURL != nil {
		updateValues[models.SpaceColumns.CoverURL] = strings.TrimSpace(*params.CoverURL)
	}
	if params.CoverSource != nil {
		updateValues[models.SpaceColumns.CoverSource] = strings.TrimSpace(*params.CoverSource)
	}
	if params.CoverWidth != nil {
		updateValues[models.SpaceColumns.CoverWidth] = *params.CoverWidth
	}
	if params.CoverHeight != nil {
		updateValues[models.SpaceColumns.CoverHeight] = *params.CoverHeight
	}

	updated := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updateTx := tx.Model(&models.Space{}).
			Where(qualifiedColumn("", models.SpaceColumns.SpaceID)+" = ?", params.SpaceID).
			Where(qualifiedColumn("", models.SpaceColumns.Status)+" <> ?", models.EntityStatusDeleted).
			Updates(updateValues)
		if updateTx.Error != nil {
			return updateTx.Error
		}
		if updateTx.RowsAffected == 0 {
			return nil
		}
		updated = true
		if params.Visibility != nil {
			return r.enqueueSpaceRebuildInTx(ctx, tx, params.SpaceID)
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return updated, nil
}

func (r *gormSpaceRepository) getBySpaceIDWithTx(
	ctx context.Context,
	tx *gorm.DB,
	spaceID string,
) (*models.Space, error) {
	if tx == nil {
		return nil, gorm.ErrRecordNotFound
	}

	type spaceRecordRow struct {
		ID           int64               `gorm:"column:id"`
		SpaceID      string              `gorm:"column:space_id"`
		Name         string              `gorm:"column:name"`
		Description  string              `gorm:"column:description"`
		CategoryID   string              `gorm:"column:category_id"`
		Category     string              `gorm:"column:category"`
		OwnerUserID  string              `gorm:"column:owner_user_id"`
		Visibility   models.Visibility   `gorm:"column:visibility"`
		CoverAssetID *string             `gorm:"column:cover_asset_id"`
		CoverKey     string              `gorm:"column:cover_key"`
		CoverURL     string              `gorm:"column:cover_url"`
		CoverWidth   int                 `gorm:"column:cover_width"`
		CoverHeight  int                 `gorm:"column:cover_height"`
		CoverSource  string              `gorm:"column:cover_source"`
		Status       models.EntityStatus `gorm:"column:status"`
		BannedReason string              `gorm:"column:banned_reason"`
		BannedAt     *time.Time          `gorm:"column:banned_at"`
		DeletedAt    *time.Time          `gorm:"column:deleted_at"`
		CreatedAtRaw string              `gorm:"column:created_at"`
		UpdatedAtRaw string              `gorm:"column:updated_at"`
	}

	var row spaceRecordRow
	if err := tx.WithContext(ctx).
		Model(&models.Space{}).
		Select(selectColumns(
			qualifiedColumn("", models.SpaceColumns.ID),
			qualifiedColumn("", models.SpaceColumns.SpaceID),
			qualifiedColumn("", models.SpaceColumns.Name),
			qualifiedColumn("", models.SpaceColumns.Description),
			qualifiedColumn("", models.SpaceColumns.CategoryID),
			qualifiedColumn("", models.SpaceColumns.Category),
			qualifiedColumn("", models.SpaceColumns.OwnerUserID),
			qualifiedColumn("", models.SpaceColumns.Visibility),
			qualifiedColumn("", models.SpaceColumns.CoverAssetID),
			qualifiedColumn("", models.SpaceColumns.CoverKey),
			qualifiedColumn("", models.SpaceColumns.CoverURL),
			qualifiedColumn("", models.SpaceColumns.CoverWidth),
			qualifiedColumn("", models.SpaceColumns.CoverHeight),
			qualifiedColumn("", models.SpaceColumns.CoverSource),
			qualifiedColumn("", models.SpaceColumns.Status),
			qualifiedColumn("", models.SpaceColumns.BannedReason),
			qualifiedColumn("", models.SpaceColumns.BannedAt),
			qualifiedColumn("", models.SpaceColumns.DeletedAt),
			qualifiedColumn("", models.SpaceColumns.CreatedAt),
			qualifiedColumn("", models.SpaceColumns.UpdatedAt),
		)).
		Where(qualifiedColumn("", models.SpaceColumns.SpaceID)+" = ?", strings.TrimSpace(spaceID)).
		Take(&row).Error; err != nil {
		return nil, err
	}

	space := models.Space{
		ID:           row.ID,
		SpaceID:      strings.TrimSpace(row.SpaceID),
		Name:         strings.TrimSpace(row.Name),
		Description:  strings.TrimSpace(row.Description),
		CategoryID:   strings.TrimSpace(row.CategoryID),
		Category:     strings.TrimSpace(row.Category),
		OwnerUserID:  strings.TrimSpace(row.OwnerUserID),
		Visibility:   row.Visibility,
		CoverAssetID: row.CoverAssetID,
		CoverKey:     strings.TrimSpace(row.CoverKey),
		CoverURL:     strings.TrimSpace(row.CoverURL),
		CoverWidth:   row.CoverWidth,
		CoverHeight:  row.CoverHeight,
		CoverSource:  strings.TrimSpace(row.CoverSource),
		Status:       row.Status,
		BannedReason: strings.TrimSpace(row.BannedReason),
		BannedAt:     row.BannedAt,
		DeletedAt:    row.DeletedAt,
		CreatedAt:    parseSpaceRecordTime(row.CreatedAtRaw),
		UpdatedAt:    parseSpaceRecordTime(row.UpdatedAtRaw),
	}
	if !models.IsValidVisibility(space.Visibility) {
		space.Visibility = models.VisibilityMember
	}
	if !models.IsValidEntityStatus(space.Status) {
		space.Status = models.EntityStatusActive
	}
	return &space, nil
}

func (r *gormSpaceRepository) enqueueSpaceRebuildInTx(
	ctx context.Context,
	tx *gorm.DB,
	spaceID string,
) error {
	if r == nil || r.searchIndexJobRepo == nil {
		return nil
	}
	params, err := BuildSearchIndexRebuildSpaceJob(spaceID)
	if err != nil {
		return err
	}
	return r.searchIndexJobRepo.EnqueueInTx(ctx, tx, params)
}

func (r *gormSpaceRepository) enqueueSpacePurgeInTx(
	ctx context.Context,
	tx *gorm.DB,
	spaceID string,
) error {
	if r == nil || r.searchIndexJobRepo == nil {
		return nil
	}
	params, err := BuildSearchIndexSpacePurgeJob(spaceID)
	if err != nil {
		return err
	}
	return r.searchIndexJobRepo.EnqueueInTx(ctx, tx, params)
}

func (r *gormSpaceRepository) IsMember(ctx context.Context, spaceID string, userID string) (bool, error) {
	// 关键函数：校验用户是否为指定空间成员（用于转让前置校验）。
	if r == nil || r.db == nil {
		return false, fmt.Errorf("space repository db is nil")
	}
	if strings.TrimSpace(spaceID) == "" || strings.TrimSpace(userID) == "" {
		return false, nil
	}

	var count int64
	if err := r.db.WithContext(ctx).
		Model(&models.SpaceMember{}).
		Where(qualifiedColumn("", models.SpaceMemberColumns.SpaceID)+" = ?", spaceID).
		Where(qualifiedColumn("", models.SpaceMemberColumns.UserID)+" = ?", userID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *gormSpaceRepository) TransferOwnership(
	ctx context.Context,
	spaceID string,
	fromUserID string,
	toUserID string,
	updatedAt time.Time,
) (bool, error) {
	// 关键函数：在同一事务内更新 owner 与成员角色，避免状态不一致。
	if r == nil || r.db == nil {
		return false, fmt.Errorf("space repository db is nil")
	}
	spaceID = strings.TrimSpace(spaceID)
	fromUserID = strings.TrimSpace(fromUserID)
	toUserID = strings.TrimSpace(toUserID)
	if spaceID == "" || fromUserID == "" || toUserID == "" {
		return false, nil
	}
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updateTx := tx.Model(&models.Space{}).
			Where(qualifiedColumn("", models.SpaceColumns.SpaceID)+" = ?", spaceID).
			Where(qualifiedColumn("", models.SpaceColumns.OwnerUserID)+" = ?", fromUserID).
			Where(qualifiedColumn("", models.SpaceColumns.Status)+" <> ?", models.EntityStatusDeleted).
			Updates(map[string]any{
				models.SpaceColumns.OwnerUserID: toUserID,
				models.SpaceColumns.UpdatedAt:   updatedAt,
			})
		if updateTx.Error != nil {
			return updateTx.Error
		}
		if updateTx.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		memberUpsert := func(userID string, role models.Role) error {
			now := updatedAt
			return tx.Model(&models.SpaceMember{}).Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: models.SpaceMemberColumns.SpaceID}, {Name: models.SpaceMemberColumns.UserID}},
				DoUpdates: clause.Assignments(map[string]any{
					models.SpaceMemberColumns.Role:      role,
					models.SpaceMemberColumns.UpdatedAt: now,
				}),
			}).Create(map[string]any{
				models.SpaceMemberColumns.SpaceID:   spaceID,
				models.SpaceMemberColumns.UserID:    userID,
				models.SpaceMemberColumns.Role:      role,
				models.SpaceMemberColumns.CreatedAt: now,
				models.SpaceMemberColumns.UpdatedAt: now,
			}).Error
		}

		if err := memberUpsert(fromUserID, models.RoleCollaborator); err != nil {
			return err
		}
		if err := memberUpsert(toUserID, models.RoleOwner); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *gormSpaceRepository) SoftDelete(ctx context.Context, spaceID string, deletedAt time.Time) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("space repository db is nil")
	}
	if strings.TrimSpace(spaceID) == "" {
		return false, nil
	}
	if deletedAt.IsZero() {
		deletedAt = time.Now().UTC()
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		spaceTx := tx.Model(&models.Space{}).
			Where(qualifiedColumn("", models.SpaceColumns.SpaceID)+" = ?", spaceID).
			Where(qualifiedColumn("", models.SpaceColumns.Status)+" <> ?", models.EntityStatusDeleted).
			Updates(map[string]any{
				models.SpaceColumns.Status:       models.EntityStatusDeleted,
				models.SpaceColumns.DeletedAt:    deletedAt,
				models.SpaceColumns.BannedReason: "",
				models.SpaceColumns.BannedAt:     nil,
				models.SpaceColumns.UpdatedAt:    deletedAt,
			})
		if spaceTx.Error != nil {
			return spaceTx.Error
		}
		if spaceTx.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		documentsQuery := tx.Model(&models.Node{}).
			Select(qualifiedColumn("", models.NodeColumns.NodeID)).
			Where(qualifiedColumn("", models.NodeColumns.SpaceID)+" = ?", spaceID)

		documentTx := tx.Model(&models.Document{}).
			Where(qualifiedColumn("", models.DocumentColumns.NodeID)+" IN ?", documentsQuery).
			Where(qualifiedColumn("", models.DocumentColumns.Status)+" <> ?", models.EntityStatusDeleted).
			Updates(map[string]any{
				models.DocumentColumns.Status:       models.EntityStatusDeleted,
				models.DocumentColumns.DeletedAt:    deletedAt,
				models.DocumentColumns.BannedReason: "",
				models.DocumentColumns.BannedAt:     nil,
				models.DocumentColumns.UpdatedAt:    deletedAt,
			})
		if documentTx.Error != nil {
			return documentTx.Error
		}

		return r.enqueueSpacePurgeInTx(ctx, tx, spaceID)
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *gormSpaceRepository) HardDelete(ctx context.Context, spaceID string) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("space repository db is nil")
	}
	normalizedSpaceID := strings.TrimSpace(spaceID)
	if normalizedSpaceID == "" {
		return false, nil
	}

	var spaceDeleted bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		nodeIDsQuery := tx.Model(&models.Node{}).
			Select(qualifiedColumn("", models.NodeColumns.NodeID)).
			Where(qualifiedColumn("", models.NodeColumns.SpaceID)+" = ?", normalizedSpaceID)
		type documentIDRow struct {
			DocumentID string `gorm:"column:document_id"`
		}
		documentIDRows := make([]documentIDRow, 0, 8)
		if err := tx.Table(tableWithAlias(models.Document{}, "d")).
			Select(qualifiedColumn("d", models.DocumentColumns.DocumentID)+" AS document_id").
			Joins("JOIN "+tableName(models.Node{})+" AS n ON "+qualifiedColumn("n", models.NodeColumns.NodeID)+" = "+qualifiedColumn("d", models.DocumentColumns.NodeID)).
			Where(qualifiedColumn("n", models.NodeColumns.SpaceID)+" = ?", normalizedSpaceID).
			Scan(&documentIDRows).Error; err != nil {
			return err
		}

		documentIDs := make([]string, 0, len(documentIDRows))
		for _, row := range documentIDRows {
			documentID := strings.TrimSpace(row.DocumentID)
			if documentID == "" {
				continue
			}
			documentIDs = append(documentIDs, documentID)
		}

		if _, err := DeleteDocumentsCascadeInTx(tx, uniqueNonEmptyStrings(documentIDs)); err != nil {
			return err
		}
		if err := tx.Model(&models.NodePermission{}).
			Where(qualifiedColumn("", models.NodeColumns.NodeID)+" IN (?)", nodeIDsQuery).
			Delete(nil).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.Node{}).
			Where(qualifiedColumn("", models.NodeColumns.SpaceID)+" = ?", normalizedSpaceID).
			Delete(nil).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.SpaceMember{}).
			Where(qualifiedColumn("", models.SpaceMemberColumns.SpaceID)+" = ?", normalizedSpaceID).
			Delete(nil).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.SpaceAdminScope{}).
			Where(qualifiedColumn("", models.SpaceAdminScopeColumns.SpaceID)+" = ?", normalizedSpaceID).
			Delete(nil).Error; err != nil {
			return err
		}

		deleteSpaceTx := tx.Where(qualifiedColumn("", models.SpaceColumns.SpaceID)+" = ?", normalizedSpaceID).Delete(&models.Space{})
		if deleteSpaceTx.Error != nil {
			return deleteSpaceTx.Error
		}
		spaceDeleted = deleteSpaceTx.RowsAffected > 0
		if !spaceDeleted {
			return nil
		}
		return r.enqueueSpacePurgeInTx(ctx, tx, normalizedSpaceID)
	})
	if err != nil {
		return false, err
	}
	return spaceDeleted, nil
}

func (r *gormSpaceRepository) HasReaderAccess(ctx context.Context, spaceID string, userID string) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("space repository db is nil")
	}
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return false, nil
	}

	// 读取权限：owner/member 以及管理员（平台管理员、具备空间管理范围）均可访问。
	// 使用 LIMIT 1 探测存在性，避免 COUNT(*) 在大表场景的额外扫描成本。
	var probe struct {
		Hit int `gorm:"column:hit"`
	}
	if err := r.db.WithContext(ctx).
		Table(tableWithAlias(models.Space{}, "s")).
		Select("1 AS hit").
		Where(qualifiedColumn("s", models.SpaceColumns.SpaceID)+" = ?", spaceID).
		Where(qualifiedColumn("s", models.SpaceColumns.Status)+" = ?", models.EntityStatusActive).
		Where(qualifiedColumn("s", models.SpaceColumns.DeletedAt)+" IS NULL").
		Where(
			"("+
				qualifiedColumn("s", models.SpaceColumns.OwnerUserID)+" = ? OR "+
				"EXISTS (?) OR "+
				"EXISTS (?) OR "+
				"EXISTS (?)"+
				")",
			normalizedUserID,
			r.db.WithContext(ctx).
				Model(&models.UserAdminRole{}).
				Select("1").
				Where(qualifiedColumn("", models.UserAdminRoleColumns.UserID)+" = ?", normalizedUserID).
				Where(qualifiedColumn("", models.UserAdminRoleColumns.Role)+" = ?", models.AdminRolePlatformAdmin),
			r.db.WithContext(ctx).
				Model(&models.SpaceAdminScope{}).
				Select("1").
				Where(qualifiedColumn("", models.SpaceAdminScopeColumns.SpaceID)+" = "+qualifiedColumn("s", models.SpaceColumns.SpaceID)).
				Where(qualifiedColumn("", models.SpaceAdminScopeColumns.UserID)+" = ?", normalizedUserID),
			r.db.WithContext(ctx).
				Model(&models.SpaceMember{}).
				Select("1").
				Where(qualifiedColumn("", models.SpaceMemberColumns.SpaceID)+" = "+qualifiedColumn("s", models.SpaceColumns.SpaceID)).
				Where(qualifiedColumn("", models.SpaceMemberColumns.UserID)+" = ?", normalizedUserID).
				Where(qualifiedColumn("", models.SpaceMemberColumns.Role)+" IN ?", []models.Role{models.RoleOwner, models.RoleCollaborator, models.RoleReader}),
		).
		Limit(1).
		Take(&probe).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (r *gormSpaceRepository) HasWriterAccess(ctx context.Context, spaceID string, userID string) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("space repository db is nil")
	}
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return false, nil
	}

	// 写入权限：owner/collaborator 以及管理员（平台管理员、具备空间管理范围）可写；
	// reader 仍不具备写能力。
	var probe struct {
		Hit int `gorm:"column:hit"`
	}
	if err := r.db.WithContext(ctx).
		Table(tableWithAlias(models.Space{}, "s")).
		Select("1 AS hit").
		Where(qualifiedColumn("s", models.SpaceColumns.SpaceID)+" = ?", spaceID).
		Where(qualifiedColumn("s", models.SpaceColumns.Status)+" = ?", models.EntityStatusActive).
		Where(qualifiedColumn("s", models.SpaceColumns.DeletedAt)+" IS NULL").
		Where(
			"("+
				qualifiedColumn("s", models.SpaceColumns.OwnerUserID)+" = ? OR "+
				"EXISTS (?) OR "+
				"EXISTS (?) OR "+
				"EXISTS (?)"+
				")",
			normalizedUserID,
			r.db.WithContext(ctx).
				Model(&models.UserAdminRole{}).
				Select("1").
				Where(qualifiedColumn("", models.UserAdminRoleColumns.UserID)+" = ?", normalizedUserID).
				Where(qualifiedColumn("", models.UserAdminRoleColumns.Role)+" = ?", models.AdminRolePlatformAdmin),
			r.db.WithContext(ctx).
				Model(&models.SpaceAdminScope{}).
				Select("1").
				Where(qualifiedColumn("", models.SpaceAdminScopeColumns.SpaceID)+" = "+qualifiedColumn("s", models.SpaceColumns.SpaceID)).
				Where(qualifiedColumn("", models.SpaceAdminScopeColumns.UserID)+" = ?", normalizedUserID),
			r.db.WithContext(ctx).
				Model(&models.SpaceMember{}).
				Select("1").
				Where(qualifiedColumn("", models.SpaceMemberColumns.SpaceID)+" = "+qualifiedColumn("s", models.SpaceColumns.SpaceID)).
				Where(qualifiedColumn("", models.SpaceMemberColumns.UserID)+" = ?", normalizedUserID).
				Where(qualifiedColumn("", models.SpaceMemberColumns.Role)+" IN ?", []models.Role{models.RoleOwner, models.RoleCollaborator}),
		).
		Limit(1).
		Take(&probe).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func normalizeSpaceMemberRole(value models.Role) models.Role {
	switch models.Role(strings.ToLower(strings.TrimSpace(string(value)))) {
	case models.RoleOwner:
		return models.RoleOwner
	case models.RoleCollaborator:
		return models.RoleCollaborator
	case models.RoleReader:
		return models.RoleReader
	default:
		return ""
	}
}

func normalizeSpaceStatuses(input []models.EntityStatus) []models.EntityStatus {
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

func normalizeSpaceVisibilities(input []models.Visibility) []models.Visibility {
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

func parseSpaceRecordTime(raw string) time.Time {
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
	// 兼容数据库返回的 "YYYY-MM-DD HH:MM:SS+00:00" 等格式，统一转换为 RFC3339 再解析。
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
