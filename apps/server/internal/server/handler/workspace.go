package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"
)

const (
	defaultWorkspaceCategoryID   = "01jmf4v2x7m7f1m6qv5kh0t2mn"
	defaultWorkspaceCategoryName = "未分类"
	maxWorkspaceSpaceNameLength  = 64
)

var errWorkspaceDocumentVersionConflict = errors.New("workspace document version conflict")

type workspaceHandler struct {
	db                *gorm.DB
	authService       *service.AuthService
	visibilityService *service.VisibilityService
}

type workspaceSpaceResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type workspaceTreeNodeResponse struct {
	ID       string                      `json:"id"`
	SpaceID  string                      `json:"spaceId"`
	ParentID *string                     `json:"parentId"`
	Type     models.NodeType             `json:"type"`
	Title    string                      `json:"title"`
	Sort     int                         `json:"sort"`
	Children []workspaceTreeNodeResponse `json:"children"`
}

type workspaceDocumentResponse struct {
	ID        string `json:"id"`
	NodeID    string `json:"nodeId"`
	ThemeID   string `json:"themeId"`
	Title     string `json:"title"`
	ContentMD string `json:"contentMd"`
	Version   int    `json:"version"`
	UpdatedAt string `json:"updatedAt"`
}

type workspaceRevisionResponse struct {
	ID          string `json:"id"`
	DocumentID  string `json:"documentId"`
	Version     int    `json:"version"`
	ContentMD   string `json:"contentMd"`
	BaseVersion int    `json:"baseVersion"`
	CreatedAt   string `json:"createdAt"`
	Source      string `json:"source"`
}

type workspaceDocumentRow struct {
	DocumentID   string `gorm:"column:document_id"`
	NodeID       string `gorm:"column:node_id"`
	ThemeID      string `gorm:"column:theme_id"`
	Title        string `gorm:"column:title"`
	ContentMD    string `gorm:"column:content_md"`
	Version      int    `gorm:"column:version"`
	SpaceID      string `gorm:"column:space_id"`
	UpdatedAtRaw string `gorm:"column:updated_at"`
}

type createWorkspaceSpaceRequest struct {
	Name string `json:"name" binding:"required"`
}

type createWorkspaceNodeRequest struct {
	ParentID *string         `json:"parentId"`
	Type     models.NodeType `json:"type" binding:"required"`
	Title    string          `json:"title" binding:"required"`
}

type createWorkspaceNodeResponse struct {
	NodeID string `json:"nodeId"`
	DocID  string `json:"docId,omitempty"`
}

type updateWorkspaceNodeRequest struct {
	Title    *string `json:"title"`
	ParentID *string `json:"parentId"`
	Sort     *int    `json:"sort"`
}

type saveWorkspaceDocumentRequest struct {
	ContentMD   string `json:"contentMd"`
	BaseVersion int    `json:"baseVersion" binding:"required"`
}

type saveWorkspaceDocumentResponse struct {
	Document workspaceDocumentResponse `json:"document"`
}

type workspaceTreeNode struct {
	ID       string
	SpaceID  string
	ParentID *string
	Type     models.NodeType
	Title    string
	Sort     int
	Children []*workspaceTreeNode
}

// NewWorkspaceHandler 创建编辑器工作区处理器。
func NewWorkspaceHandler(
	db *gorm.DB,
	authService *service.AuthService,
	visibilityService *service.VisibilityService,
) *workspaceHandler {
	return &workspaceHandler{
		db:                db,
		authService:       authService,
		visibilityService: visibilityService,
	}
}

