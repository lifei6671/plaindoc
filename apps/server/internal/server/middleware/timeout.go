package middleware

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
)

// Timeout 为请求注入 deadline，供数据库与下游调用统一感知超时。
func Timeout(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if timeout <= 0 {
			c.Next()
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)

		c.Next()

		// 复杂分支：当处理链路未主动写响应且已经超时，返回统一 504。
		if c.Writer.Written() {
			return
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			response.Error(c, http.StatusGatewayTimeout, "REQUEST_TIMEOUT", "request timeout")
		}
	}
}
