package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/rendercache"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"
)

const (
	defaultWorkspaceCategoryID   = "01jmf4v2x7m7f1m6qv5kh0t2mn"
	defaultWorkspaceCategoryName = "未分类"
	maxWorkspaceSpaceNameLength  = 64
)

type workspaceHandler struct {
	workspaceRepo              repository.WorkspaceRepository
	documentAttachmentRepo     repository.DocumentAttachmentRepository
	documentAttachmentCleanup  *service.DocumentAttachmentCleanupService
	documentImageAssetService  *service.DocumentImageAssetService
	searchIndexService         *service.SearchIndexService
	authService                *service.AuthService
	visibilityService          *service.VisibilityService
	imageHostingService        *service.ImageHostingService
	attachmentTokenService     *service.DocumentAttachmentDownloadTokenService
	localImageRootDir          string
	remoteImageHTTPClient      *http.Client
	remoteImageFailureCooldown time.Duration
	remoteImageFailureMu       sync.Mutex
	remoteImageFailureUntil    map[string]time.Time
	renderCache                *rendercache.Cache
}

type workspaceSpaceResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type workspaceTreeNodeResponse struct {
	ID         string                      `json:"id"`
	DocumentID *string                     `json:"documentId,omitempty"`
	SpaceID    string                      `json:"spaceId"`
	ParentID   *string                     `json:"parentId"`
	Type       models.NodeType             `json:"type"`
	Title      string                      `json:"title"`
	Sort       int                         `json:"sort"`
	Visibility *models.Visibility          `json:"visibility,omitempty"`
	Children   []workspaceTreeNodeResponse `json:"children"`
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

type moveWorkspaceNodeRequest struct {
	TargetParentID *string `json:"targetParentId"`
	TargetIndex    *int    `json:"targetIndex"`
}

type saveWorkspaceDocumentRequest struct {
	ContentMD   string `json:"contentMd"`
	BaseVersion int    `json:"baseVersion" binding:"required"`
}

type saveWorkspaceDocumentResponse struct {
	Document workspaceDocumentResponse `json:"document"`
}

type localizeDocumentRemoteImagesRequest struct {
	ImageURLs []string `json:"imageUrls"`
}

type localizeDocumentRemoteImagesResponse struct {
	LocalizedURLs map[string]string `json:"localizedUrls"`
}

type workspaceTreeNode struct {
	ID         string
	DocumentID *string
	SpaceID    string
	ParentID   *string
	Type       models.NodeType
	Title      string
	Sort       int
	Visibility *models.Visibility
	Children   []*workspaceTreeNode
}

// NewWorkspaceHandler 创建编辑器工作区处理器。
func NewWorkspaceHandler(
	workspaceRepo repository.WorkspaceRepository,
	documentAttachmentRepo repository.DocumentAttachmentRepository,
	documentAttachmentCleanup *service.DocumentAttachmentCleanupService,
	documentImageAssetService *service.DocumentImageAssetService,
	authService *service.AuthService,
	visibilityService *service.VisibilityService,
	imageHostingService *service.ImageHostingService,
	attachmentTokenService *service.DocumentAttachmentDownloadTokenService,
	renderCache *rendercache.Cache,
	searchIndexServices ...*service.SearchIndexService,
) *workspaceHandler {
	var searchIndexService *service.SearchIndexService
	if len(searchIndexServices) > 0 {
		searchIndexService = searchIndexServices[0]
	}

	return &workspaceHandler{
		workspaceRepo:             workspaceRepo,
		documentAttachmentRepo:    documentAttachmentRepo,
		documentAttachmentCleanup: documentAttachmentCleanup,
		documentImageAssetService: documentImageAssetService,
		searchIndexService:        searchIndexService,
		authService:               authService,
		visibilityService:         visibilityService,
		imageHostingService:       imageHostingService,
		attachmentTokenService:    attachmentTokenService,
		localImageRootDir:         defaultLocalImageStorageRoot,
		remoteImageHTTPClient: &http.Client{
			Timeout: 12 * time.Second,
		},
		remoteImageFailureCooldown: defaultRemoteImageFailureCooldown,
		remoteImageFailureUntil:    make(map[string]time.Time),
		renderCache:                renderCache,
	}
}

// ListSpaces 返回当前登录用户可进入编辑器的空间列表。
func (h *workspaceHandler) ListSpaces(c *gin.Context) {
	actorUserID, ok := h.requireActorUserID(c)
	if !ok {
		return
	}
	if h == nil || h.workspaceRepo == nil {
		response.InternalError(c)
		return
	}

	rows, err := h.workspaceRepo.ListSpacesByActor(c.Request.Context(), actorUserID)
	if err != nil {
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
	if h == nil || h.workspaceRepo == nil {
		response.InternalError(c)
		return
	}

	var req createWorkspaceSpaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.WorkspaceErrSpaceNameRequired.Write(c)
		return
	}

	spaceName := strings.TrimSpace(req.Name)
	if spaceName == "" || len([]rune(spaceName)) > maxWorkspaceSpaceNameLength {
		response.WorkspaceErrSpaceName.Write(c)
		return
	}

	defaultCategory := models.SpaceCategory{
		CategoryID: defaultWorkspaceCategoryID,
		Name:       defaultWorkspaceCategoryName,
	}
	defaultCategoryValue, err := h.workspaceRepo.GetDefaultCategory(c.Request.Context())
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		response.InternalError(c)
		return
	}
	if defaultCategoryValue != nil {
		defaultCategory = *defaultCategoryValue
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
	if err := h.workspaceRepo.CreateSpace(c.Request.Context(), space); err != nil {
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
	if h == nil || h.workspaceRepo == nil {
		response.InternalError(c)
		return
	}

	spaceID := strings.TrimSpace(c.Param("spaceId"))
	if spaceID == "" {
		response.WorkspaceErrSpaceIDRequired.Write(c)
		return
	}

	if err := h.ensureSpaceReadable(c.Request.Context(), spaceID, actorUserID); err != nil {
		switch {
		case errors.Is(err, service.ErrSpaceNotFound):
			response.WorkspaceErrSpaceNotFound.Write(c)
		case errors.Is(err, service.ErrViewerLoginRequired):
			response.WorkspaceErrLoginRequired.Write(c)
		case errors.Is(err, service.ErrSpaceAccessDenied):
			response.WorkspaceErrInsufficientSpacePermission.Write(c)
		default:
			response.InternalError(c)
		}
		return
	}

	rows, err := h.workspaceRepo.ListTreeNodesBySpaceID(c.Request.Context(), spaceID)
	if err != nil {
		response.InternalError(c)
		return
	}

	treeNodes := make(map[string]*workspaceTreeNode, len(rows))
	for _, row := range rows {
		nodeID := strings.TrimSpace(row.NodeID)
		if nodeID == "" {
			continue
		}
		documentID := normalizeOptionalString(row.DocumentID)
		documentVisibility := normalizeWorkspaceDocumentVisibility(row.Type, row.DocumentVisibility)
		treeNodes[nodeID] = &workspaceTreeNode{
			ID:         nodeID,
			DocumentID: documentID,
			SpaceID:    strings.TrimSpace(row.SpaceID),
			ParentID:   normalizeOptionalString(row.ParentNodeID),
			Type:       normalizeWorkspaceNodeType(row.Type),
			Title:      strings.TrimSpace(row.Title),
			Sort:       row.Sort,
			Visibility: documentVisibility,
			Children:   make([]*workspaceTreeNode, 0),
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
	if h == nil || h.workspaceRepo == nil {
		response.InternalError(c)
		return
	}

	spaceID := strings.TrimSpace(c.Param("spaceId"))
	if spaceID == "" {
		response.WorkspaceErrSpaceIDRequired.Write(c)
		return
	}
	spaceAccess, err := h.ensureSpaceWritable(c.Request.Context(), spaceID, actorUserID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrSpaceNotFound):
			response.WorkspaceErrSpaceNotFound.Write(c)
		case errors.Is(err, service.ErrSpaceAccessDenied):
			response.WorkspaceErrInsufficientSpacePermission.Write(c)
		default:
			response.InternalError(c)
		}
		return
	}

	var req createWorkspaceNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.WorkspaceErrCreateNodeRequest.Write(c)
		return
	}

	if req.Type != models.NodeTypeFolder && req.Type != models.NodeTypeDoc {
		response.WorkspaceErrNodeType.Write(c)
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
		parent, err := h.workspaceRepo.GetNodeByNodeID(c.Request.Context(), *parentID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				response.WorkspaceErrParentNodeNotFound.Write(c)
				return
			}
			response.InternalError(c)
			return
		}
		if strings.TrimSpace(parent.SpaceID) != spaceID {
			response.WorkspaceErrParentNodeNotTargetSpace.Write(c)
			return
		}
	}

	maxSort, err := h.workspaceRepo.GetMaxNodeSort(c.Request.Context(), spaceID, parentID)
	if err != nil {
		response.InternalError(c)
		return
	}

	now := time.Now().UTC()
	nodeID := strings.ToLower(ulid.Make().String())
	node := &models.Node{
		NodeID:          nodeID,
		SpaceID:         spaceID,
		ParentNodeID:    parentID,
		Type:            req.Type,
		Title:           title,
		Sort:            maxSort + 1,
		CreatedByUserID: &actorUserID,
		UpdatedByUserID: &actorUserID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	responseBody := createWorkspaceNodeResponse{
		NodeID: node.NodeID,
	}
	defaultDocumentVisibility := spaceAccess.Visibility
	if !models.IsValidVisibility(defaultDocumentVisibility) {
		defaultDocumentVisibility = models.VisibilityMember
	}

	var doc *models.Document
	var revision *models.DocumentRevision
	if req.Type == models.NodeTypeDoc {
		documentID := node.NodeID
		doc = &models.Document{
			DocumentID:      documentID,
			NodeID:          node.NodeID,
			ThemeID:         "default",
			Visibility:      defaultDocumentVisibility,
			Status:          models.EntityStatusActive,
			Title:           title,
			ContentMD:       "",
			Version:         1,
			CreatedByUserID: &actorUserID,
			UpdatedByUserID: &actorUserID,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		revision = &models.DocumentRevision{
			DocumentRevisionID: strings.ToLower(ulid.Make().String()),
			DocumentID:         documentID,
			Version:            1,
			ContentMD:          "",
			BaseVersion:        0,
			EditorUserID:       &actorUserID,
			Source:             models.RevisionSourceRemote,
			CreatedAt:          now,
		}
		responseBody.DocID = documentID
	}

	if err := h.workspaceRepo.CreateNode(c.Request.Context(), repository.WorkspaceCreateNodeParams{
		Node:       node,
		Document:   doc,
		Revision:   revision,
		TouchSpace: spaceID,
		TouchedAt:  now,
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
	if h == nil || h.workspaceRepo == nil {
		response.InternalError(c)
		return
	}

	nodeID := strings.TrimSpace(c.Param("nodeId"))
	if nodeID == "" {
		response.WorkspaceErrNodeIDRequired.Write(c)
		return
	}

	var req updateWorkspaceNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.WorkspaceErrUpdateNodeRequest.Write(c)
		return
	}
	if req.Title == nil && req.ParentID == nil && req.Sort == nil {
		response.JSON(c, http.StatusOK, struct{}{})
		return
	}

	node, err := h.workspaceRepo.GetNodeByNodeID(c.Request.Context(), nodeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.WorkspaceErrNodeNotFound.Write(c)
			return
		}
		response.InternalError(c)
		return
	}

	if _, err := h.ensureSpaceWritable(c.Request.Context(), strings.TrimSpace(node.SpaceID), actorUserID); err != nil {
		switch {
		case errors.Is(err, service.ErrSpaceNotFound):
			response.WorkspaceErrSpaceNotFound.Write(c)
		case errors.Is(err, service.ErrSpaceAccessDenied):
			response.WorkspaceErrInsufficientSpacePermission.Write(c)
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
			response.WorkspaceErrNodeCannotItsOwnParent.Write(c)
			return
		}
		if parentID != nil {
			parent, err := h.workspaceRepo.GetNodeByNodeID(c.Request.Context(), *parentID)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					response.WorkspaceErrParentNodeNotFound.Write(c)
					return
				}
				response.InternalError(c)
				return
			}
			if strings.TrimSpace(parent.SpaceID) != strings.TrimSpace(node.SpaceID) {
				response.WorkspaceErrParentNodeNotTargetSpace.Write(c)
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

	var documentTitle *string
	if node.Type == models.NodeTypeDoc && req.Title != nil {
		documentTitle = &nextTitle
	}

	if err := h.workspaceRepo.UpdateNode(c.Request.Context(), repository.WorkspaceUpdateNodeParams{
		NodeID:        nodeID,
		UpdateValues:  updateValues,
		DocumentTitle: documentTitle,
		ActorUserID:   actorUserID,
		TouchSpace:    strings.TrimSpace(node.SpaceID),
		TouchedAt:     now,
	}); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.WorkspaceErrNodeNotFound.Write(c)
			return
		}
		response.InternalError(c)
		return
	}

	if h != nil && h.renderCache != nil && node.Type == models.NodeTypeDoc {
		// 文档节点标题变更会影响阅读页展示，按文档维度主动失效缓存。
		h.renderCache.PurgeDoc(strings.TrimSpace(node.NodeID))
	}
	response.JSON(c, http.StatusOK, struct{}{})
}

// MoveNode 移动目录节点（支持同级重排与跨父级移动）。
func (h *workspaceHandler) MoveNode(c *gin.Context) {
	actorUserID, ok := h.requireActorUserID(c)
	if !ok {
		return
	}
	if h == nil || h.workspaceRepo == nil {
		response.InternalError(c)
		return
	}

	nodeID := strings.TrimSpace(c.Param("nodeId"))
	if nodeID == "" {
		response.WorkspaceErrNodeIDRequired.Write(c)
		return
	}

	var req moveWorkspaceNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.WorkspaceErrMoveNodeRequest.Write(c)
		return
	}
	if req.TargetIndex == nil || *req.TargetIndex < 0 {
		response.WorkspaceErrMoveNodeRequest.Write(c)
		return
	}

	node, err := h.workspaceRepo.GetNodeByNodeID(c.Request.Context(), nodeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.WorkspaceErrNodeNotFound.Write(c)
			return
		}
		response.InternalError(c)
		return
	}

	spaceID := strings.TrimSpace(node.SpaceID)
	if _, err := h.ensureSpaceWritable(c.Request.Context(), spaceID, actorUserID); err != nil {
		switch {
		case errors.Is(err, service.ErrSpaceNotFound):
			response.WorkspaceErrSpaceNotFound.Write(c)
		case errors.Is(err, service.ErrSpaceAccessDenied):
			response.WorkspaceErrInsufficientSpacePermission.Write(c)
		default:
			response.InternalError(c)
		}
		return
	}

	now := time.Now().UTC()
	if err := h.workspaceRepo.MoveNode(c.Request.Context(), repository.WorkspaceMoveNodeParams{
		NodeID:             nodeID,
		TargetParentNodeID: normalizeOptionalString(req.TargetParentID),
		TargetIndex:        *req.TargetIndex,
		ActorUserID:        actorUserID,
		TouchSpace:         spaceID,
		TouchedAt:          now,
	}); err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			response.WorkspaceErrNodeNotFound.Write(c)
		case errors.Is(err, repository.ErrWorkspaceMoveTargetParentNotFound):
			response.WorkspaceErrParentNodeNotFound.Write(c)
		case errors.Is(err, repository.ErrWorkspaceMoveTargetParentNotInSameSpace):
			response.WorkspaceErrParentNodeNotTargetSpace.Write(c)
		case errors.Is(err, repository.ErrWorkspaceMoveCycleDetected):
			response.WorkspaceErrNodeMoveCycleDetected.Write(c)
		default:
			response.InternalError(c)
		}
		return
	}

	if h != nil && h.renderCache != nil && node.Type == models.NodeTypeDoc {
		// 文档节点位置变化会影响阅读页树结构，按文档维度主动清理缓存。
		h.renderCache.PurgeDoc(strings.TrimSpace(node.NodeID))
	}

	response.JSON(c, http.StatusOK, struct{}{})
}