// ListSpaces 返回当前登录用户可进入编辑器的空间列表。
func (h *workspaceHandler) ListSpaces(c *gin.Context) {
	actorUserID, ok := h.requireActorUserID(c)
	if !ok {
		return
	}
	if h == nil || h.db == nil {
		response.InternalError(c)
		return
	}

	type spaceRow struct {
		SpaceID      string `gorm:"column:space_id"`
		Name         string `gorm:"column:name"`
		CreatedAtRaw string `gorm:"column:created_at"`
		UpdatedAtRaw string `gorm:"column:updated_at"`
	}

	isPlatformAdmin, err := h.isPlatformAdmin(c.Request.Context(), actorUserID)
	if err != nil {
		response.InternalError(c)
		return
	}

	query := h.db.WithContext(c.Request.Context()).
		Table("spaces AS s").
		Distinct("s.space_id", "s.name", "s.created_at", "s.updated_at").
		Select("s.space_id", "s.name", "s.created_at", "s.updated_at").
		Where("s.status = ? AND s.deleted_at IS NULL", models.EntityStatusActive)

	if !isPlatformAdmin {
		query = query.
			Joins("LEFT JOIN space_members AS sm ON sm.space_id = s.space_id AND sm.user_id = ?", actorUserID).
			Joins("LEFT JOIN space_admin_scopes AS sas ON sas.space_id = s.space_id AND sas.user_id = ?", actorUserID).
			Where("s.owner_user_id = ? OR sm.id IS NOT NULL OR sas.id IS NOT NULL", actorUserID)
	}

	var rows []spaceRow
	if err := query.
		Order("s.updated_at DESC, s.id DESC").
		Find(&rows).Error; err != nil {
		response.InternalError(c)
		return
	}

	items := make([]workspaceSpaceResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, workspaceSpaceResponse{
			ID:        strings.TrimSpace(row.SpaceID),
			Name:      strings.TrimSpace(row.Name),
			CreatedAt: formatWorkspaceTime(row.CreatedAtRaw),
			UpdatedAt: formatWorkspaceTime(row.UpdatedAtRaw),
		})
	}
	response.JSON(c, http.StatusOK, items)
}

// CreateSpace 创建编辑器工作区空间。
func (h *workspaceHandler) CreateSpace(c *gin.Context) {
	actorUserID, ok := h.requireActorUserID(c)
	if !ok {
		return
	}
	if h == nil || h.db == nil {
		response.InternalError(c)
		return
	}

	var req createWorkspaceSpaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "space name is required")
		return
	}

	spaceName := strings.TrimSpace(req.Name)
	if spaceName == "" || len([]rune(spaceName)) > maxWorkspaceSpaceNameLength {
		response.Error(c, http.StatusBadRequest, "INVALID_SPACE_NAME", "invalid space name")
		return
	}

	type defaultCategoryRow struct {
		CategoryID string `gorm:"column:category_id"`
		Name       string `gorm:"column:name"`
	}
	defaultCategory := defaultCategoryRow{
		CategoryID: defaultWorkspaceCategoryID,
		Name:       defaultWorkspaceCategoryName,
	}
	if err := h.db.WithContext(c.Request.Context()).
		Table("space_categories").
		Select("category_id", "name").
		Where("is_default = ?", true).
		Order("id ASC").
		Take(&defaultCategory).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		response.InternalError(c)
		return
	}
	if strings.TrimSpace(defaultCategory.CategoryID) == "" {
		defaultCategory.CategoryID = defaultWorkspaceCategoryID
	}
	if strings.TrimSpace(defaultCategory.Name) == "" {
		defaultCategory.Name = defaultWorkspaceCategoryName
	}

	now := time.Now().UTC()
	space := &models.Space{
		SpaceID:     strings.ToLower(ulid.Make().String()),
		Name:        spaceName,
		Description: "",
		CategoryID:  strings.TrimSpace(defaultCategory.CategoryID),
		Category:    strings.TrimSpace(defaultCategory.Name),
		OwnerUserID: actorUserID,
		Visibility:  models.VisibilityMember,
		Status:      models.EntityStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := h.db.WithContext(c.Request.Context()).Create(space).Error; err != nil {
		response.InternalError(c)
		return
	}

	response.JSON(c, http.StatusOK, workspaceSpaceResponse{
		ID:        space.SpaceID,
		Name:      space.Name,
		CreatedAt: now.Format(time.RFC3339Nano),
		UpdatedAt: now.Format(time.RFC3339Nano),
	})
}

