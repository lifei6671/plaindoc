package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

type gormWorkspaceRepository struct {
	db                 *gorm.DB
	searchIndexJobRepo SearchIndexJobRepository
}

// NewGormWorkspaceRepository 创建基于 GORM 的编辑器工作区仓储实现。
func NewGormWorkspaceRepository(
	db *gorm.DB,
	searchIndexJobRepos ...SearchIndexJobRepository,
) WorkspaceRepository {
	var searchIndexJobRepo SearchIndexJobRepository
	if len(searchIndexJobRepos) > 0 {
		searchIndexJobRepo = searchIndexJobRepos[0]
	}
	return &gormWorkspaceRepository{
		db:                 db,
		searchIndexJobRepo: searchIndexJobRepo,
	}
}

func (r *gormWorkspaceRepository) ListSpacesByActor(
	ctx context.Context,
	actorUserID string,
) ([]WorkspaceSpaceListRecord, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("workspace repository db is nil")
	}

	userID := strings.TrimSpace(actorUserID)
	if userID == "" {
		return []WorkspaceSpaceListRecord{}, nil
	}

	type spaceRow struct {
		SpaceID      string `gorm:"column:space_id"`
		Name         string `gorm:"column:name"`
		CreatedAtRaw string `gorm:"column:created_at"`
		UpdatedAtRaw string `gorm:"column:updated_at"`
	}

	var rows []spaceRow
	platformAdminQuery := r.db.WithContext(ctx).
		Model(&models.UserAdminRole{}).
		Select("1").
		Where(qualifiedColumn("", models.UserAdminRoleColumns.UserID)+" = ?", userID).
		Where(qualifiedColumn("", models.UserAdminRoleColumns.Role)+" = ?", models.AdminRolePlatformAdmin)
	spaceAdminScopeQuery := r.db.WithContext(ctx).
		Model(&models.SpaceAdminScope{}).
		Select("1").
		Where(qualifiedColumn("", models.SpaceAdminScopeColumns.SpaceID)+" = "+qualifiedColumn("s", models.SpaceColumns.SpaceID)).
		Where(qualifiedColumn("", models.SpaceAdminScopeColumns.UserID)+" = ?", userID)
	spaceMemberQuery := r.db.WithContext(ctx).
		Model(&models.SpaceMember{}).
		Select("1").
		Where(qualifiedColumn("", models.SpaceMemberColumns.SpaceID)+" = "+qualifiedColumn("s", models.SpaceColumns.SpaceID)).
		Where(qualifiedColumn("", models.SpaceMemberColumns.UserID)+" = ?", userID).
		Where(qualifiedColumn("", models.SpaceMemberColumns.Role)+" IN ?", []models.Role{models.RoleOwner, models.RoleCollaborator})
	if err := r.db.WithContext(ctx).
		Table(tableWithAlias(models.Space{}, "s")).
		Select(selectColumns(
			qualifiedColumn("s", models.SpaceColumns.SpaceID),
			qualifiedColumn("s", models.SpaceColumns.Name),
			qualifiedColumn("s", models.SpaceColumns.CreatedAt),
			qualifiedColumn("s", models.SpaceColumns.UpdatedAt),
		)).
		Where(qualifiedColumn("s", models.SpaceColumns.Status)+" = ?", models.EntityStatusActive).
		Where(qualifiedColumn("s", models.SpaceColumns.DeletedAt)+" IS NULL").
		Where(
			"("+
				"EXISTS (?) OR "+
				qualifiedColumn("s", models.SpaceColumns.OwnerUserID)+" = ? OR "+
				"EXISTS (?) OR "+
				"EXISTS (?)"+
				")",
			platformAdminQuery,
			userID,
			spaceAdminScopeQuery,
			spaceMemberQuery,
		).
		Order(qualifiedColumn("s", models.SpaceColumns.UpdatedAt) + " DESC, " + qualifiedColumn("s", models.SpaceColumns.ID) + " DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]WorkspaceSpaceListRecord, 0, len(rows))
	for _, row := range rows {
		items = append(items, WorkspaceSpaceListRecord{
			SpaceID:      strings.TrimSpace(row.SpaceID),
			Name:         strings.TrimSpace(row.Name),
			CreatedAtRaw: row.CreatedAtRaw,
			UpdatedAtRaw: row.UpdatedAtRaw,
		})
	}
	return items, nil
}

func (r *gormWorkspaceRepository) GetDefaultCategory(ctx context.Context) (*models.SpaceCategory, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("workspace repository db is nil")
	}

	type categoryRow struct {
		CategoryID string `gorm:"column:category_id"`
		Name       string `gorm:"column:name"`
	}

	var row categoryRow
	if err := r.db.WithContext(ctx).
		Model(&models.SpaceCategory{}).
		Select(selectColumns(
			qualifiedColumn("", models.SpaceCategoryColumns.CategoryID),
			qualifiedColumn("", models.SpaceCategoryColumns.Name),
		)).
		Where(qualifiedColumn("", models.SpaceCategoryColumns.IsDefault)+" = ?", true).
		Order(qualifiedColumn("", models.SpaceCategoryColumns.ID) + " ASC").
		Take(&row).Error; err != nil {
		return nil, err
	}

	return &models.SpaceCategory{
		CategoryID: strings.TrimSpace(row.CategoryID),
		Name:       strings.TrimSpace(row.Name),
		IsDefault:  true,
	}, nil
}

func (r *gormWorkspaceRepository) CreateSpace(ctx context.Context, space *models.Space) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("workspace repository db is nil")
	}
	if space == nil {
		return fmt.Errorf("space must not be nil")
	}

	space.SpaceID = strings.TrimSpace(space.SpaceID)
	space.Name = strings.TrimSpace(space.Name)
	space.Description = strings.TrimSpace(space.Description)
	space.CategoryID = strings.TrimSpace(space.CategoryID)
	space.Category = strings.TrimSpace(space.Category)
	space.OwnerUserID = strings.TrimSpace(space.OwnerUserID)
	if !models.IsValidVisibility(space.Visibility) {
		space.Visibility = models.VisibilityMember
	}
	if !models.IsValidEntityStatus(space.Status) {
		space.Status = models.EntityStatusActive
	}

	return r.db.WithContext(ctx).Create(space).Error
}

