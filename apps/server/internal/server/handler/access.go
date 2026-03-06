package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/rendercache"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
)

var (
	accessGetSpaceErrorMappings = []response.ErrorTemplateMapping{
		{Target: service.ErrSpaceNotFound, Template: response.AccessErrSpaceNotFound},
		{Target: service.ErrViewerLoginRequired, Template: response.AccessErrLoginRequired},
		{Target: service.ErrSpaceAccessDenied, Template: response.AccessErrInsufficientSpacePermission},
	}

	accessUpdateSpaceVisibilityErrorMappings = []response.ErrorTemplateMapping{
		{Target: service.ErrInvalidVisibilityValue, Template: response.AccessErrVisibilityPublicAuthenticatedMember},
		{Target: service.ErrViewerLoginRequired, Template: response.AccessErrLoginRequired},
		{Target: service.ErrSpaceNotFound, Template: response.AccessErrSpaceNotFound},
		{Target: service.ErrSpaceAccessDenied, Template: response.AccessErrOnlyOwnerCanUpdateSpaceVisibility},
	}

	accessGetDocumentErrorMappings = []response.ErrorTemplateMapping{
		{Target: service.ErrDocumentNotFound, Template: response.AccessErrDocumentNotFound},
		{Target: service.ErrViewerLoginRequired, Template: response.AccessErrLoginRequired},
		{Target: service.ErrDocumentAccessDenied, Template: response.AccessErrInsufficientDocumentPermission},
	}

	accessUpdateDocumentVisibilityErrorMappings = []response.ErrorTemplateMapping{
		{Target: service.ErrInvalidVisibilityValue, Template: response.AccessErrVisibilityPublicAuthenticatedMember},
		{Target: service.ErrViewerLoginRequired, Template: response.AccessErrLoginRequired},
		{Target: service.ErrDocumentNotFound, Template: response.AccessErrDocumentNotFound},
		{Target: service.ErrDocumentAccessDenied, Template: response.AccessErrInsufficientDocumentPermission},
	}
)

type accessHandler struct {
	authService        *service.AuthService
	visibilityService  *service.VisibilityService
	renderCache        *rendercache.Cache
	searchIndexService *service.SearchIndexService
}

type updateVisibilityRequest struct {
	Visibility models.Visibility `json:"visibility" binding:"required"`
}

