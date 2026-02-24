package response

import "net/http"

// 当前文件收敛 `admin_user.go` 的错误响应模板。
// 统一在 response 层维护 status/code/message，避免在 handler 中散落字面量。
var (
	// AdminUserErrAdminActorMissing 对应场景：admin actor is missing
	AdminUserErrAdminActorMissing = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "admin actor is missing",
	}
	// AdminUserErrRequestBody 对应场景：invalid request body
	AdminUserErrRequestBody = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "invalid request body",
	}
	// AdminUserErrPagePositiveInteger 对应场景：page must be a positive integer
	AdminUserErrPagePositiveInteger = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidPage,
		Message: "page must be a positive integer",
	}
	// AdminUserErrPageSizePositiveInteger 对应场景：pageSize must be a positive integer
	AdminUserErrPageSizePositiveInteger = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidPageSize,
		Message: "pageSize must be a positive integer",
	}
	// AdminUserErrUserIDRequired 对应场景：user id is required
	AdminUserErrUserIDRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidUserID,
		Message: "user id is required",
	}
)
