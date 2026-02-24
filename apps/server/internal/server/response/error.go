package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/logit"
)

// RequestIDContextKey 用于在请求生命周期内传递 request_id。
const RequestIDContextKey = "request_id"

const (
	// SuccessCode 统一成功响应码。
	SuccessCode = 0
	// SuccessMessage 统一成功响应描述。
	SuccessMessage = "success"
)

// JsonResult 定义统一 JSON 返回结构。
type JsonResult[T any] struct {
	Code      int    `json:"code"`
	Message   string `json:"message,omitempty"`
	RequestID string `json:"requestId,omitempty"`
	Data      T      `json:"data"`
}

// MappedError 定义可直接映射为统一 API 错误响应的错误接口。
type MappedError interface {
	error
	HTTPStatus() int
	ErrorCode() string
	ErrorMessage() string
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

// JSON 统一输出 JSON 成功响应，并自动附带 request_id。
func JSON[T any](c *gin.Context, _ int, data T) {
	c.JSON(http.StatusOK, JsonResult[T]{
		Code:      SuccessCode,
		Message:   SuccessMessage,
		RequestID: RequestIDFromContext(c),
		Data:      data,
	})
}

// Error 统一输出 JSON 错误响应，并自动附带 request_id。
func Error(c *gin.Context, status int, code string, message string) {
	writeError(c, NormalizeErrorHTTPStatus(status), code, message)
}

// ErrorWithStatus 直接使用传入的 HTTP 状态码输出错误响应。
func ErrorWithStatus(c *gin.Context, status int, code string, message string) {
	writeError(c, preserveErrorHTTPStatus(status), code, message)
}

func writeError(c *gin.Context, httpStatus int, code string, message string) {
	errorCode := ResolveErrorCode(code)

	// 中文注释：错误信息进入请求容器，便于请求结束时统一输出结构化日志。
	logit.SetRequestAttrs(c.Request.Context(),
		logit.Int("error_status", httpStatus),
		logit.Int("error_code", errorCode),
		logit.String("error_key", code),
		logit.String("error_message", message),
	)

	c.AbortWithStatusJSON(httpStatus, JsonResult[any]{
		Code:      errorCode,
		Message:   message,
		RequestID: RequestIDFromContext(c),
		Data:      nil,
	})
}

func preserveErrorHTTPStatus(status int) int {
	if status >= http.StatusBadRequest && status <= 599 {
		return status
	}
	return http.StatusInternalServerError
}

// InternalError 用于服务端兜底错误，避免暴露内部细节。
func InternalError(c *gin.Context) {
	Error(c, http.StatusInternalServerError, CodeInternalError, "internal server error")
}

// FromError 优先按业务错误映射输出；无法识别时回落到 InternalError。
func FromError(c *gin.Context, err error) {
	if err == nil {
		return
	}

	var mappedErr MappedError
	if errors.As(err, &mappedErr) {
		Error(c, mappedErr.HTTPStatus(), mappedErr.ErrorCode(), mappedErr.ErrorMessage())
		return
	}
	InternalError(c)
}