func (r *gormWorkspaceRepository) GetSpacePermissionSnapshot(
	ctx context.Context,
	spaceID string,
	actorUserID string,
) (*WorkspaceSpacePermissionSnapshot, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("workspace repository db is nil")
	}

	normalizedSpaceID := strings.TrimSpace(spaceID)
	normalizedActorUserID := strings.TrimSpace(actorUserID)
	if normalizedSpaceID == "" {
		return nil, gorm.ErrRecordNotFound
	}

	type spacePermissionRow struct {
		SpaceID            string              `gorm:"column:space_id"`
		OwnerUserID        string              `gorm:"column:owner_user_id"`
		Visibility         models.Visibility   `gorm:"column:visibility"`
		Status             models.EntityStatus `gorm:"column:status"`
		DeletedAt          *time.Time          `gorm:"column:deleted_at"`
		IsPlatformAdmin    int                 `gorm:"column:is_platform_admin"`
		HasSpaceAdminScope int                 `gorm:"column:has_space_admin_scope"`
		MemberRole         *string             `gorm:"column:member_role"`
	}

	var row spacePermissionRow
	if err := r.db.WithContext(ctx).
		Table(tableWithAlias(models.Space{}, "s")).
		Select(selectColumns(
			qualifiedColumn("s", models.SpaceColumns.SpaceID),
			qualifiedColumn("s", models.SpaceColumns.OwnerUserID),
			qualifiedColumn("s", models.SpaceColumns.Visibility),
			qualifiedColumn("s", models.SpaceColumns.Status),
			qualifiedColumn("s", models.SpaceColumns.DeletedAt),
			"CASE WHEN uar."+models.UserAdminRoleColumns.UserID+" IS NULL THEN 0 ELSE 1 END AS is_platform_admin",
			"CASE WHEN sas."+models.SpaceAdminScopeColumns.ID+" IS NULL THEN 0 ELSE 1 END AS has_space_admin_scope",
			"sm."+models.SpaceMemberColumns.Role+" AS member_role",
		)).
		Joins(
			"LEFT JOIN "+tableName(models.UserAdminRole{})+" AS uar ON uar."+models.UserAdminRoleColumns.UserID+" = ? AND uar."+models.UserAdminRoleColumns.Role+" = ?",
			normalizedActorUserID,
			models.AdminRolePlatformAdmin,
		).
		Joins(
			"LEFT JOIN "+tableName(models.SpaceAdminScope{})+" AS sas ON sas."+models.SpaceAdminScopeColumns.UserID+" = ? AND sas."+models.SpaceAdminScopeColumns.SpaceID+" = "+qualifiedColumn("s", models.SpaceColumns.SpaceID),
			normalizedActorUserID,
		).
		Joins(
			"LEFT JOIN "+tableName(models.SpaceMember{})+" AS sm ON sm."+models.SpaceMemberColumns.UserID+" = ? AND sm."+models.SpaceMemberColumns.SpaceID+" = "+qualifiedColumn("s", models.SpaceColumns.SpaceID),
			normalizedActorUserID,
		).
		Where(qualifiedColumn("s", models.SpaceColumns.SpaceID)+" = ?", normalizedSpaceID).
		Take(&row).Error; err != nil {
		return nil, err
	}

	if !models.IsValidVisibility(row.Visibility) {
		row.Visibility = models.VisibilityMember
	}
	if !models.IsValidEntityStatus(row.Status) {
		row.Status = models.EntityStatusActive
	}

	var memberRole *models.Role
	if row.MemberRole != nil {
		normalized := strings.ToLower(strings.TrimSpace(*row.MemberRole))
		if normalized != "" {
			role := models.Role(normalized)
			switch role {
			case models.RoleOwner, models.RoleCollaborator, models.RoleReader:
			default:
				role = models.RoleReader
			}
			memberRole = &role
		}
	}

	return &WorkspaceSpacePermissionSnapshot{
		SpaceID:            strings.TrimSpace(row.SpaceID),
		OwnerUserID:        strings.TrimSpace(row.OwnerUserID),
		Visibility:         row.Visibility,
		Status:             row.Status,
		DeletedAt:          row.DeletedAt,
		IsPlatformAdmin:    row.IsPlatformAdmin > 0,
		HasSpaceAdminScope: row.HasSpaceAdminScope > 0,
		MemberRole:         memberRole,
	}, nil
}

func (r *gormWorkspaceRepository) ListTreeNodesBySpaceID(
	ctx context.Context,
	spaceID string,
) ([]WorkspaceTreeNodeRecord, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("workspace repository db is nil")
	}

	type nodeRow struct {
		NodeID             string          `gorm:"column:node_id"`
		DocumentID         *string         `gorm:"column:document_id"`
		ReaderSlug         *string         `gorm:"column:reader_slug"`
		SpaceID            string          `gorm:"column:space_id"`
		ParentNodeID       *string         `gorm:"column:parent_node_id"`
		Type               models.NodeType `gorm:"column:type"`
		Title              string          `gorm:"column:title"`
		Sort               int             `gorm:"column:sort"`
		DocumentVisibility *string         `gorm:"column:document_visibility"`
		DocumentFormat     *string         `gorm:"column:document_format"`
	}

	var rows []nodeRow
	if err := r.db.WithContext(ctx).
		Table(tableWithAlias(models.Node{}, "n")).
		Select(selectColumns(
			qualifiedColumn("n", models.NodeColumns.NodeID),
			qualifiedColumn("d", models.DocumentColumns.DocumentID)+" AS document_id",
			qualifiedColumn("n", models.NodeColumns.ReaderSlug),
			qualifiedColumn("n", models.NodeColumns.SpaceID),
			qualifiedColumn("n", models.NodeColumns.ParentNodeID),
			qualifiedColumn("n", models.NodeColumns.Type),
			qualifiedColumn("n", models.NodeColumns.Title),
			qualifiedColumn("n", models.NodeColumns.Sort),
			qualifiedColumn("d", models.DocumentColumns.Visibility)+" AS document_visibility",
			qualifiedColumn("d", models.DocumentColumns.Format)+" AS document_format",
		)).
		Joins("LEFT JOIN "+tableName(models.Document{})+" AS d ON "+qualifiedColumn("d", models.DocumentColumns.NodeID)+" = "+qualifiedColumn("n", models.NodeColumns.NodeID)).
		Where(qualifiedColumn("n", models.NodeColumns.SpaceID)+" = ?", strings.TrimSpace(spaceID)).
		Where(
			"("+qualifiedColumn("n", models.NodeColumns.Type)+" <> ? OR ("+qualifiedColumn("d", models.DocumentColumns.DocumentID)+" IS NOT NULL AND "+qualifiedColumn("d", models.DocumentColumns.DeletedAt)+" IS NULL AND "+qualifiedColumn("d", models.DocumentColumns.Status)+" <> ?))",
			models.NodeTypeDoc,
			models.EntityStatusDeleted,
		).
		Order(qualifiedColumn("n", models.NodeColumns.ParentNodeID) + " ASC, " + qualifiedColumn("n", models.NodeColumns.Sort) + " ASC, " + qualifiedColumn("n", models.NodeColumns.ID) + " ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]WorkspaceTreeNodeRecord, 0, len(rows))
	for _, row := range rows {
		items = append(items, WorkspaceTreeNodeRecord{
			NodeID:             strings.TrimSpace(row.NodeID),
			DocumentID:         trimOptionalString(row.DocumentID),
			ReaderSlug:         trimOptionalString(row.ReaderSlug),
			SpaceID:            strings.TrimSpace(row.SpaceID),
			ParentNodeID:       trimOptionalString(row.ParentNodeID),
			Type:               row.Type,
			Title:              strings.TrimSpace(row.Title),
			Sort:               row.Sort,
			DocumentVisibility: trimOptionalString(row.DocumentVisibility),
			DocumentFormat:     normalizeOptionalDocumentFormat(row.DocumentFormat),
		})
	}

	return items, nil
}

