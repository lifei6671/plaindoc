package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/middleware"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
)

type adminHandler struct {
	adminAccessService *service.AdminAccessService
}

type adminMeResponse struct {
	UserID string   `json:"userId"`
	Roles  []string `json:"roles"`
}

type adminSpaceCheckResponse struct {
	SpaceID   string `json:"spaceId"`
	CanManage bool   `json:"canManage"`
}

// NewAdminHandler 创建管理后台基础处理器。
func NewAdminHandler(adminAccessService *service.AdminAccessService) *adminHandler {
	return &adminHandler{
		adminAccessService: adminAccessService,
	}
}

// Me 返回当前管理员身份信息。
func (h *adminHandler) Me(c *gin.Context) {
	if h == nil || h.adminAccessService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "admin actor is missing")
		return
	}

	roles, err := h.adminAccessService.ListAdminRoles(c.Request.Context(), actorUserID)
	if err != nil {
		response.InternalError(c)
		return
	}

	payloadRoles := make([]string, 0, len(roles))
	for _, role := range roles {
		payloadRoles = append(payloadRoles, string(role))
	}

	c.JSON(http.StatusOK, adminMeResponse{
		UserID: actorUserID,
		Roles:  payloadRoles,
	})
}

// CheckSpace 返回当前管理员是否可管理指定空间。
func (h *adminHandler) CheckSpace(c *gin.Context) {
	if h == nil || h.adminAccessService == nil {
		response.InternalError(c)
		return
	}

	spaceID := c.Param("spaceId")
	if spaceID == "" {
		response.Error(c, http.StatusBadRequest, "INVALID_SPACE_ID", "space id is required")
		return
	}

	c.JSON(http.StatusOK, adminSpaceCheckResponse{
		SpaceID:   spaceID,
		CanManage: true,
	})
}
