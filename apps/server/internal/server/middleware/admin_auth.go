package middleware

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
)

const (
	adminActorUserIDContextKey = "admin_actor_user_id"
)

// RequireAdminSession 仅确保请求方已登录，并把当前用户写入后台 actor 上下文。
//
// 这个中间件用于后台壳页、个人信息页和成员态空间列表：
// 只要用户已登录，就允许进入后台，但后续业务层仍会按角色/能力继续收口。
func RequireAdminSession(authService *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if authService == nil {
			response.InternalError(c)
			return
		}

		actorUserID, ok := resolveAdminActorUserIDFromRequest(c, authService)
		if !ok {
			return
		}

		c.Set(adminActorUserIDContextKey, actorUserID)
		c.Next()
	}
}

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

		actorUserID, ok := resolveAdminActorUserIDFromRequest(c, authService)
		if !ok {
			return
		}

		isAdmin, err := adminAccessService.IsAdmin(c.Request.Context(), actorUserID)
		if err != nil {
			response.InternalError(c)
			return
		}
		if !isAdmin {
			response.MiddlewareAdminAuthErrAdminRoleRequired.Write(c)
			return
		}

		c.Set(adminActorUserIDContextKey, actorUserID)
		c.Next()
	}
}

func resolveAdminActorUserIDFromRequest(c *gin.Context, authService *service.AuthService) (string, bool) {
	rawToken, ok := bearerTokenFromRequest(c)
	if !ok {
		response.MiddlewareAdminAuthErrAuthorizationTokenRequired.Write(c)
		return "", false
	}

	session, err := authService.Me(c.Request.Context(), rawToken)
	if err != nil {
		response.MiddlewareAdminAuthErrInvalidAccessToken.Write(c)
		return "", false
	}

	return session.User.ID, true
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
			response.MiddlewareAdminAuthErrAdminActorMissing.Write(c)
			return
		}
		actorUserID, ok := rawActorUserID.(string)
		if !ok || strings.TrimSpace(actorUserID) == "" {
			response.MiddlewareAdminAuthErrAdminActorInvalid.Write(c)
			return
		}

		spaceID := strings.TrimSpace(c.Param(spaceIDParam))
		if spaceID == "" {
			response.MiddlewareAdminAuthErrSpaceIDRequired.Write(c)
			return
		}

		allowed, err := adminAccessService.CanManageSpace(c.Request.Context(), actorUserID, spaceID)
		if err != nil {
			response.InternalError(c)
			return
		}
		if !allowed {
			response.MiddlewareAdminAuthErrInsufficientSpaceAdminPermission.Write(c)
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
			response.MiddlewareAdminAuthErrAdminActorMissing.Write(c)
			return
		}

		allowed, err := adminAccessService.IsPlatformAdmin(c.Request.Context(), actorUserID)
		if err != nil {
			response.InternalError(c)
			return
		}
		if !allowed {
			response.MiddlewareAdminAuthErrPlatformAdminRoleRequired.Write(c)
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
	if rawAuthorization != "" {
		parts := strings.SplitN(rawAuthorization, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			token := strings.TrimSpace(parts[1])
			if token != "" {
				return token, true
			}
		}
	}

	tokenFromCookie, err := c.Cookie("accessToken")
	if err != nil {
		return "", false
	}
	token := strings.TrimSpace(tokenFromCookie)
	if token == "" {
		return "", false
	}

	return token, true
}
