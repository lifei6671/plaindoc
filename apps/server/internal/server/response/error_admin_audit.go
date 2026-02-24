package response

import "net/http"

// 当前文件收敛 `admin_audit.go` 的错误响应模板。
// 统一在 response 层维护 status/code/message，避免在 handler 中散落字面量。
var (
	// AdminAuditErrAdminActorMissing 对应场景：admin actor is missing
	AdminAuditErrAdminActorMissing = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "admin actor is missing",
	}
	// AdminAuditErrPagePositiveInteger 对应场景：page must be a positive integer
	AdminAuditErrPagePositiveInteger = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidPage,
		Message: "page must be a positive integer",
	}
	// AdminAuditErrPageSizePositiveInteger 对应场景：pageSize must be a positive integer
	AdminAuditErrPageSizePositiveInteger = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidPageSize,
		Message: "pageSize must be a positive integer",
	}
	// AdminAuditErrRFC3339Datetime 对应场景：from must be RFC3339 datetime
	AdminAuditErrRFC3339Datetime = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidFrom,
		Message: "from must be RFC3339 datetime",
	}
	// AdminAuditErrInvalidToRFC3339Datetime 对应场景：to must be RFC3339 datetime
	AdminAuditErrInvalidToRFC3339Datetime = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidTo,
		Message: "to must be RFC3339 datetime",
	}
)