// DeleteNode 删除目录节点及其子树。
func (h *workspaceHandler) DeleteNode(c *gin.Context) {
	actorUserID, ok := h.requireActorUserID(c)
	if !ok {
		return
	}
	if h == nil || h.workspaceRepo == nil {
		response.InternalError(c)
		return
	}

	nodeID := strings.TrimSpace(c.Param("nodeId"))
	if nodeID == "" {
		response.WorkspaceErrNodeIDRequired.Write(c)
		return
	}

	node, err := h.workspaceRepo.GetNodeByNodeID(c.Request.Context(), nodeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.WorkspaceErrNodeNotFound.Write(c)
			return
		}
		response.InternalError(c)
		return
	}

	spaceID := strings.TrimSpace(node.SpaceID)
	if _, err := h.ensureSpaceWritable(c.Request.Context(), spaceID, actorUserID); err != nil {
		switch {
		case errors.Is(err, service.ErrSpaceNotFound):
			response.WorkspaceErrSpaceNotFound.Write(c)
		case errors.Is(err, service.ErrSpaceAccessDenied):
			response.WorkspaceErrInsufficientSpacePermission.Write(c)
		default:
			response.InternalError(c)
		}
		return
	}

	now := time.Now().UTC()
	deleted, err := h.workspaceRepo.DeleteNode(c.Request.Context(), nodeID, spaceID, now)
	if err != nil {
		response.InternalError(c)
		return
	}
	if !deleted {
		response.WorkspaceErrNodeNotFound.Write(c)
		return
	}

	if h != nil && h.documentAttachmentCleanup != nil {
		if _, cleanupErr := h.documentAttachmentCleanup.CleanupDeletedDocumentAttachments(c.Request.Context(), 200); cleanupErr != nil {
			setRequestErrmsg(c, cleanupErr, "删除节点后清理附件孤儿文件失败")
		}
	}
	response.JSON(c, http.StatusOK, struct{}{})
}