func (r *gormWorkspaceRepository) GetNodeByNodeID(
	ctx context.Context,
	nodeID string,
) (*WorkspaceNodeRecord, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("workspace repository db is nil")
	}

	type nodeRow struct {
		NodeID       string          `gorm:"column:node_id"`
		SpaceID      string          `gorm:"column:space_id"`
		ParentNodeID *string         `gorm:"column:parent_node_id"`
		ReaderSlug   *string         `gorm:"column:reader_slug"`
		Type         models.NodeType `gorm:"column:type"`
		Title        string          `gorm:"column:title"`
		Sort         int             `gorm:"column:sort"`
	}

	var row nodeRow
	if err := r.db.WithContext(ctx).
		Model(&models.Node{}).
		Select(selectColumns(
			qualifiedColumn("", models.NodeColumns.NodeID),
			qualifiedColumn("", models.NodeColumns.SpaceID),
			qualifiedColumn("", models.NodeColumns.ParentNodeID),
			qualifiedColumn("", models.NodeColumns.ReaderSlug),
			qualifiedColumn("", models.NodeColumns.Type),
			qualifiedColumn("", models.NodeColumns.Title),
			qualifiedColumn("", models.NodeColumns.Sort),
		)).
		Where(qualifiedColumn("", models.NodeColumns.NodeID)+" = ?", strings.TrimSpace(nodeID)).
		Take(&row).Error; err != nil {
		return nil, err
	}

	nodeType := row.Type
	if nodeType != models.NodeTypeFolder && nodeType != models.NodeTypeDoc {
		nodeType = models.NodeTypeDoc
	}

	return &WorkspaceNodeRecord{
		NodeID:       strings.TrimSpace(row.NodeID),
		SpaceID:      strings.TrimSpace(row.SpaceID),
		ParentNodeID: trimOptionalString(row.ParentNodeID),
		ReaderSlug:   trimOptionalString(row.ReaderSlug),
		Type:         nodeType,
		Title:        strings.TrimSpace(row.Title),
		Sort:         row.Sort,
	}, nil
}

func (r *gormWorkspaceRepository) GetMaxNodeSort(
	ctx context.Context,
	spaceID string,
	parentNodeID *string,
) (int, error) {
	if r == nil || r.db == nil {
		return 0, fmt.Errorf("workspace repository db is nil")
	}

	type maxSortRow struct {
		Value int `gorm:"column:value"`
	}

	query := r.db.WithContext(ctx).
		Model(&models.Node{}).
		Select("COALESCE(MAX("+qualifiedColumn("", models.NodeColumns.Sort)+"), 0) AS value").
		Where(qualifiedColumn("", models.NodeColumns.SpaceID)+" = ?", strings.TrimSpace(spaceID))
	if parentNodeID == nil {
		query = query.Where(qualifiedColumn("", models.NodeColumns.ParentNodeID) + " IS NULL")
	} else {
		query = query.Where(qualifiedColumn("", models.NodeColumns.ParentNodeID)+" = ?", strings.TrimSpace(*parentNodeID))
	}

	var row maxSortRow
	if err := query.Take(&row).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}
	return row.Value, nil
}

func (r *gormWorkspaceRepository) CreateNode(
	ctx context.Context,
	params WorkspaceCreateNodeParams,
) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("workspace repository db is nil")
	}
	if params.Node == nil {
		return fmt.Errorf("workspace create node params must include node")
	}
	if params.Node.ReaderSlug != nil {
		normalizedReaderSlug := strings.ToLower(strings.TrimSpace(*params.Node.ReaderSlug))
		if normalizedReaderSlug == "" {
			params.Node.ReaderSlug = nil
		} else {
			params.Node.ReaderSlug = &normalizedReaderSlug
		}
	}
	if params.Document != nil {
		params.Document.Format = models.NormalizeDocumentFormat(params.Document.Format)
		if params.Document.ContentVersion <= 0 {
			params.Document.ContentVersion = normalizeContentVersion(0, params.Document.Version)
		}
		params.Document.SourceBlobID = trimOptionalString(params.Document.SourceBlobID)
		params.Document.SourceFileName = trimOptionalString(params.Document.SourceFileName)
		params.Document.SourceMimeType = trimOptionalString(params.Document.SourceMimeType)
		if params.Document.Format == models.DocumentFormatMarkdown {
			params.Document.SourceBlobID = nil
			params.Document.SourceFileName = nil
			params.Document.SourceMimeType = nil
		}
	}

	spaceID := strings.TrimSpace(params.TouchSpace)
	if spaceID == "" {
		spaceID = strings.TrimSpace(params.Node.SpaceID)
	}
	touchedAt := params.TouchedAt
	if touchedAt.IsZero() {
		touchedAt = time.Now().UTC()
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(params.Node).Error; err != nil {
			return err
		}
		if params.Document != nil {
			if err := tx.Create(params.Document).Error; err != nil {
				return err
			}
			if err := r.enqueueDocumentUpsertInTx(ctx, tx, params.Document.DocumentID); err != nil {
				return err
			}
		}
		if params.Revision != nil {
			if err := tx.Create(params.Revision).Error; err != nil {
				return err
			}
		}
		if params.FileRevision != nil {
			if err := tx.Create(params.FileRevision).Error; err != nil {
				return err
			}
		}
		if spaceID == "" {
			return nil
		}
		return tx.Model(&models.Space{}).
			Where(qualifiedColumn("", models.SpaceColumns.SpaceID)+" = ?", spaceID).
			Update(models.SpaceColumns.UpdatedAt, touchedAt).Error
	})
}

