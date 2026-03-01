package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/middleware"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

type adminHandler struct {
	adminAccessService *service.AdminAccessService
	userRepo           repository.UserRepository
}

type adminMeResponse struct {
	UserID    string   `json:"userId"`
	Email     string   `json:"email"`
	Name      string   `json:"name"`
	AvatarURL string   `json:"avatarUrl"`
	Roles     []string `json:"roles"`
}

type adminSpaceCheckResponse struct {
	SpaceID   string `json:"spaceId"`
	CanManage bool   `json:"canManage"`
}

// NewAdminHandler 创建管理后台基础处理器。
func NewAdminHandler(
	adminAccessService *service.AdminAccessService,
	userRepo repository.UserRepository,
) *adminHandler {
	return &adminHandler{
		adminAccessService: adminAccessService,
		userRepo:           userRepo,
	}
}

// Me 返回当前管理员身份信息。
func (h *adminHandler) Me(c *gin.Context) {
	if h == nil || h.adminAccessService == nil || h.userRepo == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		setRequestErrmsg(c, err, "解析管理员身份失败")
		response.AdminErrAdminActorMissing.Write(c)
		return
	}

	roles, err := h.adminAccessService.ListAdminRoles(c.Request.Context(), actorUserID)
	if err != nil {
		setRequestErrmsg(c, err, "获取管理员角色失败")
		response.InternalError(c)
		return
	}
	user, err := h.userRepo.GetByUserID(c.Request.Context(), actorUserID)
	if err != nil {
		setRequestErrmsg(c, err, "获取管理员用户信息失败")
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.AdminErrAdminUserNotFound.Write(c)
			return
		}
		response.InternalError(c)
		return
	}

	payloadRoles := make([]string, 0, len(roles))
	for _, role := range roles {
		payloadRoles = append(payloadRoles, string(role))
	}

	response.JSON(c, http.StatusOK, adminMeResponse{
		UserID:    actorUserID,
		Email:     strings.TrimSpace(user.Email),
		Name:      strings.TrimSpace(user.Name),
		AvatarURL: strings.TrimSpace(user.AvatarURL),
		Roles:     payloadRoles,
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
		setRequestErrmsg(c, nil, "空间 ID 不能为空")
		response.AdminErrSpaceIDRequired.Write(c)
		return
	}

	response.JSON(c, http.StatusOK, adminSpaceCheckResponse{
		SpaceID:   spaceID,
		CanManage: true,
	})
}