// GetTree 返回空间目录树。
func (h *workspaceHandler) GetTree(c *gin.Context) {
	actorUserID, ok := h.requireActorUserID(c)
	if !ok {
		return
	}
	if h == nil || h.db == nil {
		response.InternalError(c)
		return
	}

	spaceID := strings.TrimSpace(c.Param("spaceId"))
	if spaceID == "" {
		response.Error(c, http.StatusBadRequest, "INVALID_SPACE_ID", "space id is required")
		return
	}

	if err := h.ensureSpaceReadable(c.Request.Context(), spaceID, actorUserID); err != nil {
		switch {
		case errors.Is(err, service.ErrSpaceNotFound):
			response.Error(c, http.StatusNotFound, "SPACE_NOT_FOUND", "space not found")
		case errors.Is(err, service.ErrViewerLoginRequired):
			response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "login required")
		case errors.Is(err, service.ErrSpaceAccessDenied):
			response.Error(c, http.StatusForbidden, "FORBIDDEN", "insufficient space permission")
		default:
			response.InternalError(c)
		}
		return
	}

	type nodeRow struct {
		NodeID       string          `gorm:"column:node_id"`
		SpaceID      string          `gorm:"column:space_id"`
		ParentNodeID *string         `gorm:"column:parent_node_id"`
		Type         models.NodeType `gorm:"column:type"`
		Title        string          `gorm:"column:title"`
		Sort         int             `gorm:"column:sort"`
	}

	var rows []nodeRow
	if err := h.db.WithContext(c.Request.Context()).
		Table("nodes").
		Select("node_id", "space_id", "parent_node_id", "type", "title", "sort").
		Where("space_id = ?", spaceID).
		Order("parent_node_id ASC, sort ASC, id ASC").
		Find(&rows).Error; err != nil {
		response.InternalError(c)
		return
	}

	treeNodes := make(map[string]*workspaceTreeNode, len(rows))
	for _, row := range rows {
		nodeID := strings.TrimSpace(row.NodeID)
		if nodeID == "" {
			continue
		}
		treeNodes[nodeID] = &workspaceTreeNode{
			ID:       nodeID,
			SpaceID:  strings.TrimSpace(row.SpaceID),
			ParentID: normalizeOptionalString(row.ParentNodeID),
			Type:     normalizeWorkspaceNodeType(row.Type),
			Title:    strings.TrimSpace(row.Title),
			Sort:     row.Sort,
			Children: make([]*workspaceTreeNode, 0),
		}
	}

	roots := make([]*workspaceTreeNode, 0)
	for _, row := range rows {
		nodeID := strings.TrimSpace(row.NodeID)
		node, ok := treeNodes[nodeID]
		if !ok {
			continue
		}
		parentID := normalizeOptionalString(row.ParentNodeID)
		if parentID == nil {
			roots = append(roots, node)
			continue
		}
		parentNode, exists := treeNodes[*parentID]
		if !exists {
			roots = append(roots, node)
			continue
		}
		parentNode.Children = append(parentNode.Children, node)
	}

	response.JSON(c, http.StatusOK, mapWorkspaceTreeResponses(roots))
}

