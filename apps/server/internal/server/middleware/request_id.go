package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/observability"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
)

const requestIDHeader = "X-Request-Id"

// RequestID 为每个请求注入 request_id，便于日志与错误响应关联定位。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(requestIDHeader)
		if requestID == "" {
			requestID = generateRequestID()
		}

		c.Set(response.RequestIDContextKey, requestID)
		observability.SetRequestAttrs(c.Request.Context(), slog.String("request_id", requestID))
		c.Header(requestIDHeader, requestID)
		c.Next()
	}
}

func generateRequestID() string {
	buffer := make([]byte, 8)
	if _, err := rand.Read(buffer); err != nil {
		// 退化分支：随机源不可用时使用时间戳保证可追踪性。
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(buffer)
}
