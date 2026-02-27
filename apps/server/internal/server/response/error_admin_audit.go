package response

import "net/http"

// 当前文件收敛 `admin_audit.go` 的错误响应模板。
// 统一在 response 层维护 status/code/message，避免在 handler 中散落字面量。
var (
	// AdminAuditErrAdminActorMissing 对应场景：admin actor is missing
	AdminAuditErrAdminActorMissing = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "缺少管理员身份",
	}
	// AdminAuditErrPagePositiveInteger 对应场景：page must be a positive integer
	AdminAuditErrPagePositiveInteger = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidPage,
		Message: "页码参数（page）必须是正整数",
	}
	// AdminAuditErrPageSizePositiveInteger 对应场景：pageSize must be a positive integer
	AdminAuditErrPageSizePositiveInteger = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidPageSize,
		Message: "每页数量参数（pageSize）必须是正整数",
	}
	// AdminAuditErrRFC3339Datetime 对应场景：from must be RFC3339 datetime
	AdminAuditErrRFC3339Datetime = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidFrom,
		Message: "起始时间参数（from）必须是 RFC3339 时间格式",
	}
	// AdminAuditErrInvalidToRFC3339Datetime 对应场景：to must be RFC3339 datetime
	AdminAuditErrInvalidToRFC3339Datetime = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidTo,
		Message: "结束时间参数（to）必须是 RFC3339 时间格式",
	}
)
