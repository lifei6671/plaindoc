package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

type gormSearchVisibilityRepository struct {
	db *gorm.DB
}

// NewGormSearchVisibilityRepository 创建检索可见性仓储。
func NewGormSearchVisibilityRepository(db *gorm.DB) SearchVisibilityRepository {
	return &gormSearchVisibilityRepository{db: db}
}

func (r *gormSearchVisibilityRepository) SearchVisibleDocuments(
	ctx context.Context,
	params SearchVisibleDocumentsParams,
) ([]SearchVisibleDocumentRow, int64, error) {
	if r == nil || r.db == nil {
		return nil, 0, fmt.Errorf("search visibility repository db is nil")
	}

	query := r.baseVisibleDocumentsQuery(ctx, params.ActorUserID, params.SpaceID, params.ScopeSpaceIDs)
	normalizedTerms := make([]string, 0, len(params.Terms))
	for _, item := range params.Terms {
		normalizedTerm := strings.ToLower(strings.TrimSpace(item))
		if normalizedTerm == "" {
			continue
		}
		normalizedTerms = append(normalizedTerms, normalizedTerm)
	}
	if len(normalizedTerms) > 0 {
		clauses := make([]string, 0, len(normalizedTerms))
		args := make([]any, 0, len(normalizedTerms)*2)
		for _, item := range normalizedTerms {
			clauses = append(clauses, "(LOWER("+qualifiedColumn("d", models.DocumentColumns.Title)+") LIKE ? OR LOWER("+qualifiedColumn("d", models.DocumentColumns.ContentMD)+") LIKE ?)")
			likePattern := "%" + item + "%"
			args = append(args, likePattern, likePattern)
		}
		query = query.Where("("+strings.Join(clauses, " OR ")+")", args...)
	}

	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total <= 0 {
		return []SearchVisibleDocumentRow{}, 0, nil
	}

	rows := make([]SearchVisibleDocumentRow, 0, params.Limit)
	err := query.Session(&gorm.Session{}).
		Select(selectColumns(
			qualifiedColumn("s", models.SpaceColumns.SpaceID)+" AS space_id",
			qualifiedColumn("d", models.DocumentColumns.DocumentID),
			qualifiedColumn("d", models.DocumentColumns.Title),
			qualifiedColumn("d", models.DocumentColumns.ContentMD),
			"CASE "+
				"WHEN ("+qualifiedColumn("s", models.SpaceColumns.Visibility)+" = 'member' OR "+qualifiedColumn("d", models.DocumentColumns.Visibility)+" = 'member') THEN 'member' "+
				"WHEN ("+qualifiedColumn("s", models.SpaceColumns.Visibility)+" = 'authenticated' OR "+qualifiedColumn("d", models.DocumentColumns.Visibility)+" = 'authenticated') THEN 'authenticated' "+
				"ELSE 'public' "+
				"END AS visibility_scope",
			"CASE "+
				"WHEN ("+qualifiedColumn("s", models.SpaceColumns.Visibility)+" = 'member' OR "+qualifiedColumn("d", models.DocumentColumns.Visibility)+" = 'member') THEN 1 "+
				"ELSE 0 "+
				"END AS min_role",
		)).
		Order(qualifiedColumn("d", models.DocumentColumns.UpdatedAt) + " DESC, " + qualifiedColumn("d", models.DocumentColumns.ID) + " DESC").
		Limit(params.Limit).
		Offset(params.Offset).
		Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *gormSearchVisibilityRepository) FilterVisibleDocumentIDsByCandidates(
	ctx context.Context,
	params SearchVisibleDocumentIDsByCandidatesParams,
) ([]string, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("search visibility repository db is nil")
	}
	if len(params.CandidateDocumentIDs) == 0 {
		return []string{}, nil
	}

	query := r.baseVisibleDocumentsQuery(ctx, params.ActorUserID, params.SpaceID, params.ScopeSpaceIDs).
		Where(qualifiedColumn("d", models.DocumentColumns.DocumentID)+" IN ?", params.CandidateDocumentIDs)

	type row struct {
		DocumentID string `gorm:"column:document_id"`
	}
	rows := make([]row, 0, len(params.CandidateDocumentIDs))
	if err := query.Select(qualifiedColumn("d", models.DocumentColumns.DocumentID)).Find(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]string, 0, len(rows))
	for _, item := range rows {
		documentID := strings.TrimSpace(item.DocumentID)
		if documentID == "" {
			continue
		}
		result = append(result, documentID)
	}
	return result, nil
}

