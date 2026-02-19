package middleware

import (
	"log/slog"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/observability"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
)

// AccessLog 在请求结束时统一落一条日志，并合并请求上下文中的动态属性。
func AccessLog(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = c.Request.URL.Path
		}

		status := c.Writer.Status()
		level := slog.LevelInfo
		if status >= 500 {
			level = slog.LevelError
		} else if status >= 400 {
			level = slog.LevelWarn
		}

		baseAttrs := []slog.Attr{
			slog.String("request_id", response.RequestIDFromContext(c)),
			slog.String("method", c.Request.Method),
			slog.String("route", route),
			slog.Int("status", status),
			slog.Duration("latency", time.Since(start)),
			slog.String("client_ip", c.ClientIP()),
			slog.String("user_agent", c.Request.UserAgent()),
		}

		requestAttrs := observability.SnapshotRequestAttrs(c.Request.Context())
		attrs := mergeAttrs(baseAttrs, requestAttrs)

		logger.LogAttrs(c.Request.Context(), level, "request completed", attrs...)
	}
}

func mergeAttrs(base []slog.Attr, overlay []slog.Attr) []slog.Attr {
	merged := make(map[string]slog.Attr, len(base)+len(overlay))
	for _, attr := range base {
		if attr.Key == "" {
			continue
		}
		merged[attr.Key] = attr
	}
	for _, attr := range overlay {
		if attr.Key == "" {
			continue
		}
		merged[attr.Key] = attr
	}

	keys := make([]string, 0, len(merged))
	for key := range merged {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result := make([]slog.Attr, 0, len(keys))
	for _, key := range keys {
		result = append(result, merged[key])
	}
	return result
}
