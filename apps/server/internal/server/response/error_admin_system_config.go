package response

import "net/http"

// 当前文件收敛 `admin_system_config.go` 的错误响应模板。
// 统一在 response 层维护 status/code/message，避免在 handler 中散落字面量。
var (
	// AdminSystemConfigErrAdminActorMissing 对应场景：admin actor is missing
	AdminSystemConfigErrAdminActorMissing = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "admin actor is missing",
	}
	// AdminSystemConfigErrConfigKeyRequired 对应场景：config key is required
	AdminSystemConfigErrConfigKeyRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidConfigKey,
		Message: "config key is required",
	}
	// AdminSystemConfigErrRequestBody 对应场景：invalid request body
	AdminSystemConfigErrRequestBody = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "invalid request body",
	}
)
