package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
)

// AttachAdminAuditContext 在后台请求链路中注入统一审计上下文元信息。
func AttachAdminAuditContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		actorUserID, err := AdminActorUserID(c)
		if err != nil {
			response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "admin actor is missing")
			return
		}

		ctx := service.WithAdminAuditMeta(c.Request.Context(), actorUserID, response.RequestIDFromContext(c))
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
