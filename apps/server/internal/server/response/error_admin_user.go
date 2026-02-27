package response

import "net/http"

// 当前文件收敛 `admin_user.go` 的错误响应模板。
// 统一在 response 层维护 status/code/message，避免在 handler 中散落字面量。
var (
	// AdminUserErrAdminActorMissing 对应场景：admin actor is missing
	AdminUserErrAdminActorMissing = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "缺少管理员身份",
	}
	// AdminUserErrRequestBody 对应场景：invalid request body
	AdminUserErrRequestBody = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "请求体无效",
	}
	// AdminUserErrPagePositiveInteger 对应场景：page must be a positive integer
	AdminUserErrPagePositiveInteger = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidPage,
		Message: "页码参数（page）必须是正整数",
	}
	// AdminUserErrPageSizePositiveInteger 对应场景：pageSize must be a positive integer
	AdminUserErrPageSizePositiveInteger = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidPageSize,
		Message: "每页数量参数（pageSize）必须是正整数",
	}
	// AdminUserErrUserIDRequired 对应场景：user id is required
	AdminUserErrUserIDRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidUserID,
		Message: "用户 ID 不能为空",
	}
)