func (r *gormSearchVisibilityRepository) ResolveSearchScopeSpaceIDs(
	ctx context.Context,
	actorUserID string,
) ([]string, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("search visibility repository db is nil")
	}

	normalizedActorUserID := strings.TrimSpace(actorUserID)
	query := r.db.WithContext(ctx).
		Table(tableWithAlias(models.Space{}, "s")).
		Where(qualifiedColumn("s", models.SpaceColumns.Status)+" = ?", models.EntityStatusActive).
		Where(qualifiedColumn("s", models.SpaceColumns.DeletedAt) + " IS NULL")

	if normalizedActorUserID == "" {
		query = query.Where(qualifiedColumn("s", models.SpaceColumns.Visibility)+" = ?", models.VisibilityPublic)
	} else {
		query = query.
			Joins("LEFT JOIN "+tableName(models.SpaceMember{})+" AS sm ON sm."+models.SpaceMemberColumns.SpaceID+" = "+qualifiedColumn("s", models.SpaceColumns.SpaceID)+" AND sm."+models.SpaceMemberColumns.UserID+" = ?", normalizedActorUserID).
			Where(
				"("+
					qualifiedColumn("s", models.SpaceColumns.OwnerUserID)+" = ? OR "+
					qualifiedColumn("s", models.SpaceColumns.Visibility)+" IN (?,?) OR "+
					qualifiedColumn("sm", models.SpaceMemberColumns.ID)+" IS NOT NULL"+
					")",
				normalizedActorUserID,
				models.VisibilityPublic,
				models.VisibilityAuthenticated,
			)
	}

	type row struct {
		SpaceID string `gorm:"column:space_id"`
	}
	rows := make([]row, 0, 64)
	if err := query.Distinct(qualifiedColumn("s", models.SpaceColumns.SpaceID)).Select(qualifiedColumn("s", models.SpaceColumns.SpaceID)).Find(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]string, 0, len(rows))
	for _, item := range rows {
		spaceID := strings.TrimSpace(item.SpaceID)
		if spaceID == "" {
			continue
		}
		result = append(result, spaceID)
	}
	return result, nil
}

func (r *gormSearchVisibilityRepository) ResolveUserRoleLevel(
	ctx context.Context,
	spaceID string,
	actorUserID string,
) (int, error) {
	roleLevels, err := r.ResolveUserRoleLevelsBySpaces(ctx, actorUserID, []string{spaceID})
	if err != nil {
		return 0, err
	}
	normalizedSpaceID := strings.TrimSpace(spaceID)
	if normalizedSpaceID == "" {
		return 0, nil
	}
	return roleLevels[normalizedSpaceID], nil
}

func (r *gormSearchVisibilityRepository) ResolveUserRoleLevelsBySpaces(
	ctx context.Context,
	actorUserID string,
	spaceIDs []string,
) (map[string]int, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("search visibility repository db is nil")
	}

	normalizedActorUserID := strings.TrimSpace(actorUserID)
	normalizedSpaceIDs := normalizeSearchVisibilitySpaceIDs(spaceIDs)
	if normalizedActorUserID == "" || len(normalizedSpaceIDs) == 0 {
		return map[string]int{}, nil
	}

	roleLevels := make(map[string]int, len(normalizedSpaceIDs))

	type ownerRow struct {
		SpaceID string `gorm:"column:space_id"`
	}
	ownerRows := make([]ownerRow, 0, len(normalizedSpaceIDs))
	if err := r.db.WithContext(ctx).
		Model(&models.Space{}).
		Select(qualifiedColumn("", models.SpaceColumns.SpaceID)).
		Where(qualifiedColumn("", models.SpaceColumns.SpaceID)+" IN ?", normalizedSpaceIDs).
		Where(qualifiedColumn("", models.SpaceColumns.OwnerUserID)+" = ?", normalizedActorUserID).
		Find(&ownerRows).Error; err != nil {
		return nil, err
	}
	for _, item := range ownerRows {
		spaceID := strings.TrimSpace(item.SpaceID)
		if spaceID == "" {
			continue
		}
		roleLevels[spaceID] = 3
	}

	type memberRoleRow struct {
		SpaceID string `gorm:"column:space_id"`
		Role    string `gorm:"column:role"`
	}
	memberRows := make([]memberRoleRow, 0, len(normalizedSpaceIDs))
	if err := r.db.WithContext(ctx).
		Model(&models.SpaceMember{}).
		Select(selectColumns(
			qualifiedColumn("", models.SpaceMemberColumns.SpaceID),
			qualifiedColumn("", models.SpaceMemberColumns.Role),
		)).
		Where(qualifiedColumn("", models.SpaceMemberColumns.SpaceID)+" IN ?", normalizedSpaceIDs).
		Where(qualifiedColumn("", models.SpaceMemberColumns.UserID)+" = ?", normalizedActorUserID).
		Find(&memberRows).Error; err != nil {
		return nil, err
	}
	for _, item := range memberRows {
		spaceID := strings.TrimSpace(item.SpaceID)
		if spaceID == "" {
			continue
		}
		level := roleLevelFromMemberRole(item.Role)
		if level <= 0 {
			continue
		}
		if level > roleLevels[spaceID] {
			roleLevels[spaceID] = level
		}
	}

	return roleLevels, nil
}

