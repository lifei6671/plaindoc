package middleware

import (
	"fmt"
	"log/slog"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/observability"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
)

// Recovery 捕获未处理 panic，保证返回统一错误结构而不是连接中断。
func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		// 中文注释：panic 信息先聚合到请求容器，最终由 AccessLog 统一输出。
		observability.SetRequestAttrs(c.Request.Context(),
			slog.Bool("panic", true),
			slog.String("panic_message", fmt.Sprintf("%v", recovered)),
			slog.String("panic_stack", string(debug.Stack())),
			slog.String("panic_path", c.Request.URL.Path),
		)
		response.InternalError(c)
	})
}