// CreateNode 在指定空间下创建目录节点（文档或文件夹）。
func (h *workspaceHandler) CreateNode(c *gin.Context) {
	actorUserID, ok := h.requireActorUserID(c)
	if !ok {
		return
	}
	if h == nil || h.db == nil {
		response.InternalError(c)
		return
	}

	spaceID := strings.TrimSpace(c.Param("spaceId"))
	if spaceID == "" {
		response.Error(c, http.StatusBadRequest, "INVALID_SPACE_ID", "space id is required")
		return
	}
	if err := h.ensureSpaceWritable(c.Request.Context(), spaceID, actorUserID); err != nil {
		switch {
		case errors.Is(err, service.ErrSpaceNotFound):
			response.Error(c, http.StatusNotFound, "SPACE_NOT_FOUND", "space not found")
		case errors.Is(err, service.ErrSpaceAccessDenied):
			response.Error(c, http.StatusForbidden, "FORBIDDEN", "insufficient space permission")
		default:
			response.InternalError(c)
		}
		return
	}

	var req createWorkspaceNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid create node request")
		return
	}

	if req.Type != models.NodeTypeFolder && req.Type != models.NodeTypeDoc {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid node type")
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		if req.Type == models.NodeTypeFolder {
			title = "未命名目录"
		} else {
			title = "未命名文档"
		}
	}

	parentID := normalizeOptionalString(req.ParentID)
	if parentID != nil {
		type parentRow struct {
			SpaceID string `gorm:"column:space_id"`
		}
		var parent parentRow
		if err := h.db.WithContext(c.Request.Context()).
			Table("nodes").
			Select("space_id").
			Where("node_id = ?", *parentID).
			Take(&parent).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "parent node not found")
				return
			}
			response.InternalError(c)
			return
		}
		if strings.TrimSpace(parent.SpaceID) != spaceID {
			response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "parent node not in target space")
			return
		}
	}

	type maxSortRow struct {
		Value int `gorm:"column:value"`
	}
	maxSort := maxSortRow{Value: 0}
	sortQuery := h.db.WithContext(c.Request.Context()).
		Table("nodes").
		Select("COALESCE(MAX(sort), 0) AS value").
		Where("space_id = ?", spaceID)
	if parentID == nil {
		sortQuery = sortQuery.Where("parent_node_id IS NULL")
	} else {
		sortQuery = sortQuery.Where("parent_node_id = ?", *parentID)
	}
	if err := sortQuery.Take(&maxSort).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		response.InternalError(c)
		return
	}

	now := time.Now().UTC()
	nodeID := strings.ToLower(ulid.Make().String())
	node := &models.Node{
		NodeID:       nodeID,
		SpaceID:      spaceID,
		ParentNodeID: parentID,
		Type:         req.Type,
		Title:        title,
		Sort:         maxSort.Value + 1,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	responseBody := createWorkspaceNodeResponse{
		NodeID: node.NodeID,
	}

	if err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(node).Error; err != nil {
			return err
		}
		if req.Type == models.NodeTypeDoc {
			documentID := node.NodeID
			doc := &models.Document{
				DocumentID:      documentID,
				NodeID:          node.NodeID,
				ThemeID:         "default",
				Visibility:      models.VisibilityMember,
				Status:          models.EntityStatusActive,
				Title:           title,
				ContentMD:       "",
				Version:         1,
				UpdatedByUserID: &actorUserID,
				CreatedAt:       now,
				UpdatedAt:       now,
			}
			if err := tx.Create(doc).Error; err != nil {
				return err
			}
			revision := &models.DocumentRevision{
				DocumentRevisionID: strings.ToLower(ulid.Make().String()),
				DocumentID:         documentID,
				Version:            1,
				ContentMD:          "",
				BaseVersion:        0,
				EditorUserID:       &actorUserID,
				Source:             models.RevisionSourceRemote,
				CreatedAt:          now,
			}
			if err := tx.Create(revision).Error; err != nil {
				return err
			}
			responseBody.DocID = documentID
		}
		return tx.Table("spaces").
			Where("space_id = ?", spaceID).
			Update("updated_at", now).Error
	}); err != nil {
		response.InternalError(c)
		return
	}

	response.JSON(c, http.StatusOK, responseBody)
}

// UpdateNode 更新目录节点标题/父级/排序。
func (h *workspaceHandler) UpdateNode(c *gin.Context) {
	actorUserID, ok := h.requireActorUserID(c)
	if !ok {
		return
	}
	if h == nil || h.db == nil {
		response.InternalError(c)
		return
	}

	nodeID := strings.TrimSpace(c.Param("nodeId"))
	if nodeID == "" {
		response.Error(c, http.StatusBadRequest, "INVALID_NODE_ID", "node id is required")
		return
	}

	var req updateWorkspaceNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid update node request")
		return
	}
	if req.Title == nil && req.ParentID == nil && req.Sort == nil {
		response.JSON(c, http.StatusOK, struct{}{})
		return
	}

	type nodeRow struct {
		NodeID  string          `gorm:"column:node_id"`
		SpaceID string          `gorm:"column:space_id"`
		Type    models.NodeType `gorm:"column:type"`
		Title   string          `gorm:"column:title"`
		Sort    int             `gorm:"column:sort"`
	}
	var node nodeRow
	if err := h.db.WithContext(c.Request.Context()).
		Table("nodes").
		Select("node_id", "space_id", "type", "title", "sort").
		Where("node_id = ?", nodeID).
		Take(&node).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Error(c, http.StatusNotFound, "NODE_NOT_FOUND", "node not found")
			return
		}
		response.InternalError(c)
		return
	}

	if err := h.ensureSpaceWritable(c.Request.Context(), strings.TrimSpace(node.SpaceID), actorUserID); err != nil {
		switch {
		case errors.Is(err, service.ErrSpaceNotFound):
			response.Error(c, http.StatusNotFound, "SPACE_NOT_FOUND", "space not found")
		case errors.Is(err, service.ErrSpaceAccessDenied):
			response.Error(c, http.StatusForbidden, "FORBIDDEN", "insufficient space permission")
		default:
			response.InternalError(c)
		}
		return
	}

	updateValues := map[string]any{}
	nextTitle := strings.TrimSpace(node.Title)
	if req.Title != nil {
		resolvedTitle := strings.TrimSpace(*req.Title)
		if resolvedTitle == "" {
			if node.Type == models.NodeTypeFolder {
				resolvedTitle = "未命名目录"
			} else {
				resolvedTitle = "未命名文档"
			}
		}
		nextTitle = resolvedTitle
		updateValues["title"] = resolvedTitle
	}
	if req.ParentID != nil {
		parentID := normalizeOptionalString(req.ParentID)
		if parentID != nil && *parentID == nodeID {
			response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "node cannot be its own parent")
			return
		}
		if parentID != nil {
			type parentRow struct {
				SpaceID string `gorm:"column:space_id"`
			}
			var parent parentRow
			if err := h.db.WithContext(c.Request.Context()).
				Table("nodes").
				Select("space_id").
				Where("node_id = ?", *parentID).
				Take(&parent).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "parent node not found")
					return
				}
				response.InternalError(c)
				return
			}
			if strings.TrimSpace(parent.SpaceID) != strings.TrimSpace(node.SpaceID) {
				response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "parent node not in target space")
				return
			}
			updateValues["parent_node_id"] = *parentID
		} else {
			updateValues["parent_node_id"] = nil
		}
	}
	if req.Sort != nil {
		updateValues["sort"] = *req.Sort
	}

	now := time.Now().UTC()
	updateValues["updated_at"] = now

	if err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("nodes").
			Where("node_id = ?", nodeID).
			Updates(updateValues).Error; err != nil {
			return err
		}
		if node.Type == models.NodeTypeDoc && req.Title != nil {
			if err := tx.Table("documents").
				Where("node_id = ?", nodeID).
				Updates(map[string]any{
					"title":      nextTitle,
					"updated_at": now,
				}).Error; err != nil {
				return err
			}
		}
		return tx.Table("spaces").
			Where("space_id = ?", strings.TrimSpace(node.SpaceID)).
			Update("updated_at", now).Error
	}); err != nil {
		response.InternalError(c)
		return
	}

	response.JSON(c, http.StatusOK, struct{}{})
}

