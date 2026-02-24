package response

import "net/http"

// 当前文件收敛 `admin_operation_token.go` 的错误响应模板。
// 统一在 response 层维护 status/code/message，避免在 handler 中散落字面量。
var (
	// AdminOperationTokenErrAdminActorMissing 对应场景：admin actor is missing
	AdminOperationTokenErrAdminActorMissing = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "admin actor is missing",
	}
	// AdminOperationTokenErrRequestBody 对应场景：invalid request body
	AdminOperationTokenErrRequestBody = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "invalid request body",
	}
)