func (r *gormWorkspaceRepository) UpdateNode(
	ctx context.Context,
	params WorkspaceUpdateNodeParams,
) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("workspace repository db is nil")
	}

	nodeID := strings.TrimSpace(params.NodeID)
	if nodeID == "" {
		return gorm.ErrRecordNotFound
	}
	if len(params.UpdateValues) == 0 {
		return nil
	}
	actorUserID := strings.TrimSpace(params.ActorUserID)
	updateValues := make(map[string]any, len(params.UpdateValues)+1)
	for key, value := range params.UpdateValues {
		updateValues[key] = value
	}
	if actorUserID != "" {
		updateValues[models.NodeColumns.UpdatedByUserID] = actorUserID
	}

	touchedAt := params.TouchedAt
	if touchedAt.IsZero() {
		touchedAt = time.Now().UTC()
	}

	spaceID := strings.TrimSpace(params.TouchSpace)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updateTx := tx.Model(&models.Node{}).
			Where(qualifiedColumn("", models.NodeColumns.NodeID)+" = ?", nodeID).
			Updates(updateValues)
		if updateTx.Error != nil {
			return updateTx.Error
		}
		if updateTx.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		if params.DocumentTitle != nil {
			type documentIdentityRow struct {
				DocumentID string `gorm:"column:document_id"`
			}
			documentUpdates := map[string]any{
				models.DocumentColumns.Title:     strings.TrimSpace(*params.DocumentTitle),
				models.DocumentColumns.UpdatedAt: touchedAt,
			}
			if actorUserID != "" {
				documentUpdates[models.DocumentColumns.UpdatedByUserID] = actorUserID
			}
			if err := tx.Model(&models.Document{}).
				Where(qualifiedColumn("", models.DocumentColumns.NodeID)+" = ?", nodeID).
				Updates(documentUpdates).Error; err != nil {
				return err
			}
			var identity documentIdentityRow
			if err := tx.Model(&models.Document{}).
				Select(qualifiedColumn("", models.DocumentColumns.DocumentID)).
				Where(qualifiedColumn("", models.DocumentColumns.NodeID)+" = ?", nodeID).
				Take(&identity).Error; err != nil {
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
			} else {
				if err := r.enqueueDocumentUpsertInTx(ctx, tx, identity.DocumentID); err != nil {
					return err
				}
			}
		}

		if spaceID == "" {
			return nil
		}
		return tx.Model(&models.Space{}).
			Where(qualifiedColumn("", models.SpaceColumns.SpaceID)+" = ?", spaceID).
			Update(models.SpaceColumns.UpdatedAt, touchedAt).Error
	})
}

func (r *gormWorkspaceRepository) MoveNode(
	ctx context.Context,
	params WorkspaceMoveNodeParams,
) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("workspace repository db is nil")
	}

	nodeID := strings.TrimSpace(params.NodeID)
	if nodeID == "" {
		return gorm.ErrRecordNotFound
	}
	if params.TargetIndex < 0 {
		return fmt.Errorf("workspace move target index must be >= 0")
	}

	actorUserID := strings.TrimSpace(params.ActorUserID)
	touchedAt := params.TouchedAt
	if touchedAt.IsZero() {
		touchedAt = time.Now().UTC()
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		type nodeRow struct {
			NodeID       string  `gorm:"column:node_id"`
			SpaceID      string  `gorm:"column:space_id"`
			ParentNodeID *string `gorm:"column:parent_node_id"`
		}

		var moving nodeRow
		if err := tx.Model(&models.Node{}).
			Select(selectColumns(
				qualifiedColumn("", models.NodeColumns.NodeID),
				qualifiedColumn("", models.NodeColumns.SpaceID),
				qualifiedColumn("", models.NodeColumns.ParentNodeID),
			)).
			Where(qualifiedColumn("", models.NodeColumns.NodeID)+" = ?", nodeID).
			Take(&moving).Error; err != nil {
			return err
		}

		spaceID := strings.TrimSpace(moving.SpaceID)
		oldParentNodeID := trimOptionalString(moving.ParentNodeID)
		targetParentNodeID := trimOptionalString(params.TargetParentNodeID)

		if targetParentNodeID != nil {
			if *targetParentNodeID == nodeID {
				return ErrWorkspaceMoveCycleDetected
			}
			if err := ensureWorkspaceMoveParentPathValid(tx, spaceID, nodeID, *targetParentNodeID); err != nil {
				return err
			}
		}

		// 同父级内重排：只需重建一个同级序列。
		if optionalStringEqual(oldParentNodeID, targetParentNodeID) {
			siblingNodeIDs, err := listWorkspaceSiblingNodeIDs(tx, spaceID, oldParentNodeID, nodeID)
			if err != nil {
				return err
			}
			reorderedNodeIDs := insertWorkspaceNodeIDAt(siblingNodeIDs, nodeID, params.TargetIndex)
			if err := resequenceWorkspaceSiblingNodes(
				tx,
				reorderedNodeIDs,
				oldParentNodeID,
				actorUserID,
				touchedAt,
			); err != nil {
				return err
			}
		} else {
			// 跨父级移动：旧父级移除后重排，新父级插入后重排。
			oldSiblingNodeIDs, err := listWorkspaceSiblingNodeIDs(tx, spaceID, oldParentNodeID, nodeID)
			if err != nil {
				return err
			}
			newSiblingNodeIDs, err := listWorkspaceSiblingNodeIDs(tx, spaceID, targetParentNodeID, nodeID)
			if err != nil {
				return err
			}
			reorderedTargetNodeIDs := insertWorkspaceNodeIDAt(newSiblingNodeIDs, nodeID, params.TargetIndex)

			if err := resequenceWorkspaceSiblingNodes(
				tx,
				oldSiblingNodeIDs,
				oldParentNodeID,
				actorUserID,
				touchedAt,
			); err != nil {
				return err
			}
			if err := resequenceWorkspaceSiblingNodes(
				tx,
				reorderedTargetNodeIDs,
				targetParentNodeID,
				actorUserID,
				touchedAt,
			); err != nil {
				return err
			}
		}

		if err := tx.Model(&models.Space{}).
			Where(qualifiedColumn("", models.SpaceColumns.SpaceID)+" = ?", spaceID).
			Update(models.SpaceColumns.UpdatedAt, touchedAt).Error; err != nil {
			return err
		}
		return nil
	})
}

