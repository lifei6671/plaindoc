package response

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/observability"
)

// RequestIDContextKey 用于在请求生命周期内传递 request_id。
const RequestIDContextKey = "request_id"

// ErrorBody 定义统一错误体，便于前端和日志系统稳定解析。
type ErrorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestId,omitempty"`
}

// RequestIDFromContext 读取中间件写入的 request_id，缺失时返回空字符串。
func RequestIDFromContext(c *gin.Context) string {
	value, exists := c.Get(RequestIDContextKey)
	if !exists {
		return ""
	}

	requestID, ok := value.(string)
	if !ok {
		return ""
	}

	return requestID
}

// Error 统一输出 JSON 错误响应，并自动附带 request_id。
func Error(c *gin.Context, status int, code string, message string) {
	// 中文注释：错误信息进入请求容器，便于请求结束时统一输出结构化日志。
	observability.SetRequestAttrs(c.Request.Context(),
		slog.Int("error_status", status),
		slog.String("error_code", code),
		slog.String("error_message", message),
	)

	c.AbortWithStatusJSON(status, ErrorBody{
		Code:      code,
		Message:   message,
		RequestID: RequestIDFromContext(c),
	})
}

// InternalError 用于服务端兜底错误，避免暴露内部细节。
func InternalError(c *gin.Context) {
	Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
}