// DeleteNode 删除目录节点及其子树。
func (h *workspaceHandler) DeleteNode(c *gin.Context) {
	actorUserID, ok := h.requireActorUserID(c)
	if !ok {
		return
	}
	if h == nil || h.db == nil {
		response.InternalError(c)
		return
	}

	nodeID := strings.TrimSpace(c.Param("nodeId"))
	if nodeID == "" {
		response.Error(c, http.StatusBadRequest, "INVALID_NODE_ID", "node id is required")
		return
	}

	type nodeRow struct {
		SpaceID string `gorm:"column:space_id"`
	}
	var node nodeRow
	if err := h.db.WithContext(c.Request.Context()).
		Table("nodes").
		Select("space_id").
		Where("node_id = ?", nodeID).
		Take(&node).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Error(c, http.StatusNotFound, "NODE_NOT_FOUND", "node not found")
			return
		}
		response.InternalError(c)
		return
	}

	spaceID := strings.TrimSpace(node.SpaceID)
	if err := h.ensureSpaceWritable(c.Request.Context(), spaceID, actorUserID); err != nil {
		switch {
		case errors.Is(err, service.ErrSpaceNotFound):
			response.Error(c, http.StatusNotFound, "SPACE_NOT_FOUND", "space not found")
		case errors.Is(err, service.ErrSpaceAccessDenied):
			response.Error(c, http.StatusForbidden, "FORBIDDEN", "insufficient space permission")
		default:
			response.InternalError(c)
		}
		return
	}

	now := time.Now().UTC()
	if err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		deleteResult := tx.Where("node_id = ?", nodeID).Delete(&models.Node{})
		if deleteResult.Error != nil {
			return deleteResult.Error
		}
		if deleteResult.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return tx.Table("spaces").
			Where("space_id = ?", spaceID).
			Update("updated_at", now).Error
	}); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Error(c, http.StatusNotFound, "NODE_NOT_FOUND", "node not found")
			return
		}
		response.InternalError(c)
		return
	}

	response.JSON(c, http.StatusOK, struct{}{})
}