type spaceAccessResponse struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Visibility string `json:"visibility"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
}

type documentAccessResponse struct {
	ID             string                `json:"id"`
	NodeID         string                `json:"nodeId"`
	ThemeID        string                `json:"themeId"`
	Format         models.DocumentFormat `json:"format"`
	Visibility     string                `json:"visibility"`
	Title          string                `json:"title"`
	ContentMD      string                `json:"contentMd"`
	Version        int                   `json:"version"`
	SourceBlobID   *string               `json:"sourceBlobId,omitempty"`
	SourceFileName *string               `json:"sourceFileName,omitempty"`
	SourceMimeType *string               `json:"sourceMimeType,omitempty"`
	ContentVersion int                   `json:"contentVersion"`
	UpdatedAt      string                `json:"updatedAt"`
}

// NewAccessHandler 创建文档/空间公开访问处理器。
func NewAccessHandler(
	authService *service.AuthService,
	visibilityService *service.VisibilityService,
	renderCache *rendercache.Cache,
	searchIndexServices ...*service.SearchIndexService,
) *accessHandler {
	var searchIndexService *service.SearchIndexService
	if len(searchIndexServices) > 0 {
		searchIndexService = searchIndexServices[0]
	}

	return &accessHandler{
		authService:        authService,
		visibilityService:  visibilityService,
		renderCache:        renderCache,
		searchIndexService: searchIndexService,
	}
}

// GetSpace 按空间可见性规则返回空间基础信息。
func (h *accessHandler) GetSpace(c *gin.Context) {
	if h == nil || h.visibilityService == nil {
		response.InternalError(c)
		return
	}

	spaceID := c.Param("spaceId")
	if spaceID == "" {
		response.AccessErrSpaceIDRequired.Write(c)
		return
	}

	viewerUserID, err := h.resolveOptionalViewerUserID(c)
	if err != nil {
		setRequestErrmsg(c, err, "解析访问令牌失败")
		response.AccessErrAccessToken.Write(c)
		return
	}

	space, err := h.visibilityService.GetSpace(c.Request.Context(), spaceID, viewerUserID)
	if err != nil {
		setRequestErrmsg(c, err, "查询空间访问信息失败")
		if !response.WriteMappedError(c, err, accessGetSpaceErrorMappings...) {
			response.InternalError(c)
		}
		return
	}

	response.JSON(c, http.StatusOK, spaceAccessResponse{
		ID:         space.SpaceID,
		Name:       space.Name,
		Visibility: string(space.Visibility),
		CreatedAt:  space.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:  space.UpdatedAt.UTC().Format(time.RFC3339Nano),
	})
}

// UpdateSpaceVisibility 更新空间公开级别（仅 owner 可操作）。
func (h *accessHandler) UpdateSpaceVisibility(c *gin.Context) {
	if h == nil || h.visibilityService == nil {
		response.InternalError(c)
		return
	}

	spaceID := c.Param("spaceId")
	if spaceID == "" {
		response.AccessErrSpaceIDRequired.Write(c)
		return
	}

	actorUserID, ok := h.requireViewerUserID(c)
	if !ok {
		return
	}

	var req updateVisibilityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AccessErrVisibilityRequired.Write(c)
		return
	}

	space, err := h.visibilityService.UpdateSpaceVisibility(c.Request.Context(), spaceID, actorUserID, req.Visibility)
	if err != nil {
		setRequestErrmsg(c, err, "更新空间可见性失败")
		if !response.WriteMappedError(c, err, accessUpdateSpaceVisibilityErrorMappings...) {
			response.InternalError(c)
		}
		return
	}
	response.JSON(c, http.StatusOK, spaceAccessResponse{
		ID:         space.SpaceID,
		Name:       space.Name,
		Visibility: string(space.Visibility),
		CreatedAt:  space.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:  space.UpdatedAt.UTC().Format(time.RFC3339Nano),
	})
}

// GetDocument 按空间+文档综合可见性规则返回文档。
func (h *accessHandler) GetDocument(c *gin.Context) {
	if h == nil || h.visibilityService == nil {
		response.InternalError(c)
		return
	}

	documentID := c.Param("docId")
	if documentID == "" {
		response.AccessErrDocumentIDRequired.Write(c)
		return
	}

	viewerUserID, err := h.resolveOptionalViewerUserID(c)
	if err != nil {
		setRequestErrmsg(c, err, "解析访问令牌失败")
		response.AccessErrAccessToken.Write(c)
		return
	}

	document, err := h.visibilityService.GetDocument(c.Request.Context(), documentID, viewerUserID)
	if err != nil {
		setRequestErrmsg(c, err, "查询文档访问信息失败")
		if !response.WriteMappedError(c, err, accessGetDocumentErrorMappings...) {
			response.InternalError(c)
		}
		return
	}

	response.JSON(c, http.StatusOK, documentAccessResponse{
		ID:             document.DocumentID,
		NodeID:         document.NodeID,
		ThemeID:        document.ThemeID,
		Format:         models.NormalizeDocumentFormat(document.Format),
		Visibility:     string(document.Visibility),
		Title:          document.Title,
		ContentMD:      document.ContentMD,
		Version:        document.Version,
		SourceBlobID:   normalizeOptionalAccessString(document.SourceBlobID),
		SourceFileName: normalizeOptionalAccessString(document.SourceFileName),
		SourceMimeType: normalizeOptionalAccessString(document.SourceMimeType),
		ContentVersion: normalizeAccessContentVersion(document.ContentVersion, document.Version),
		UpdatedAt:      formatAccessTime(document.UpdatedAt),
	})
}

// UpdateDocumentVisibility 更新文档公开级别（空间 owner / collaborator / 管理员可操作）。
func (h *accessHandler) UpdateDocumentVisibility(c *gin.Context) {
	if h == nil || h.visibilityService == nil {
		response.InternalError(c)
		return
	}

	documentID := c.Param("docId")
	if documentID == "" {
		response.AccessErrDocumentIDRequired.Write(c)
		return
	}

	actorUserID, ok := h.requireViewerUserID(c)
	if !ok {
		return
	}

	var req updateVisibilityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		setRequestErrmsg(c, err, "解析文档可见性请求失败")
		response.AccessErrVisibilityRequired.Write(c)
		return
	}

	document, err := h.visibilityService.UpdateDocumentVisibility(c.Request.Context(), documentID, actorUserID, req.Visibility)
	if err != nil {
		setRequestErrmsg(c, err, "更新文档可见性失败")
		if !response.WriteMappedError(c, err, accessUpdateDocumentVisibilityErrorMappings...) {
			response.InternalError(c)
		}
		return
	}

	if h != nil && h.renderCache != nil {
		// 文档可见性变化会直接影响阅读页输出，需主动失效渲染缓存。
		h.renderCache.PurgeDoc(document.DocumentID)
	}
	response.JSON(c, http.StatusOK, documentAccessResponse{
		ID:             document.DocumentID,
		NodeID:         document.NodeID,
		ThemeID:        document.ThemeID,
		Format:         models.NormalizeDocumentFormat(document.Format),
		Visibility:     string(document.Visibility),
		Title:          document.Title,
		ContentMD:      document.ContentMD,
		Version:        document.Version,
		SourceBlobID:   normalizeOptionalAccessString(document.SourceBlobID),
		SourceFileName: normalizeOptionalAccessString(document.SourceFileName),
		SourceMimeType: normalizeOptionalAccessString(document.SourceMimeType),
		ContentVersion: normalizeAccessContentVersion(document.ContentVersion, document.Version),
		UpdatedAt:      formatAccessTime(document.UpdatedAt),
	})
}

func normalizeOptionalAccessString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := *value
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func normalizeAccessContentVersion(contentVersion int, version int) int {
	switch {
	case contentVersion > 0:
		return contentVersion
	case version > 0:
		return version
	default:
		return 1
	}
}

func formatAccessTime(updatedAt time.Time) string {
	if updatedAt.IsZero() {
		return time.Now().UTC().Format(time.RFC3339Nano)
	}
	return updatedAt.UTC().Format(time.RFC3339Nano)
}

func (h *accessHandler) requireViewerUserID(c *gin.Context) (string, bool) {
	viewerUserID, err := h.resolveRequiredViewerUserID(c)
	if err != nil {
		setRequestErrmsg(c, err, "解析访问令牌失败")
		response.AccessErrAuthorizationTokenRequired.Write(c)
		return "", false
	}
	return viewerUserID, true
}

func (h *accessHandler) resolveRequiredViewerUserID(c *gin.Context) (string, error) {
	rawToken, ok := bearerTokenFromRequest(c)
	if !ok {
		return "", errors.New("missing bearer token")
	}
	return h.resolveViewerUserIDByToken(c, rawToken)
}

func (h *accessHandler) resolveOptionalViewerUserID(c *gin.Context) (string, error) {
	rawToken, ok := bearerTokenFromRequest(c)
	if !ok {
		return "", nil
	}
	return h.resolveViewerUserIDByToken(c, rawToken)
}

func (h *accessHandler) resolveViewerUserIDByToken(c *gin.Context, rawToken string) (string, error) {
	if h.authService == nil {
		return "", errors.New("auth service is nil")
	}
	session, err := h.authService.Me(c.Request.Context(), rawToken)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorized) {
			return "", err
		}
		return "", err
	}
	return session.User.ID, nil
}
