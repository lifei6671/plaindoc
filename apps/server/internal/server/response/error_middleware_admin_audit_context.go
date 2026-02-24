package response

import "net/http"

// 当前文件收敛 `middleware/admin_audit_context.go` 的错误响应模板。
var (
	// MiddlewareAdminAuditContextErrAdminActorMissing 对应场景：admin actor is missing
	// 审计上下文依赖管理员身份，缺失时直接返回未授权错误。
	MiddlewareAdminAuditContextErrAdminActorMissing = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "admin actor is missing",
	}
)