func (r *gormWorkspaceRepository) DeleteNode(
	ctx context.Context,
	nodeID string,
	touchSpace string,
	touchedAt time.Time,
) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("workspace repository db is nil")
	}

	normalizedNodeID := strings.TrimSpace(nodeID)
	if normalizedNodeID == "" {
		return false, nil
	}
	normalizedSpaceID := strings.TrimSpace(touchSpace)
	if touchedAt.IsZero() {
		touchedAt = time.Now().UTC()
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		scope, err := collectWorkspaceNodeDeleteScopeInTx(ctx, tx, normalizedNodeID)
		if err != nil {
			return err
		}
		if _, err := DeleteDocumentsCascadeInTx(tx, scope.DocumentIDs); err != nil {
			return err
		}
		deletedNodeCount, err := deleteNodesCascadeInTx(tx, scope.NodeIDs)
		if err != nil {
			return err
		}
		if deletedNodeCount == 0 {
			return gorm.ErrRecordNotFound
		}

		if scope.Root.Type == models.NodeTypeDoc {
			documentIDs := scope.DocumentIDs
			if len(documentIDs) == 0 {
				documentIDs = []string{normalizedNodeID}
			}
			for _, documentID := range documentIDs {
				if err := r.enqueueDocumentDeleteInTx(ctx, tx, documentID); err != nil {
					return err
				}
			}
		} else {
			spaceForRebuild := normalizedSpaceID
			if strings.TrimSpace(spaceForRebuild) == "" {
				spaceForRebuild = strings.TrimSpace(scope.Root.SpaceID)
			}
			if err := r.enqueueSpaceRebuildInTx(ctx, tx, spaceForRebuild); err != nil {
				return err
			}
		}

		if normalizedSpaceID == "" {
			return nil
		}
		return tx.Model(&models.Space{}).
			Where(qualifiedColumn("", models.SpaceColumns.SpaceID)+" = ?", normalizedSpaceID).
			Update(models.SpaceColumns.UpdatedAt, touchedAt).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *gormWorkspaceRepository) GetDocumentByDocumentID(
	ctx context.Context,
	documentID string,
) (*WorkspaceDocumentRecord, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("workspace repository db is nil")
	}

	type documentRow struct {
		DocumentID     string                      `gorm:"column:document_id"`
		NodeID         string                      `gorm:"column:node_id"`
		ReaderSlug     *string                     `gorm:"column:reader_slug"`
		ThemeID        string                      `gorm:"column:theme_id"`
		Format         models.DocumentFormat       `gorm:"column:format"`
		Title          string                      `gorm:"column:title"`
		ContentMD      string                      `gorm:"column:content_md"`
		RenderStatus   models.DocumentRenderStatus `gorm:"column:render_status"`
		RenderError    string                      `gorm:"column:render_error"`
		RenderedAtRaw  *string                     `gorm:"column:rendered_at"`
		Version        int                         `gorm:"column:version"`
		SourceBlobID   *string                     `gorm:"column:source_blob_id"`
		SourceFileName *string                     `gorm:"column:source_file_name"`
		SourceMimeType *string                     `gorm:"column:source_mime_type"`
		ContentVersion int                         `gorm:"column:content_version"`
		SpaceID        string                      `gorm:"column:space_id"`
		UpdatedAtRaw   string                      `gorm:"column:updated_at"`
	}

	var row documentRow
	if err := r.db.WithContext(ctx).
		Table(tableWithAlias(models.Document{}, "d")).
		Select(selectColumns(
			qualifiedColumn("d", models.DocumentColumns.DocumentID),
			qualifiedColumn("d", models.DocumentColumns.NodeID),
			qualifiedColumn("n", models.NodeColumns.ReaderSlug),
			qualifiedColumn("d", models.DocumentColumns.ThemeID),
			qualifiedColumn("d", models.DocumentColumns.Format),
			qualifiedColumn("d", models.DocumentColumns.Title),
			qualifiedColumn("d", models.DocumentColumns.ContentMD),
			qualifiedColumn("d", models.DocumentColumns.RenderStatus),
			qualifiedColumn("d", models.DocumentColumns.RenderError),
			qualifiedColumn("d", models.DocumentColumns.RenderedAt),
			qualifiedColumn("d", models.DocumentColumns.Version),
			qualifiedColumn("d", models.DocumentColumns.SourceBlobID),
			qualifiedColumn("d", models.DocumentColumns.SourceFileName),
			qualifiedColumn("d", models.DocumentColumns.SourceMimeType),
			qualifiedColumn("d", models.DocumentColumns.ContentVersion),
			qualifiedColumn("d", models.DocumentColumns.UpdatedAt),
			qualifiedColumn("n", models.NodeColumns.SpaceID)+" AS space_id",
		)).
		Joins("JOIN "+tableName(models.Node{})+" AS n ON "+qualifiedColumn("n", models.NodeColumns.NodeID)+" = "+qualifiedColumn("d", models.DocumentColumns.NodeID)).
		Where(qualifiedColumn("d", models.DocumentColumns.DocumentID)+" = ?", strings.TrimSpace(documentID)).
		Take(&row).Error; err != nil {
		return nil, err
	}

	return &WorkspaceDocumentRecord{
		DocumentID:     strings.TrimSpace(row.DocumentID),
		NodeID:         strings.TrimSpace(row.NodeID),
		ReaderSlug:     trimOptionalString(row.ReaderSlug),
		ThemeID:        strings.TrimSpace(row.ThemeID),
		Format:         models.NormalizeDocumentFormat(row.Format),
		Title:          strings.TrimSpace(row.Title),
		ContentMD:      row.ContentMD,
		RenderStatus:   models.NormalizeDocumentRenderStatus(row.RenderStatus),
		RenderError:    strings.TrimSpace(row.RenderError),
		RenderedAt:     parseNullableRecordTime(row.RenderedAtRaw),
		Version:        row.Version,
		SourceBlobID:   trimOptionalString(row.SourceBlobID),
		SourceFileName: trimOptionalString(row.SourceFileName),
		SourceMimeType: trimOptionalString(row.SourceMimeType),
		ContentVersion: normalizeContentVersion(row.ContentVersion, row.Version),
		SpaceID:        strings.TrimSpace(row.SpaceID),
		UpdatedAtRaw:   row.UpdatedAtRaw,
	}, nil
}

