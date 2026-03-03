package repository

import (
	"context"
	"errors"
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

	query := r.baseVisibleDocumentsQuery(ctx, params.ActorUserID, params.SpaceID)
	for _, item := range params.Terms {
		normalizedTerm := strings.ToLower(strings.TrimSpace(item))
		if normalizedTerm == "" {
			continue
		}
		likePattern := "%" + normalizedTerm + "%"
		query = query.Where("(LOWER(d.title) LIKE ? OR LOWER(d.content_md) LIKE ?)", likePattern, likePattern)
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
		Select("d.document_id", "d.content_md").
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

	query := r.baseVisibleDocumentsQuery(ctx, params.ActorUserID, params.SpaceID).
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

func (r *gormSearchVisibilityRepository) ResolveUserRoleLevel(
	ctx context.Context,
	spaceID string,
	actorUserID string,
) (int, error) {
	if r == nil || r.db == nil {
		return 0, fmt.Errorf("search visibility repository db is nil")
	}

	normalizedSpaceID := strings.TrimSpace(spaceID)
	normalizedActorUserID := strings.TrimSpace(actorUserID)
	if normalizedSpaceID == "" || normalizedActorUserID == "" {
		return 0, nil
	}

	type spaceOwnerRow struct {
		OwnerUserID string `gorm:"column:owner_user_id"`
	}
	var ownerRow spaceOwnerRow
	if err := r.db.WithContext(ctx).
		Table("spaces").
		Select("owner_user_id").
		Where("space_id = ?", normalizedSpaceID).
		Take(&ownerRow).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}
	if strings.TrimSpace(ownerRow.OwnerUserID) == normalizedActorUserID {
		return 3, nil
	}

	type memberRoleRow struct {
		Role string `gorm:"column:role"`
	}
	var memberRow memberRoleRow
	if err := r.db.WithContext(ctx).
		Table("space_members").
		Select("role").
		Where("space_id = ? AND user_id = ?", normalizedSpaceID, normalizedActorUserID).
		Take(&memberRow).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}

	switch models.Role(strings.ToLower(strings.TrimSpace(memberRow.Role))) {
	case models.RoleCollaborator:
		return 2, nil
	case models.RoleReader:
		return 1, nil
	default:
		return 0, nil
	}
}

func (r *gormSearchVisibilityRepository) baseVisibleDocumentsQuery(
	ctx context.Context,
	actorUserID string,
	spaceID string,
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
