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
			clauses = append(clauses, "(LOWER(d.title) LIKE ? OR LOWER(d.content_md) LIKE ?)")
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
		Select(
			"s.space_id AS space_id",
			"d.document_id",
			"d.title",
			"d.content_md",
			"CASE "+
				"WHEN (s.visibility = 'member' OR d.visibility = 'member') THEN 'member' "+
				"WHEN (s.visibility = 'authenticated' OR d.visibility = 'authenticated') THEN 'authenticated' "+
				"ELSE 'public' "+
				"END AS visibility_scope",
			"CASE "+
				"WHEN (s.visibility = 'member' OR d.visibility = 'member') THEN 1 "+
				"ELSE 0 "+
				"END AS min_role",
		).
		Order("d.updated_at DESC, d.id DESC").
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
		Where("d.document_id IN ?", params.CandidateDocumentIDs)

	type row struct {
		DocumentID string `gorm:"column:document_id"`
	}
	rows := make([]row, 0, len(params.CandidateDocumentIDs))
	if err := query.Select("d.document_id").Find(&rows).Error; err != nil {
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
		Table("spaces AS s").
		Where("s.status = ? AND s.deleted_at IS NULL", models.EntityStatusActive)

	if normalizedActorUserID == "" {
		query = query.Where("s.visibility = ?", models.VisibilityPublic)
	} else {
		query = query.
			Joins("LEFT JOIN space_members AS sm ON sm.space_id = s.space_id AND sm.user_id = ?", normalizedActorUserID).
			Where(
				"("+
					"s.owner_user_id = ? OR "+
					"s.visibility IN (?,?) OR "+
					"sm.id IS NOT NULL"+
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
	if err := query.Distinct("s.space_id").Select("s.space_id").Find(&rows).Error; err != nil {
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
		Table("spaces").
		Select("space_id").
		Where("space_id IN ? AND owner_user_id = ?", normalizedSpaceIDs, normalizedActorUserID).
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
		Table("space_members").
		Select("space_id", "role").
		Where("space_id IN ? AND user_id = ?", normalizedSpaceIDs, normalizedActorUserID).
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
		Table("documents AS d").
		Joins("JOIN nodes AS n ON n.node_id = d.node_id").
		Joins("JOIN spaces AS s ON s.space_id = n.space_id").
		Where("s.status = ? AND s.deleted_at IS NULL", models.EntityStatusActive).
		Where("d.status = ? AND d.deleted_at IS NULL", models.EntityStatusActive)

	normalizedSpaceID := strings.TrimSpace(spaceID)
	if normalizedSpaceID != "" {
		query = query.Where("s.space_id = ?", normalizedSpaceID)
	} else {
		normalizedScopeSpaceIDs := normalizeSearchVisibilitySpaceIDs(scopeSpaceIDs)
		if len(normalizedScopeSpaceIDs) > 0 {
			query = query.Where("s.space_id IN ?", normalizedScopeSpaceIDs)
		}
	}
	return applySearchVisibilityMatrixFilter(query, actorUserID)
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
			"s.visibility = ? AND d.visibility = ?",
			models.VisibilityPublic,
			models.VisibilityPublic,
		)
	}

	return query.
		Joins("LEFT JOIN space_members AS sm ON sm.space_id = s.space_id AND sm.user_id = ?", normalizedActorUserID).
		Where(
			"("+
				"s.owner_user_id = ? OR "+
				"((s.visibility IN (?,?)) AND (d.visibility IN (?,?))) OR "+
				"sm.id IS NOT NULL"+
				")",
			normalizedActorUserID,
			models.VisibilityPublic,
			models.VisibilityAuthenticated,
			models.VisibilityPublic,
			models.VisibilityAuthenticated,
		)
}