// SaveDocument 保存文档内容（带版本冲突检测）。
func (h *workspaceHandler) SaveDocument(c *gin.Context) {
	actorUserID, ok := h.requireActorUserID(c)
	if !ok {
		return
	}
	if h == nil || h.db == nil {
		response.InternalError(c)
		return
	}

	documentID := strings.TrimSpace(c.Param("docId"))
	if documentID == "" {
		response.Error(c, http.StatusBadRequest, "INVALID_DOCUMENT_ID", "document id is required")
		return
	}

	var req saveWorkspaceDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid save document request")
		return
	}
	if req.BaseVersion <= 0 {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "baseVersion is required")
		return
	}

	fetchDocument := func(tx *gorm.DB) (workspaceDocumentRow, error) {
		var row workspaceDocumentRow
		err := tx.
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
			Where("d.document_id = ?", documentID).
			Take(&row).Error
		return row, err
	}

	current, err := fetchDocument(h.db.WithContext(c.Request.Context()))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Error(c, http.StatusNotFound, "DOCUMENT_NOT_FOUND", "document not found")
			return
		}
		response.InternalError(c)
		return
	}

	spaceID := strings.TrimSpace(current.SpaceID)
	if err := h.ensureSpaceWritable(c.Request.Context(), spaceID, actorUserID); err != nil {
		switch {
		case errors.Is(err, service.ErrSpaceNotFound):
			response.Error(c, http.StatusNotFound, "SPACE_NOT_FOUND", "space not found")
		case errors.Is(err, service.ErrSpaceAccessDenied):
			response.Error(c, http.StatusForbidden, "FORBIDDEN", "insufficient space permission")
		default:
			response.InternalError(c)
		}
		return
	}

	if current.Version != req.BaseVersion {
		h.writeDocumentVersionConflict(c, mapWorkspaceDocumentResponse(current))
		return
	}

	now := time.Now().UTC()
	nextVersion := current.Version + 1
	var latest workspaceDocumentResponse

	if err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		updateResult := tx.Table("documents").
			Where("document_id = ? AND version = ?", documentID, req.BaseVersion).
			Updates(map[string]any{
				"content_md":         req.ContentMD,
				"version":            nextVersion,
				"updated_by_user_id": actorUserID,
				"updated_at":         now,
			})
		if updateResult.Error != nil {
			return updateResult.Error
		}
		if updateResult.RowsAffected == 0 {
			return errWorkspaceDocumentVersionConflict
		}

		revision := &models.DocumentRevision{
			DocumentRevisionID: strings.ToLower(ulid.Make().String()),
			DocumentID:         documentID,
			Version:            nextVersion,
			ContentMD:          req.ContentMD,
			BaseVersion:        req.BaseVersion,
			EditorUserID:       &actorUserID,
			Source:             models.RevisionSourceRemote,
			CreatedAt:          now,
		}
		if err := tx.Create(revision).Error; err != nil {
			return err
		}

		if err := tx.Table("nodes").
			Where("node_id = ?", strings.TrimSpace(current.NodeID)).
			Update("updated_at", now).Error; err != nil {
			return err
		}
		if err := tx.Table("spaces").
			Where("space_id = ?", spaceID).
			Update("updated_at", now).Error; err != nil {
			return err
		}

		latestRow, err := fetchDocument(tx)
		if err != nil {
			return err
		}
		latest = mapWorkspaceDocumentResponse(latestRow)
		return nil
	}); err != nil {
		if errors.Is(err, errWorkspaceDocumentVersionConflict) {
			latestRow, latestErr := fetchDocument(h.db.WithContext(c.Request.Context()))
			if latestErr != nil {
				response.InternalError(c)
				return
			}
			h.writeDocumentVersionConflict(c, mapWorkspaceDocumentResponse(latestRow))
			return
		}
		response.InternalError(c)
		return
	}

	response.JSON(c, http.StatusOK, saveWorkspaceDocumentResponse{
		Document: latest,
	})
}

