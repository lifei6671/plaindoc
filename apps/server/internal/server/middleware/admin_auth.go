package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
)

const (
	adminActorUserIDContextKey = "admin_actor_user_id"
)

// RequireAdmin 确保请求方为已登录管理员。
func RequireAdmin(
	authService *service.AuthService,
	adminAccessService *service.AdminAccessService,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		if authService == nil || adminAccessService == nil {
			response.InternalError(c)
			return
		}

		rawToken, ok := bearerTokenFromRequest(c)
		if !ok {
			response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "authorization token is required")
			return
		}

		session, err := authService.Me(c.Request.Context(), rawToken)
		if err != nil {
			response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid access token")
			return
		}

		isAdmin, err := adminAccessService.IsAdmin(c.Request.Context(), session.User.ID)
		if err != nil {
			response.InternalError(c)
			return
		}
		if !isAdmin {
			response.Error(c, http.StatusForbidden, "FORBIDDEN", "admin role is required")
			return
		}

		c.Set(adminActorUserIDContextKey, session.User.ID)
		c.Next()
	}
}

// RequireSpaceManagement 确保管理员对指定空间具备管理权限。
func RequireSpaceManagement(
	adminAccessService *service.AdminAccessService,
	spaceIDParam string,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		if adminAccessService == nil {
			response.InternalError(c)
			return
		}

		rawActorUserID, exists := c.Get(adminActorUserIDContextKey)
		if !exists {
			response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "admin actor is missing")
			return
		}
		actorUserID, ok := rawActorUserID.(string)
		if !ok || strings.TrimSpace(actorUserID) == "" {
			response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "admin actor is invalid")
			return
		}

		spaceID := strings.TrimSpace(c.Param(spaceIDParam))
		if spaceID == "" {
			response.Error(c, http.StatusBadRequest, "INVALID_SPACE_ID", "space id is required")
			return
		}

		allowed, err := adminAccessService.CanManageSpace(c.Request.Context(), actorUserID, spaceID)
		if err != nil {
			response.InternalError(c)
			return
		}
		if !allowed {
			response.Error(c, http.StatusForbidden, "FORBIDDEN", "insufficient space admin permission")
			return
		}

		c.Next()
	}
}

// RequirePlatformAdmin 确保管理员具备平台管理权限。
func RequirePlatformAdmin(adminAccessService *service.AdminAccessService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if adminAccessService == nil {
			response.InternalError(c)
			return
		}

		actorUserID, err := AdminActorUserID(c)
		if err != nil {
			response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "admin actor is missing")
			return
		}

		allowed, err := adminAccessService.IsPlatformAdmin(c.Request.Context(), actorUserID)
		if err != nil {
			response.InternalError(c)
			return
		}
		if !allowed {
			response.Error(c, http.StatusForbidden, "FORBIDDEN", "platform admin role is required")
			return
		}

		c.Next()
	}
}

// AdminActorUserID 读取管理员 actor user id。
func AdminActorUserID(c *gin.Context) (string, error) {
	rawValue, exists := c.Get(adminActorUserIDContextKey)
	if !exists {
		return "", errors.New("admin actor is missing")
	}
	value, ok := rawValue.(string)
	if !ok || strings.TrimSpace(value) == "" {
		return "", errors.New("admin actor is invalid")
	}
	return value, nil
}

func bearerTokenFromRequest(c *gin.Context) (string, bool) {
	rawAuthorization := strings.TrimSpace(c.GetHeader("Authorization"))
	if rawAuthorization == "" {
		return "", false
	}
	parts := strings.SplitN(rawAuthorization, " ", 2)
	if len(parts) != 2 {
		return "", false
	}
	if !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", false
	}
	return token, true
}
