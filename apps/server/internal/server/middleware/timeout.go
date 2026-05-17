package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"
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
		if isServerSentEventRequest(c.Request) {
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
			response.MiddlewareTimeoutErrRequestTimeout.Write(c)
		}
	}
}

func isServerSentEventRequest(request *http.Request) bool {
	if request == nil {
		return false
	}
	if request.Method != http.MethodGet {
		return false
	}
	accept := strings.ToLower(request.Header.Get("Accept"))
	if !strings.Contains(accept, "text/event-stream") {
		return false
	}
	return isAdminSpaceTransferSSEPath(request.URL.Path)
}

func isAdminSpaceTransferSSEPath(rawPath string) bool {
	segments := strings.Split(strings.Trim(rawPath, "/"), "/")
	// 只允许后台空间导出 SSE 跳过业务 timeout：
	// /api/admin/spaces/:spaceId/exports/:jobId/events
	if len(segments) == 7 &&
		segments[0] == "api" &&
		segments[1] == "admin" &&
		segments[2] == "spaces" &&
		strings.TrimSpace(segments[3]) != "" &&
		segments[4] == "exports" &&
		strings.TrimSpace(segments[5]) != "" &&
		segments[6] == "events" {
		return true
	}
	// 只允许后台空间导入 SSE 跳过业务 timeout：
	// /api/admin/space-imports/:jobId/events
	if len(segments) == 5 &&
		segments[0] == "api" &&
		segments[1] == "admin" &&
		segments[2] == "space-imports" &&
		strings.TrimSpace(segments[3]) != "" &&
		segments[4] == "events" {
		return true
	}
	return false
}