func (r *gormWorkspaceRepository) UpdateDocumentIdentifier(
	ctx context.Context,
	params WorkspaceUpdateDocumentIdentifierParams,
) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("workspace repository db is nil")
	}

	documentID := strings.TrimSpace(params.DocumentID)
	if documentID == "" {
		return false, nil
	}

	actorUserID := strings.TrimSpace(params.ActorUserID)
	spaceID := strings.TrimSpace(params.TouchSpace)
	touchedAt := params.TouchedAt
	if touchedAt.IsZero() {
		touchedAt = time.Now().UTC()
	}

	readerSlug := trimOptionalString(params.ReaderSlug)
	updated := false

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		type documentIdentityRow struct {
			DocumentID string `gorm:"column:document_id"`
			NodeID     string `gorm:"column:node_id"`
			SpaceID    string `gorm:"column:space_id"`
		}

		var identity documentIdentityRow
		if err := tx.Table(tableWithAlias(models.Document{}, "d")).
			Select(selectColumns(
				qualifiedColumn("d", models.DocumentColumns.DocumentID),
				qualifiedColumn("d", models.DocumentColumns.NodeID),
				qualifiedColumn("n", models.NodeColumns.SpaceID)+" AS space_id",
			)).
			Joins("JOIN "+tableName(models.Node{})+" AS n ON "+qualifiedColumn("n", models.NodeColumns.NodeID)+" = "+qualifiedColumn("d", models.DocumentColumns.NodeID)).
			Where(qualifiedColumn("d", models.DocumentColumns.DocumentID)+" = ?", documentID).
			Take(&identity).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}

		normalizedNodeID := strings.TrimSpace(identity.NodeID)
		if normalizedNodeID == "" {
			return nil
		}

		nodeUpdates := map[string]any{
			models.NodeColumns.ReaderSlug: readerSlug,
			models.NodeColumns.UpdatedAt:  touchedAt,
		}
		if actorUserID != "" {
			nodeUpdates[models.NodeColumns.UpdatedByUserID] = actorUserID
		}
		nodeUpdateResult := tx.Model(&models.Node{}).
			Where(qualifiedColumn("", models.NodeColumns.NodeID)+" = ?", normalizedNodeID).
			Updates(nodeUpdates)
		if nodeUpdateResult.Error != nil {
			return nodeUpdateResult.Error
		}
		if nodeUpdateResult.RowsAffected == 0 {
			return nil
		}

		documentUpdates := map[string]any{
			models.DocumentColumns.UpdatedAt: touchedAt,
		}
		if actorUserID != "" {
			documentUpdates[models.DocumentColumns.UpdatedByUserID] = actorUserID
		}
		if err := tx.Model(&models.Document{}).
			Where(qualifiedColumn("", models.DocumentColumns.DocumentID)+" = ?", documentID).
			Updates(documentUpdates).Error; err != nil {
			return err
		}

		spaceForTouch := spaceID
		if spaceForTouch == "" {
			spaceForTouch = strings.TrimSpace(identity.SpaceID)
		}
		if spaceForTouch != "" {
			if err := tx.Model(&models.Space{}).
				Where(qualifiedColumn("", models.SpaceColumns.SpaceID)+" = ?", spaceForTouch).
				Update(models.SpaceColumns.UpdatedAt, touchedAt).Error; err != nil {
				return err
			}
		}

		if err := r.enqueueDocumentUpsertInTx(ctx, tx, documentID); err != nil {
			return err
		}

		updated = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return updated, nil
}

func (r *gormWorkspaceRepository) SaveDocument(
	ctx context.Context,
	params WorkspaceSaveDocumentParams,
) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("workspace repository db is nil")
	}

	documentID := strings.TrimSpace(params.DocumentID)
	if documentID == "" || params.BaseVersion <= 0 {
		return false, nil
	}

	touchedAt := params.TouchedAt
	if touchedAt.IsZero() {
		touchedAt = time.Now().UTC()
	}

	actorUserID := strings.TrimSpace(params.ActorUserID)
	nodeID := strings.TrimSpace(params.NodeID)
	spaceID := strings.TrimSpace(params.SpaceID)
	saved := false

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updateResult := tx.Model(&models.Document{}).
			Where(qualifiedColumn("", models.DocumentColumns.DocumentID)+" = ?", documentID).
			Where(qualifiedColumn("", models.DocumentColumns.Version)+" = ?", params.BaseVersion).
			Updates(map[string]any{
				models.DocumentColumns.ContentMD:       params.ContentMD,
				models.DocumentColumns.Version:         params.NextVersion,
				models.DocumentColumns.ContentVersion:  params.NextVersion,
				models.DocumentColumns.UpdatedByUserID: actorUserID,
				models.DocumentColumns.UpdatedAt:       touchedAt,
			})
		if updateResult.Error != nil {
			return updateResult.Error
		}
		if updateResult.RowsAffected == 0 {
			return nil
		}

		saved = true
		if err := r.enqueueDocumentUpsertInTx(ctx, tx, documentID); err != nil {
			return err
		}
		if params.Revision != nil {
			if err := tx.Create(params.Revision).Error; err != nil {
				return err
			}
		}

		if nodeID != "" {
			nodeUpdates := map[string]any{
				models.NodeColumns.UpdatedAt: touchedAt,
			}
			if actorUserID != "" {
				nodeUpdates[models.NodeColumns.UpdatedByUserID] = actorUserID
			}
			if err := tx.Model(&models.Node{}).
				Where(qualifiedColumn("", models.NodeColumns.NodeID)+" = ?", nodeID).
				Updates(nodeUpdates).Error; err != nil {
				return err
			}
		}
		if spaceID != "" {
			if err := tx.Model(&models.Space{}).
				Where(qualifiedColumn("", models.SpaceColumns.SpaceID)+" = ?", spaceID).
				Update(models.SpaceColumns.UpdatedAt, touchedAt).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return saved, nil
}