// ListRevisions 返回文档修订历史。
func (h *workspaceHandler) ListRevisions(c *gin.Context) {
	actorUserID, ok := h.requireActorUserID(c)
	if !ok {
		return
	}
	if h == nil || h.db == nil || h.visibilityService == nil {
		response.InternalError(c)
		return
	}

	documentID := strings.TrimSpace(c.Param("docId"))
	if documentID == "" {
		response.Error(c, http.StatusBadRequest, "INVALID_DOCUMENT_ID", "document id is required")
		return
	}

	_, err := h.visibilityService.GetDocument(c.Request.Context(), documentID, actorUserID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrDocumentNotFound):
			response.Error(c, http.StatusNotFound, "DOCUMENT_NOT_FOUND", "document not found")
		case errors.Is(err, service.ErrViewerLoginRequired):
			response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "login required")
		case errors.Is(err, service.ErrDocumentAccessDenied):
			response.Error(c, http.StatusForbidden, "FORBIDDEN", "insufficient document permission")
		default:
			response.InternalError(c)
		}
		return
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
	if err := h.db.WithContext(c.Request.Context()).
		Table("document_revisions").
		Select("document_revision_id", "document_id", "version", "content_md", "base_version", "source", "created_at").
		Where("document_id = ?", documentID).
		Order("version DESC, created_at DESC, id DESC").
		Find(&rows).Error; err != nil {
		response.InternalError(c)
		return
	}

	revisions := make([]workspaceRevisionResponse, 0, len(rows))
	for _, row := range rows {
		source := strings.TrimSpace(string(row.Source))
		if source != string(models.RevisionSourceLocal) && source != string(models.RevisionSourceRemote) {
			source = string(models.RevisionSourceRemote)
		}
		revisions = append(revisions, workspaceRevisionResponse{
			ID:          strings.TrimSpace(row.DocumentRevisionID),
			DocumentID:  strings.TrimSpace(row.DocumentID),
			Version:     row.Version,
			ContentMD:   row.ContentMD,
			BaseVersion: row.BaseVersion,
			CreatedAt:   formatWorkspaceTime(row.CreatedAtRaw),
			Source:      source,
		})
	}

	response.JSON(c, http.StatusOK, revisions)
}

func (h *workspaceHandler) requireActorUserID(c *gin.Context) (string, bool) {
	if h == nil || h.authService == nil {
		response.InternalError(c)
		return "", false
	}
	accessToken, ok := bearerTokenFromRequest(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "authorization token is required")
		return "", false
	}

	session, err := h.authService.Me(c.Request.Context(), accessToken)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid access token")
		return "", false
	}
	actorUserID := strings.TrimSpace(session.User.ID)
	if actorUserID == "" {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid access token")
		return "", false
	}
	return actorUserID, true
}

func (h *workspaceHandler) ensureSpaceReadable(ctx context.Context, spaceID string, userID string) error {
	space, err := h.loadSpaceAccess(ctx, spaceID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(space.OwnerUserID) == userID {
		return nil
	}

	isPlatformAdmin, err := h.isPlatformAdmin(ctx, userID)
	if err != nil {
		return err
	}
	if isPlatformAdmin {
		return nil
	}

	hasScope, err := h.hasSpaceAdminScope(ctx, userID, spaceID)
	if err != nil {
		return err
	}
	if hasScope {
		return nil
	}

	memberRole, memberFound, err := h.findSpaceMemberRole(ctx, spaceID, userID)
	if err != nil {
		return err
	}
	if memberFound {
		_ = memberRole
		return nil
	}

	switch space.Visibility {
	case models.VisibilityPublic, models.VisibilityAuthenticated:
		return nil
	default:
		return service.ErrSpaceAccessDenied
	}
}

func (h *workspaceHandler) ensureSpaceWritable(ctx context.Context, spaceID string, userID string) error {
	space, err := h.loadSpaceAccess(ctx, spaceID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(space.OwnerUserID) == userID {
		return nil
	}

	isPlatformAdmin, err := h.isPlatformAdmin(ctx, userID)
	if err != nil {
		return err
	}
	if isPlatformAdmin {
		return nil
	}

	hasScope, err := h.hasSpaceAdminScope(ctx, userID, spaceID)
	if err != nil {
		return err
	}
	if hasScope {
		return nil
	}

	memberRole, memberFound, err := h.findSpaceMemberRole(ctx, spaceID, userID)
	if err != nil {
		return err
	}
	if !memberFound {
		return service.ErrSpaceAccessDenied
	}
	if memberRole == models.RoleReader {
		return service.ErrSpaceAccessDenied
	}
	return nil
}

type spaceAccessRow struct {
	SpaceID     string              `gorm:"column:space_id"`
	OwnerUserID string              `gorm:"column:owner_user_id"`
	Visibility  models.Visibility   `gorm:"column:visibility"`
	Status      models.EntityStatus `gorm:"column:status"`
	DeletedAt   *time.Time          `gorm:"column:deleted_at"`
}

func (h *workspaceHandler) loadSpaceAccess(ctx context.Context, spaceID string) (*spaceAccessRow, error) {
	if h == nil || h.db == nil {
		return nil, errors.New("workspace handler dependencies are nil")
	}
	var row spaceAccessRow
	if err := h.db.WithContext(ctx).
		Table("spaces").
		Select("space_id", "owner_user_id", "visibility", "status", "deleted_at").
		Where("space_id = ?", spaceID).
		Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, service.ErrSpaceNotFound
		}
		return nil, err
	}
	if row.DeletedAt != nil || row.Status != models.EntityStatusActive {
		return nil, service.ErrSpaceNotFound
	}
	if !models.IsValidVisibility(row.Visibility) {
		row.Visibility = models.VisibilityMember
	}
	return &row, nil
}

