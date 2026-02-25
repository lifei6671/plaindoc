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

type accessHandler struct {
	authService       *service.AuthService
	visibilityService *service.VisibilityService
	renderCache       *rendercache.Cache
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
	ID         string `json:"id"`
	NodeID     string `json:"nodeId"`
	ThemeID    string `json:"themeId"`
	Visibility string `json:"visibility"`
	Title      string `json:"title"`
	ContentMD  string `json:"contentMd"`
	Version    int    `json:"version"`
	UpdatedAt  string `json:"updatedAt"`
}

// NewAccessHandler 创建文档/空间公开访问处理器。
func NewAccessHandler(
	authService *service.AuthService,
	visibilityService *service.VisibilityService,
	renderCache *rendercache.Cache,
) *accessHandler {
	return &accessHandler{
		authService:       authService,
		visibilityService: visibilityService,
		renderCache:       renderCache,
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
		response.AccessErrAccessToken.Write(c)
		return
	}

	space, err := h.visibilityService.GetSpace(c.Request.Context(), spaceID, viewerUserID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrSpaceNotFound):
			response.AccessErrSpaceNotFound.Write(c)
		case errors.Is(err, service.ErrViewerLoginRequired):
			response.AccessErrLoginRequired.Write(c)
		case errors.Is(err, service.ErrSpaceAccessDenied):
			response.AccessErrInsufficientSpacePermission.Write(c)
		default:
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
		switch {
		case errors.Is(err, service.ErrInvalidVisibilityValue):
			response.AccessErrVisibilityPublicAuthenticatedMember.Write(c)
		case errors.Is(err, service.ErrViewerLoginRequired):
			response.AccessErrLoginRequired.Write(c)
		case errors.Is(err, service.ErrSpaceNotFound):
			response.AccessErrSpaceNotFound.Write(c)
		case errors.Is(err, service.ErrSpaceAccessDenied):
			response.AccessErrOnlyOwnerCanUpdateSpaceVisibility.Write(c)
		default:
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
		response.AccessErrAccessToken.Write(c)
		return
	}

	document, err := h.visibilityService.GetDocument(c.Request.Context(), documentID, viewerUserID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrDocumentNotFound):
			response.AccessErrDocumentNotFound.Write(c)
		case errors.Is(err, service.ErrViewerLoginRequired):
			response.AccessErrLoginRequired.Write(c)
		case errors.Is(err, service.ErrDocumentAccessDenied):
			response.AccessErrInsufficientDocumentPermission.Write(c)
		default:
			response.InternalError(c)
		}
		return
	}

	response.JSON(c, http.StatusOK, documentAccessResponse{
		ID:         document.DocumentID,
		NodeID:     document.NodeID,
		ThemeID:    document.ThemeID,
		Visibility: string(document.Visibility),
		Title:      document.Title,
		ContentMD:  document.ContentMD,
		Version:    document.Version,
		UpdatedAt:  time.Now().UTC().Format(time.RFC3339Nano),
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
		response.AccessErrVisibilityRequired.Write(c)
		return
	}

	document, err := h.visibilityService.UpdateDocumentVisibility(c.Request.Context(), documentID, actorUserID, req.Visibility)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidVisibilityValue):
			response.AccessErrVisibilityPublicAuthenticatedMember.Write(c)
		case errors.Is(err, service.ErrViewerLoginRequired):
			response.AccessErrLoginRequired.Write(c)
		case errors.Is(err, service.ErrDocumentNotFound):
			response.AccessErrDocumentNotFound.Write(c)
		case errors.Is(err, service.ErrDocumentAccessDenied):
			response.AccessErrInsufficientDocumentPermission.Write(c)
		default:
			response.InternalError(c)
		}
		return
	}

	if h != nil && h.renderCache != nil {
		// 文档可见性变化会直接影响阅读页输出，需主动失效渲染缓存。
		h.renderCache.PurgeDoc(document.DocumentID)
	}

	response.JSON(c, http.StatusOK, documentAccessResponse{
		ID:         document.DocumentID,
		NodeID:     document.NodeID,
		ThemeID:    document.ThemeID,
		Visibility: string(document.Visibility),
		Title:      document.Title,
		ContentMD:  document.ContentMD,
		Version:    document.Version,
		UpdatedAt:  time.Now().UTC().Format(time.RFC3339Nano),
	})
}

func (h *accessHandler) requireViewerUserID(c *gin.Context) (string, bool) {
	viewerUserID, err := h.resolveRequiredViewerUserID(c)
	if err != nil {
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