// SaveDocument 保存文档内容（带版本冲突检测）。
func (h *workspaceHandler) SaveDocument(c *gin.Context) {
	actorUserID, ok := h.requireActorUserID(c)
	if !ok {
		return
	}
	if h == nil || h.workspaceRepo == nil {
		response.InternalError(c)
		return
	}

	documentID := strings.TrimSpace(c.Param("docId"))
	if documentID == "" {
		response.WorkspaceErrDocumentIDRequired.Write(c)
		return
	}

	var req saveWorkspaceDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.WorkspaceErrSaveDocumentRequest.Write(c)
		return
	}
	if req.BaseVersion <= 0 {
		response.WorkspaceErrBaseversionRequired.Write(c)
		return
	}

	currentRecord, err := h.workspaceRepo.GetDocumentByDocumentID(c.Request.Context(), documentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.WorkspaceErrDocumentNotFound.Write(c)
			return
		}
		response.InternalError(c)
		return
	}
	current := workspaceDocumentRow{
		DocumentID:   currentRecord.DocumentID,
		NodeID:       currentRecord.NodeID,
		ThemeID:      currentRecord.ThemeID,
		Title:        currentRecord.Title,
		ContentMD:    currentRecord.ContentMD,
		Version:      currentRecord.Version,
		SpaceID:      currentRecord.SpaceID,
		UpdatedAtRaw: currentRecord.UpdatedAtRaw,
	}

	spaceID := strings.TrimSpace(current.SpaceID)
	if _, err := h.ensureSpaceWritable(c.Request.Context(), spaceID, actorUserID); err != nil {
		switch {
		case errors.Is(err, service.ErrSpaceNotFound):
			response.WorkspaceErrSpaceNotFound.Write(c)
		case errors.Is(err, service.ErrSpaceAccessDenied):
			response.WorkspaceErrInsufficientSpacePermission.Write(c)
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
	saved, err := h.workspaceRepo.SaveDocument(c.Request.Context(), repository.WorkspaceSaveDocumentParams{
		DocumentID:  documentID,
		BaseVersion: req.BaseVersion,
		NextVersion: nextVersion,
		ContentMD:   req.ContentMD,
		ActorUserID: actorUserID,
		NodeID:      current.NodeID,
		SpaceID:     spaceID,
		TouchedAt:   now,
		Revision:    revision,
	})
	if err != nil {
		response.InternalError(c)
		return
	}
	if !saved {
		latestRecord, latestErr := h.workspaceRepo.GetDocumentByDocumentID(c.Request.Context(), documentID)
		if latestErr != nil {
			response.InternalError(c)
			return
		}
		h.writeDocumentVersionConflict(c, mapWorkspaceDocumentResponse(workspaceDocumentRow{
			DocumentID:   latestRecord.DocumentID,
			NodeID:       latestRecord.NodeID,
			ThemeID:      latestRecord.ThemeID,
			Title:        latestRecord.Title,
			ContentMD:    latestRecord.ContentMD,
			Version:      latestRecord.Version,
			SpaceID:      latestRecord.SpaceID,
			UpdatedAtRaw: latestRecord.UpdatedAtRaw,
		}))
		return
	}

	if h != nil && h.renderCache != nil {
		// 文档更新后主动失效该文档所有渲染缓存，避免旧版本内容被命中。
		h.renderCache.PurgeDoc(documentID)
	}

	current.ContentMD = req.ContentMD
	current.Version = nextVersion
	current.UpdatedAtRaw = now.Format(time.RFC3339Nano)

	if h != nil && h.documentImageAssetService != nil {
		if syncErr := h.documentImageAssetService.SyncDocumentImageAssets(
			c.Request.Context(),
			service.SyncDocumentImageAssetsInput{
				DocumentID:   documentID,
				SpaceID:      spaceID,
				ContentMD:    req.ContentMD,
				ReferencedAt: now,
			},
		); syncErr != nil {
			setRequestErrmsg(c, syncErr, "同步文档图片引用失败")
		}
	}
	response.JSON(c, http.StatusOK, saveWorkspaceDocumentResponse{
		Document: mapWorkspaceDocumentResponse(current),
	})
}

// LocalizeDocumentRemoteImages 将指定文档中的外链图片 URL 转存到本地并返回映射关系。
func (h *workspaceHandler) LocalizeDocumentRemoteImages(c *gin.Context) {
	actorUserID, ok := h.requireActorUserID(c)
	if !ok {
		return
	}
	if h == nil || h.workspaceRepo == nil {
		response.InternalError(c)
		return
	}

	documentID := strings.TrimSpace(c.Param("docId"))
	if documentID == "" {
		response.WorkspaceErrDocumentIDRequired.Write(c)
		return
	}

	var req localizeDocumentRemoteImagesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.WorkspaceErrLocalizeRemoteImagesRequest.Write(c)
		return
	}

	currentRecord, err := h.workspaceRepo.GetDocumentByDocumentID(c.Request.Context(), documentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.WorkspaceErrDocumentNotFound.Write(c)
			return
		}
		response.InternalError(c)
		return
	}

	spaceID := strings.TrimSpace(currentRecord.SpaceID)
	if _, err := h.ensureSpaceWritable(c.Request.Context(), spaceID, actorUserID); err != nil {
		switch {
		case errors.Is(err, service.ErrSpaceNotFound):
			response.WorkspaceErrSpaceNotFound.Write(c)
		case errors.Is(err, service.ErrSpaceAccessDenied):
			response.WorkspaceErrInsufficientSpacePermission.Write(c)
		default:
			response.InternalError(c)
		}
		return
	}

	localizedURLs := h.localizeRemoteImageURLs(c.Request.Context(), documentID, req.ImageURLs)
	if len(localizedURLs) == 0 {
		response.JSON(c, http.StatusOK, localizeDocumentRemoteImagesResponse{
			LocalizedURLs: map[string]string{},
		})
		return
	}

	response.JSON(c, http.StatusOK, localizeDocumentRemoteImagesResponse{
		LocalizedURLs: localizedURLs,
	})
}

