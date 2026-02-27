package response

import "net/http"

// 当前文件收敛 `admin_theme.go` 的错误响应模板。
// 统一在 response 层维护 status/code/message，避免在 handler 中散落字面量。
var (
	// AdminThemeErrAdminActorMissing 对应场景：admin actor is missing
	AdminThemeErrAdminActorMissing = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "缺少管理员身份",
	}
	// AdminThemeErrRequestBody 对应场景：invalid request body
	AdminThemeErrRequestBody = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "请求体无效",
	}
	// AdminThemeErrThemeIDRequired 对应场景：theme id is required
	AdminThemeErrThemeIDRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidThemeID,
		Message: "主题 ID 不能为空",
	}
)
