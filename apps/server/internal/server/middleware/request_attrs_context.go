package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/logit"
)

// RequestAttrsContext 在请求入口初始化日志属性容器，供整条链路共享。
func RequestAttrsContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := logit.WithRequestAttrsContainer(c.Request.Context())
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