// ListRevisions 返回文档修订历史。
func (h *workspaceHandler) ListRevisions(c *gin.Context) {
	actorUserID, ok := h.requireActorUserID(c)
	if !ok {
		return
	}
	if h == nil || h.workspaceRepo == nil || h.visibilityService == nil {
		response.InternalError(c)
		return
	}

	documentID := strings.TrimSpace(c.Param("docId"))
	if documentID == "" {
		response.WorkspaceErrDocumentIDRequired.Write(c)
		return
	}

	_, err := h.visibilityService.GetDocument(c.Request.Context(), documentID, actorUserID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrDocumentNotFound):
			response.WorkspaceErrDocumentNotFound.Write(c)
		case errors.Is(err, service.ErrViewerLoginRequired):
			response.WorkspaceErrLoginRequired.Write(c)
		case errors.Is(err, service.ErrDocumentAccessDenied):
			response.WorkspaceErrInsufficientDocumentPermission.Write(c)
		default:
			response.InternalError(c)
		}
		return
	}

	rows, err := h.workspaceRepo.ListRevisionsByDocumentID(c.Request.Context(), documentID)
	if err != nil {
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
		response.WorkspaceErrAuthorizationTokenRequired.Write(c)
		return "", false
	}

	session, err := h.authService.Me(c.Request.Context(), accessToken)
	if err != nil {
		response.WorkspaceErrAccessToken.Write(c)
		return "", false
	}
	actorUserID := strings.TrimSpace(session.User.ID)
	if actorUserID == "" {
		response.WorkspaceErrAccessToken.Write(c)
		return "", false
	}
	return actorUserID, true
}

