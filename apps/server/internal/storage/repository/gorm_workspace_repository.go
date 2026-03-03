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
	if err := r.db.WithContext(ctx).
		Table("spaces AS s").
		Select("s.space_id", "s.name", "s.created_at", "s.updated_at").
		Where("s.status = ? AND s.deleted_at IS NULL", models.EntityStatusActive).
		Where(
			"("+
				"EXISTS (SELECT 1 FROM user_admin_roles AS uar WHERE uar.user_id = ? AND uar.role = ?) OR "+
				"s.owner_user_id = ? OR "+
				"EXISTS (SELECT 1 FROM space_admin_scopes AS sas WHERE sas.space_id = s.space_id AND sas.user_id = ?) OR "+
				"EXISTS (SELECT 1 FROM space_members AS sm WHERE sm.space_id = s.space_id AND sm.user_id = ? AND sm.role IN ?)"+
				")",
			userID,
			models.AdminRolePlatformAdmin,
			userID,
			userID,
			userID,
			[]models.Role{models.RoleOwner, models.RoleCollaborator},
		).
		Order("s.updated_at DESC, s.id DESC").
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
		Table("space_categories").
		Select("category_id", "name").
		Where("is_default = ?", true).
		Order("id ASC").
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
		Table("spaces AS s").
		Select(
			"s.space_id",
			"s.owner_user_id",
			"s.visibility",
			"s.status",
			"s.deleted_at",
			"CASE WHEN uar.user_id IS NULL THEN 0 ELSE 1 END AS is_platform_admin",
			"CASE WHEN sas.id IS NULL THEN 0 ELSE 1 END AS has_space_admin_scope",
			"sm.role AS member_role",
		).
		Joins(
			"LEFT JOIN user_admin_roles AS uar ON uar.user_id = ? AND uar.role = ?",
			normalizedActorUserID,
			models.AdminRolePlatformAdmin,
		).
		Joins(
			"LEFT JOIN space_admin_scopes AS sas ON sas.user_id = ? AND sas.space_id = s.space_id",
			normalizedActorUserID,
		).
		Joins(
			"LEFT JOIN space_members AS sm ON sm.user_id = ? AND sm.space_id = s.space_id",
			normalizedActorUserID,
		).
		Where("s.space_id = ?", normalizedSpaceID).
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
		SpaceID            string          `gorm:"column:space_id"`
		ParentNodeID       *string         `gorm:"column:parent_node_id"`
		Type               models.NodeType `gorm:"column:type"`
		Title              string          `gorm:"column:title"`
		Sort               int             `gorm:"column:sort"`
		DocumentVisibility *string         `gorm:"column:document_visibility"`
	}

	var rows []nodeRow
	if err := r.db.WithContext(ctx).
		Table("nodes AS n").
		Select(
			"n.node_id",
			"d.document_id AS document_id",
			"n.space_id",
			"n.parent_node_id",
			"n.type",
			"n.title",
			"n.sort",
			"d.visibility AS document_visibility",
		).
		Joins("LEFT JOIN documents AS d ON d.node_id = n.node_id").
		Where("n.space_id = ?", strings.TrimSpace(spaceID)).
		Where(
			"(n.type <> ? OR (d.document_id IS NOT NULL AND d.deleted_at IS NULL AND d.status <> ?))",
			models.NodeTypeDoc,
			models.EntityStatusDeleted,
		).
		Order("n.parent_node_id ASC, n.sort ASC, n.id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]WorkspaceTreeNodeRecord, 0, len(rows))
	for _, row := range rows {
		items = append(items, WorkspaceTreeNodeRecord{
			NodeID:             strings.TrimSpace(row.NodeID),
			DocumentID:         trimOptionalString(row.DocumentID),
			SpaceID:            strings.TrimSpace(row.SpaceID),
			ParentNodeID:       trimOptionalString(row.ParentNodeID),
			Type:               row.Type,
			Title:              strings.TrimSpace(row.Title),
			Sort:               row.Sort,
			DocumentVisibility: trimOptionalString(row.DocumentVisibility),
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
		Type         models.NodeType `gorm:"column:type"`
		Title        string          `gorm:"column:title"`
		Sort         int             `gorm:"column:sort"`
	}

	var row nodeRow
	if err := r.db.WithContext(ctx).
		Table("nodes").
		Select("node_id", "space_id", "parent_node_id", "type", "title", "sort").
		Where("node_id = ?", strings.TrimSpace(nodeID)).
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
		Table("nodes").
		Select("COALESCE(MAX(sort), 0) AS value").
		Where("space_id = ?", strings.TrimSpace(spaceID))
	if parentNodeID == nil {
		query = query.Where("parent_node_id IS NULL")
	} else {
		query = query.Where("parent_node_id = ?", strings.TrimSpace(*parentNodeID))
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
		if spaceID == "" {
			return nil
		}
		return tx.Table("spaces").
			Where("space_id = ?", spaceID).
			Update("updated_at", touchedAt).Error
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
		updateValues["updated_by_user_id"] = actorUserID
	}

	touchedAt := params.TouchedAt
	if touchedAt.IsZero() {
		touchedAt = time.Now().UTC()
	}

	spaceID := strings.TrimSpace(params.TouchSpace)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updateTx := tx.Table("nodes").
			Where("node_id = ?", nodeID).
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
				"title":      strings.TrimSpace(*params.DocumentTitle),
				"updated_at": touchedAt,
			}
			if actorUserID != "" {
				documentUpdates["updated_by_user_id"] = actorUserID
			}
			if err := tx.Table("documents").
				Where("node_id = ?", nodeID).
				Updates(documentUpdates).Error; err != nil {
				return err
			}
			var identity documentIdentityRow
			if err := tx.Table("documents").
				Select("document_id").
				Where("node_id = ?", nodeID).
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
		return tx.Table("spaces").
			Where("space_id = ?", spaceID).
			Update("updated_at", touchedAt).Error
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
		if err := tx.Table("nodes").
			Select("node_id", "space_id", "parent_node_id").
			Where("node_id = ?", nodeID).
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

		if err := tx.Table("spaces").
			Where("space_id = ?", spaceID).
			Update("updated_at", touchedAt).Error; err != nil {
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
		type deletedNodeSnapshot struct {
			SpaceID string          `gorm:"column:space_id"`
			Type    models.NodeType `gorm:"column:type"`
		}
		type documentIdentityRow struct {
			DocumentID string `gorm:"column:document_id"`
		}
		var snapshot deletedNodeSnapshot
		if err := tx.Table("nodes").
			Select("space_id", "type").
			Where("node_id = ?", normalizedNodeID).
			Take(&snapshot).Error; err != nil {
			return err
		}
		documentID := normalizedNodeID
		if snapshot.Type == models.NodeTypeDoc {
			var identity documentIdentityRow
			if err := tx.Table("documents").
				Select("document_id").
				Where("node_id = ?", normalizedNodeID).
				Take(&identity).Error; err == nil {
				if strings.TrimSpace(identity.DocumentID) != "" {
					documentID = strings.TrimSpace(identity.DocumentID)
				}
			} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}

		deleteResult := tx.Where("node_id = ?", normalizedNodeID).Delete(&models.Node{})
		if deleteResult.Error != nil {
			return deleteResult.Error
		}
		if deleteResult.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		if snapshot.Type == models.NodeTypeDoc {
			if err := r.enqueueDocumentDeleteInTx(ctx, tx, documentID); err != nil {
				return err
			}
		} else {
			spaceForRebuild := normalizedSpaceID
			if strings.TrimSpace(spaceForRebuild) == "" {
				spaceForRebuild = strings.TrimSpace(snapshot.SpaceID)
			}
			if err := r.enqueueSpaceRebuildInTx(ctx, tx, spaceForRebuild); err != nil {
				return err
			}
		}

		if normalizedSpaceID == "" {
			return nil
		}
		return tx.Table("spaces").
			Where("space_id = ?", normalizedSpaceID).
			Update("updated_at", touchedAt).Error
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
		DocumentID   string `gorm:"column:document_id"`
		NodeID       string `gorm:"column:node_id"`
		ThemeID      string `gorm:"column:theme_id"`
		Title        string `gorm:"column:title"`
		ContentMD    string `gorm:"column:content_md"`
		Version      int    `gorm:"column:version"`
		SpaceID      string `gorm:"column:space_id"`
		UpdatedAtRaw string `gorm:"column:updated_at"`
	}

	var row documentRow
	if err := r.db.WithContext(ctx).
		Table("documents AS d").
		Select(
			"d.document_id",
			"d.node_id",
			"d.theme_id",
			"d.title",
			"d.content_md",
			"d.version",
			"d.updated_at",
			"n.space_id AS space_id",
		).
		Joins("JOIN nodes AS n ON n.node_id = d.node_id").
		Where("d.document_id = ?", strings.TrimSpace(documentID)).
		Take(&row).Error; err != nil {
		return nil, err
	}

	return &WorkspaceDocumentRecord{
		DocumentID:   strings.TrimSpace(row.DocumentID),
		NodeID:       strings.TrimSpace(row.NodeID),
		ThemeID:      strings.TrimSpace(row.ThemeID),
		Title:        strings.TrimSpace(row.Title),
		ContentMD:    row.ContentMD,
		Version:      row.Version,
		SpaceID:      strings.TrimSpace(row.SpaceID),
		UpdatedAtRaw: row.UpdatedAtRaw,
	}, nil
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
		updateResult := tx.Table("documents").
			Where("document_id = ? AND version = ?", documentID, params.BaseVersion).
			Updates(map[string]any{
				"content_md":         params.ContentMD,
				"version":            params.NextVersion,
				"updated_by_user_id": actorUserID,
				"updated_at":         touchedAt,
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
				"updated_at": touchedAt,
			}
			if actorUserID != "" {
				nodeUpdates["updated_by_user_id"] = actorUserID
			}
			if err := tx.Table("nodes").
				Where("node_id = ?", nodeID).
				Updates(nodeUpdates).Error; err != nil {
				return err
			}
		}
		if spaceID != "" {
			if err := tx.Table("spaces").
				Where("space_id = ?", spaceID).
				Update("updated_at", touchedAt).Error; err != nil {
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
		Table("document_revisions").
		Select(
			"document_revision_id",
			"document_id",
			"version",
			"content_md",
			"base_version",
			"source",
			"created_at",
		).
		Where("document_id = ?", strings.TrimSpace(documentID)).
		Order("version DESC, created_at DESC, id DESC").
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

	query := tx.Table("nodes").
		Select("node_id").
		Where("space_id = ?", strings.TrimSpace(spaceID))
	if parentNodeID == nil {
		query = query.Where("parent_node_id IS NULL")
	} else {
		query = query.Where("parent_node_id = ?", strings.TrimSpace(*parentNodeID))
	}
	if normalizedExcludeNodeID := strings.TrimSpace(excludeNodeID); normalizedExcludeNodeID != "" {
		query = query.Where("node_id <> ?", normalizedExcludeNodeID)
	}

	var rows []siblingRow
	if err := query.Order("sort ASC, id ASC").Find(&rows).Error; err != nil {
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
			"parent_node_id": parentNodeID,
			"sort":           index + 1,
			"updated_at":     touchedAt,
		}
		if actorUserID != "" {
			updateValues["updated_by_user_id"] = actorUserID
		}

		updateResult := tx.Table("nodes").
			Where("node_id = ?", nodeID).
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
		if err := tx.Table("nodes").
			Select("node_id", "space_id", "parent_node_id").
			Where("node_id = ?", currentNodeID).
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