func (r *gormWorkspaceRepository) SaveOfficeDocument(
	ctx context.Context,
	params WorkspaceSaveOfficeDocumentParams,
) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("workspace repository db is nil")
	}

	documentID := strings.TrimSpace(params.DocumentID)
	sourceBlobID := strings.TrimSpace(params.SourceBlobID)
	if documentID == "" || sourceBlobID == "" || params.BaseContentVersion <= 0 {
		return false, nil
	}

	nextVersion := params.NextVersion
	if nextVersion <= 0 {
		nextVersion = params.BaseContentVersion + 1
	}
	nextContentVersion := params.NextContentVersion
	if nextContentVersion <= 0 {
		nextContentVersion = params.BaseContentVersion + 1
	}
	touchedAt := params.TouchedAt
	if touchedAt.IsZero() {
		touchedAt = time.Now().UTC()
	}

	actorUserID := strings.TrimSpace(params.ActorUserID)
	nodeID := strings.TrimSpace(params.NodeID)
	spaceID := strings.TrimSpace(params.SpaceID)
	saved := false

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updateResult := tx.Model(&models.Document{}).
			Where(qualifiedColumn("", models.DocumentColumns.DocumentID)+" = ?", documentID).
			Where(qualifiedColumn("", models.DocumentColumns.ContentVersion)+" = ?", params.BaseContentVersion).
			Updates(map[string]any{
				models.DocumentColumns.Version:         nextVersion,
				models.DocumentColumns.ContentVersion:  nextContentVersion,
				models.DocumentColumns.SourceBlobID:    sourceBlobID,
				models.DocumentColumns.SourceFileName:  trimOptionalString(pointerString(params.SourceFileName)),
				models.DocumentColumns.SourceMimeType:  trimOptionalString(pointerString(params.SourceMimeType)),
				models.DocumentColumns.RenderStatus:    models.DocumentRenderStatusPending,
				models.DocumentColumns.RenderError:     "",
				models.DocumentColumns.RenderedAt:      nil,
				models.DocumentColumns.UpdatedByUserID: trimOptionalString(pointerString(actorUserID)),
				models.DocumentColumns.UpdatedAt:       touchedAt,
			})
		if updateResult.Error != nil {
			return updateResult.Error
		}
		if updateResult.RowsAffected == 0 {
			return nil
		}

		saved = true
		if err := r.enqueueDocumentUpsertInTx(ctx, tx, documentID); err != nil {
			return err
		}
		if params.FileRevision != nil {
			if err := tx.Create(params.FileRevision).Error; err != nil {
				return err
			}
		}

		if nodeID != "" {
			nodeUpdates := map[string]any{
				models.NodeColumns.UpdatedAt: touchedAt,
			}
			if actorUserID != "" {
				nodeUpdates[models.NodeColumns.UpdatedByUserID] = actorUserID
			}
			if err := tx.Model(&models.Node{}).
				Where(qualifiedColumn("", models.NodeColumns.NodeID)+" = ?", nodeID).
				Updates(nodeUpdates).Error; err != nil {
				return err
			}
		}
		if spaceID != "" {
			if err := tx.Model(&models.Space{}).
				Where(qualifiedColumn("", models.SpaceColumns.SpaceID)+" = ?", spaceID).
				Update(models.SpaceColumns.UpdatedAt, touchedAt).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return saved, nil
}

func (r *gormWorkspaceRepository) ListRevisionsByDocumentID(
	ctx context.Context,
	documentID string,
) ([]WorkspaceRevisionRecord, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("workspace repository db is nil")
	}

	type revisionRow struct {
		DocumentRevisionID string                `gorm:"column:document_revision_id"`
		DocumentID         string                `gorm:"column:document_id"`
		Version            int                   `gorm:"column:version"`
		ContentMD          string                `gorm:"column:content_md"`
		BaseVersion        int                   `gorm:"column:base_version"`
		Source             models.RevisionSource `gorm:"column:source"`
		CreatedAtRaw       string                `gorm:"column:created_at"`
	}

	var rows []revisionRow
	if err := r.db.WithContext(ctx).
		Model(&models.DocumentRevision{}).
		Select(selectColumns(
			qualifiedColumn("", models.DocumentRevisionColumns.DocumentRevisionID),
			qualifiedColumn("", models.DocumentRevisionColumns.DocumentID),
			qualifiedColumn("", models.DocumentRevisionColumns.Version),
			qualifiedColumn("", models.DocumentRevisionColumns.ContentMD),
			qualifiedColumn("", models.DocumentRevisionColumns.BaseVersion),
			qualifiedColumn("", models.DocumentRevisionColumns.Source),
			qualifiedColumn("", models.DocumentRevisionColumns.CreatedAt),
		)).
		Where(qualifiedColumn("", models.DocumentRevisionColumns.DocumentID)+" = ?", strings.TrimSpace(documentID)).
		Order(qualifiedColumn("", models.DocumentRevisionColumns.Version) + " DESC, " + qualifiedColumn("", models.DocumentRevisionColumns.CreatedAt) + " DESC, " + qualifiedColumn("", models.DocumentRevisionColumns.ID) + " DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	revisions := make([]WorkspaceRevisionRecord, 0, len(rows))
	for _, row := range rows {
		revisions = append(revisions, WorkspaceRevisionRecord{
			DocumentRevisionID: strings.TrimSpace(row.DocumentRevisionID),
			DocumentID:         strings.TrimSpace(row.DocumentID),
			Version:            row.Version,
			ContentMD:          row.ContentMD,
			BaseVersion:        row.BaseVersion,
			Source:             row.Source,
			CreatedAtRaw:       row.CreatedAtRaw,
		})
	}

	return revisions, nil
}

