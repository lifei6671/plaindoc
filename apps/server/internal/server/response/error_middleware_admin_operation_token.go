package response

import "net/http"

// 当前文件收敛 `middleware/admin_operation_token.go` 的错误响应模板。
var (
	// MiddlewareAdminOperationTokenErrAdminActorMissing 对应场景：admin actor is missing
	MiddlewareAdminOperationTokenErrAdminActorMissing = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "admin actor is missing",
	}
)