func (h *workspaceHandler) ensureSpaceReadable(ctx context.Context, spaceID string, userID string) error {
	space, err := h.loadSpaceAccess(ctx, spaceID, userID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(space.OwnerUserID) == userID {
		return nil
	}
	if space.IsPlatformAdmin {
		return nil
	}
	if space.HasSpaceAdminScope {
		return nil
	}
	if space.MemberRole != nil {
		return nil
	}

	switch space.Visibility {
	case models.VisibilityPublic, models.VisibilityAuthenticated:
		return nil
	default:
		return service.ErrSpaceAccessDenied
	}
}

func (h *workspaceHandler) ensureSpaceWritable(
	ctx context.Context,
	spaceID string,
	userID string,
) (*repository.WorkspaceSpacePermissionSnapshot, error) {
	space, err := h.loadSpaceAccess(ctx, spaceID, userID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(space.OwnerUserID) == userID {
		return space, nil
	}
	if space.IsPlatformAdmin {
		return space, nil
	}
	if space.HasSpaceAdminScope {
		return space, nil
	}
	if space.MemberRole == nil {
		return nil, service.ErrSpaceAccessDenied
	}
	if *space.MemberRole == models.RoleReader {
		return nil, service.ErrSpaceAccessDenied
	}
	return space, nil
}

func (h *workspaceHandler) loadSpaceAccess(
	ctx context.Context,
	spaceID string,
	userID string,
) (*repository.WorkspaceSpacePermissionSnapshot, error) {
	if h == nil || h.workspaceRepo == nil {
		return nil, errors.New("workspace handler dependencies are nil")
	}

	row, err := h.workspaceRepo.GetSpacePermissionSnapshot(ctx, spaceID, userID)
	if err != nil {
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
	return row, nil
}

func (h *workspaceHandler) writeDocumentVersionConflict(c *gin.Context, latest workspaceDocumentResponse) {
	c.JSON(http.StatusOK, response.JsonResult[map[string]any]{
		Code:      response.ResolveErrorCode(response.CodeDocumentVersionConflict),
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
			ID:         node.ID,
			DocumentID: node.DocumentID,
			SpaceID:    node.SpaceID,
			ParentID:   node.ParentID,
			Type:       normalizeWorkspaceNodeType(node.Type),
			Title:      node.Title,
			Sort:       node.Sort,
			Visibility: node.Visibility,
			Children:   mapWorkspaceTreeResponses(node.Children),
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

func normalizeWorkspaceDocumentVisibility(
	nodeType models.NodeType,
	rawVisibility *string,
) *models.Visibility {
	if normalizeWorkspaceNodeType(nodeType) != models.NodeTypeDoc {
		return nil
	}

	if rawVisibility == nil {
		visibility := models.VisibilityMember
		return &visibility
	}

	visibility := models.Visibility(strings.TrimSpace(*rawVisibility))
	if !models.IsValidVisibility(visibility) {
		visibility = models.VisibilityMember
	}
	return &visibility
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