func (h *workspaceHandler) isPlatformAdmin(ctx context.Context, userID string) (bool, error) {
	if h == nil || h.db == nil {
		return false, errors.New("workspace handler dependencies are nil")
	}
	var count int64
	if err := h.db.WithContext(ctx).
		Table("user_admin_roles").
		Where("user_id = ? AND role = ?", userID, models.AdminRolePlatformAdmin).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (h *workspaceHandler) hasSpaceAdminScope(ctx context.Context, userID string, spaceID string) (bool, error) {
	if h == nil || h.db == nil {
		return false, errors.New("workspace handler dependencies are nil")
	}
	var count int64
	if err := h.db.WithContext(ctx).
		Table("space_admin_scopes").
		Where("user_id = ? AND space_id = ?", userID, spaceID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (h *workspaceHandler) findSpaceMemberRole(
	ctx context.Context,
	spaceID string,
	userID string,
) (models.Role, bool, error) {
	if h == nil || h.db == nil {
		return "", false, errors.New("workspace handler dependencies are nil")
	}
	type memberRoleRow struct {
		Role models.Role `gorm:"column:role"`
	}
	var row memberRoleRow
	if err := h.db.WithContext(ctx).
		Table("space_members").
		Select("role").
		Where("space_id = ? AND user_id = ?", spaceID, userID).
		Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	switch row.Role {
	case models.RoleOwner, models.RoleCollaborator, models.RoleReader:
		return row.Role, true, nil
	default:
		return models.RoleReader, true, nil
	}
}

func (h *workspaceHandler) writeDocumentVersionConflict(c *gin.Context, latest workspaceDocumentResponse) {
	c.JSON(http.StatusOK, response.JsonResult[map[string]any]{
		Code:      response.ResolveErrorCode("DOCUMENT_VERSION_CONFLICT"),
		Message:   "document version conflict",
		RequestID: response.RequestIDFromContext(c),
		Data: map[string]any{
			"latestDocument": latest,
		},
	})
}

func mapWorkspaceTreeResponses(nodes []*workspaceTreeNode) []workspaceTreeNodeResponse {
	items := make([]workspaceTreeNodeResponse, 0, len(nodes))
	for _, node := range nodes {
		if node == nil {
			continue
		}
		items = append(items, workspaceTreeNodeResponse{
			ID:       node.ID,
			SpaceID:  node.SpaceID,
			ParentID: node.ParentID,
			Type:     normalizeWorkspaceNodeType(node.Type),
			Title:    node.Title,
			Sort:     node.Sort,
			Children: mapWorkspaceTreeResponses(node.Children),
		})
	}
	return items
}

func mapWorkspaceDocumentResponse(row workspaceDocumentRow) workspaceDocumentResponse {
	return workspaceDocumentResponse{
		ID:        strings.TrimSpace(row.DocumentID),
		NodeID:    strings.TrimSpace(row.NodeID),
		ThemeID:   strings.TrimSpace(row.ThemeID),
		Title:     strings.TrimSpace(row.Title),
		ContentMD: row.ContentMD,
		Version:   row.Version,
		UpdatedAt: formatWorkspaceTime(row.UpdatedAtRaw),
	}
}

func normalizeOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func normalizeWorkspaceNodeType(value models.NodeType) models.NodeType {
	if value == models.NodeTypeFolder || value == models.NodeTypeDoc {
		return value
	}
	return models.NodeTypeDoc
}

func formatWorkspaceTime(raw string) string {
	parsed := parseWorkspaceTime(raw)
	if parsed.IsZero() {
		return time.Now().UTC().Format(time.RFC3339Nano)
	}
	return parsed.UTC().Format(time.RFC3339Nano)
}

func parseWorkspaceTime(raw string) time.Time {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05.999999999",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05.999999999-07:00",
	}
	for _, layout := range layouts {
		if parsedAt, err := time.Parse(layout, value); err == nil {
			return parsedAt.UTC()
		}
	}
	return time.Time{}
}