func pointerString(value string) *string {
	return &value
}

func (r *gormWorkspaceRepository) enqueueDocumentUpsertInTx(
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

func (r *gormWorkspaceRepository) enqueueDocumentDeleteInTx(
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

func (r *gormWorkspaceRepository) enqueueSpaceRebuildInTx(
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

func trimOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func normalizeOptionalDocumentFormat(value *string) *models.DocumentFormat {
	if value == nil {
		return nil
	}
	normalized := models.NormalizeDocumentFormat(models.DocumentFormat(strings.TrimSpace(*value)))
	return &normalized
}

func normalizeContentVersion(contentVersion int, version int) int {
	switch {
	case contentVersion > 0:
		return contentVersion
	case version > 0:
		return version
	default:
		return 1
	}
}

func optionalStringEqual(left *string, right *string) bool {
	if left == nil && right == nil {
		return true
	}
	if left == nil || right == nil {
		return false
	}
	return strings.TrimSpace(*left) == strings.TrimSpace(*right)
}

func insertWorkspaceNodeIDAt(nodeIDs []string, nodeID string, index int) []string {
	if index < 0 {
		index = 0
	}
	if index > len(nodeIDs) {
		index = len(nodeIDs)
	}

	items := make([]string, 0, len(nodeIDs)+1)
	items = append(items, nodeIDs[:index]...)
	items = append(items, nodeID)
	items = append(items, nodeIDs[index:]...)
	return items
}

func listWorkspaceSiblingNodeIDs(
	tx *gorm.DB,
	spaceID string,
	parentNodeID *string,
	excludeNodeID string,
) ([]string, error) {
	type siblingRow struct {
		NodeID string `gorm:"column:node_id"`
	}

	query := tx.Model(&models.Node{}).
		Select(qualifiedColumn("", models.NodeColumns.NodeID)).
		Where(qualifiedColumn("", models.NodeColumns.SpaceID)+" = ?", strings.TrimSpace(spaceID))
	if parentNodeID == nil {
		query = query.Where(qualifiedColumn("", models.NodeColumns.ParentNodeID) + " IS NULL")
	} else {
		query = query.Where(qualifiedColumn("", models.NodeColumns.ParentNodeID)+" = ?", strings.TrimSpace(*parentNodeID))
	}
	if normalizedExcludeNodeID := strings.TrimSpace(excludeNodeID); normalizedExcludeNodeID != "" {
		query = query.Where(qualifiedColumn("", models.NodeColumns.NodeID)+" <> ?", normalizedExcludeNodeID)
	}

	var rows []siblingRow
	if err := query.Order(qualifiedColumn("", models.NodeColumns.Sort) + " ASC, " + qualifiedColumn("", models.NodeColumns.ID) + " ASC").Find(&rows).Error; err != nil {
		return nil, err
	}

	nodeIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		normalizedNodeID := strings.TrimSpace(row.NodeID)
		if normalizedNodeID == "" {
			continue
		}
		nodeIDs = append(nodeIDs, normalizedNodeID)
	}
	return nodeIDs, nil
}

func resequenceWorkspaceSiblingNodes(
	tx *gorm.DB,
	nodeIDs []string,
	parentNodeID *string,
	actorUserID string,
	touchedAt time.Time,
) error {
	for index, nodeID := range nodeIDs {
		updateValues := map[string]any{
			models.NodeColumns.ParentNodeID: parentNodeID,
			models.NodeColumns.Sort:         index + 1,
			models.NodeColumns.UpdatedAt:    touchedAt,
		}
		if actorUserID != "" {
			updateValues[models.NodeColumns.UpdatedByUserID] = actorUserID
		}

		updateResult := tx.Model(&models.Node{}).
			Where(qualifiedColumn("", models.NodeColumns.NodeID)+" = ?", nodeID).
			Updates(updateValues)
		if updateResult.Error != nil {
			return updateResult.Error
		}
		if updateResult.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
	}
	return nil
}

func ensureWorkspaceMoveParentPathValid(
	tx *gorm.DB,
	spaceID string,
	nodeID string,
	targetParentNodeID string,
) error {
	type parentRow struct {
		NodeID       string  `gorm:"column:node_id"`
		SpaceID      string  `gorm:"column:space_id"`
		ParentNodeID *string `gorm:"column:parent_node_id"`
	}

	visitedNodeIDs := make(map[string]struct{})
	currentNodeID := strings.TrimSpace(targetParentNodeID)
	for currentNodeID != "" {
		if currentNodeID == strings.TrimSpace(nodeID) {
			return ErrWorkspaceMoveCycleDetected
		}
		if _, duplicated := visitedNodeIDs[currentNodeID]; duplicated {
			// 数据异常时直接阻止移动，避免陷入循环链。
			return ErrWorkspaceMoveCycleDetected
		}
		visitedNodeIDs[currentNodeID] = struct{}{}

		var parent parentRow
		if err := tx.Model(&models.Node{}).
			Select(selectColumns(
				qualifiedColumn("", models.NodeColumns.NodeID),
				qualifiedColumn("", models.NodeColumns.SpaceID),
				qualifiedColumn("", models.NodeColumns.ParentNodeID),
			)).
			Where(qualifiedColumn("", models.NodeColumns.NodeID)+" = ?", currentNodeID).
			Take(&parent).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrWorkspaceMoveTargetParentNotFound
			}
			return err
		}

		if strings.TrimSpace(parent.SpaceID) != strings.TrimSpace(spaceID) {
			return ErrWorkspaceMoveTargetParentNotInSameSpace
		}

		nextParentNodeID := trimOptionalString(parent.ParentNodeID)
		if nextParentNodeID == nil {
			break
		}
		currentNodeID = strings.TrimSpace(*nextParentNodeID)
	}
	return nil
}
