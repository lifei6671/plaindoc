package response

import "net/http"

// 当前文件收敛 `admin.go` 的错误响应模板。
// 统一在 response 层维护 status/code/message，避免在 handler 中散落字面量。
var (
	// AdminErrAdminActorMissing 对应场景：admin actor is missing
	AdminErrAdminActorMissing = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "缺少管理员身份",
	}
	// AdminErrAdminUserNotFound 对应场景：admin user not found
	AdminErrAdminUserNotFound = ErrorTemplate{
		Status:  http.StatusNotFound,
		Code:    CodeUserNotFound,
		Message: "管理员用户不存在",
	}
	// AdminErrSpaceIDRequired 对应场景：space id is required
	AdminErrSpaceIDRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidSpaceID,
		Message: "空间 ID 不能为空",
	}
)