func normalizeSearchVisibilitySpaceIDs(items []string) []string {
	if len(items) == 0 {
		return []string{}
	}
	result := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		spaceID := strings.TrimSpace(item)
		if spaceID == "" {
			continue
		}
		if _, exists := seen[spaceID]; exists {
			continue
		}
		seen[spaceID] = struct{}{}
		result = append(result, spaceID)
	}
	return result
}

func roleLevelFromMemberRole(role string) int {
	switch models.Role(strings.ToLower(strings.TrimSpace(role))) {
	case models.RoleOwner:
		return 3
	case models.RoleCollaborator:
		return 2
	case models.RoleReader:
		return 1
	default:
		return 0
	}
}

func (r *gormSearchVisibilityRepository) baseVisibleDocumentsQuery(
	ctx context.Context,
	actorUserID string,
	spaceID string,
	scopeSpaceIDs []string,
) *gorm.DB {
	query := r.db.WithContext(ctx).
		Table(tableWithAlias(models.Document{}, "d")).
		Joins("JOIN "+tableName(models.Node{})+" AS n ON "+qualifiedColumn("n", models.NodeColumns.NodeID)+" = "+qualifiedColumn("d", models.DocumentColumns.NodeID)).
		Joins("JOIN "+tableName(models.Space{})+" AS s ON "+qualifiedColumn("s", models.SpaceColumns.SpaceID)+" = "+qualifiedColumn("n", models.NodeColumns.SpaceID)).
		Where(qualifiedColumn("s", models.SpaceColumns.Status)+" = ?", models.EntityStatusActive).
		Where(qualifiedColumn("s", models.SpaceColumns.DeletedAt)+" IS NULL").
		Where(qualifiedColumn("d", models.DocumentColumns.Status)+" = ?", models.EntityStatusActive).
		Where(qualifiedColumn("d", models.DocumentColumns.DeletedAt) + " IS NULL")
	if r.hasDocumentFormatColumn() {
		query = query.Where(qualifiedColumn("d", models.DocumentColumns.Format)+" = ?", models.DocumentFormatMarkdown)
	}

	normalizedSpaceID := strings.TrimSpace(spaceID)
	if normalizedSpaceID != "" {
		query = query.Where(qualifiedColumn("s", models.SpaceColumns.SpaceID)+" = ?", normalizedSpaceID)
	} else {
		normalizedScopeSpaceIDs := normalizeSearchVisibilitySpaceIDs(scopeSpaceIDs)
		if len(normalizedScopeSpaceIDs) > 0 {
			query = query.Where(qualifiedColumn("s", models.SpaceColumns.SpaceID)+" IN ?", normalizedScopeSpaceIDs)
		}
	}
	return applySearchVisibilityMatrixFilter(query, actorUserID)
}

func (r *gormSearchVisibilityRepository) hasDocumentFormatColumn() bool {
	if r == nil || r.db == nil {
		return false
	}
	// 兼容旧测试库或迁移前数据：没有 format 列的 documents 只包含 Markdown 文档。
	return r.db.Migrator().HasColumn(&models.Document{}, models.DocumentColumns.Format)
}

// applySearchVisibilityMatrixFilter 按检索权限矩阵追加过滤条件。
//
// 规则：
// 1) 未登录：仅允许 space=public 且 doc=public；
// 2) 已登录：允许
//   - owner 空间内全部文档；
//   - 非成员场景下的 (space in [public,authenticated] AND doc in [public,authenticated])；
//   - 该空间成员（owner/collaborator/reader）看到该空间全部文档。
func applySearchVisibilityMatrixFilter(query *gorm.DB, actorUserID string) *gorm.DB {
	if query == nil {
		return query
	}

	normalizedActorUserID := strings.TrimSpace(actorUserID)
	if normalizedActorUserID == "" {
		return query.Where(
			qualifiedColumn("s", models.SpaceColumns.Visibility)+" = ? AND "+qualifiedColumn("d", models.DocumentColumns.Visibility)+" = ?",
			models.VisibilityPublic,
			models.VisibilityPublic,
		)
	}

	return query.
		Joins("LEFT JOIN "+tableName(models.SpaceMember{})+" AS sm ON sm."+models.SpaceMemberColumns.SpaceID+" = "+qualifiedColumn("s", models.SpaceColumns.SpaceID)+" AND sm."+models.SpaceMemberColumns.UserID+" = ?", normalizedActorUserID).
		Where(
			"("+
				qualifiedColumn("s", models.SpaceColumns.OwnerUserID)+" = ? OR "+
				"(("+qualifiedColumn("s", models.SpaceColumns.Visibility)+" IN (?,?)) AND ("+qualifiedColumn("d", models.DocumentColumns.Visibility)+" IN (?,?))) OR "+
				qualifiedColumn("sm", models.SpaceMemberColumns.ID)+" IS NOT NULL"+
				")",
			normalizedActorUserID,
			models.VisibilityPublic,
			models.VisibilityAuthenticated,
			models.VisibilityPublic,
			models.VisibilityAuthenticated,
		)
}
