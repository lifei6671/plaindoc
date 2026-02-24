package response

import "net/http"

// 当前文件收敛 `admin_document.go` 的错误响应模板。
// 统一在 response 层维护 status/code/message，避免在 handler 中散落字面量。
var (
	// AdminDocumentErrAdminActorMissing 对应场景：admin actor is missing
	AdminDocumentErrAdminActorMissing = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "admin actor is missing",
	}
	// AdminDocumentErrPagePositiveInteger 对应场景：page must be a positive integer
	AdminDocumentErrPagePositiveInteger = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidPage,
		Message: "page must be a positive integer",
	}
	// AdminDocumentErrPageSizePositiveInteger 对应场景：pageSize must be a positive integer
	AdminDocumentErrPageSizePositiveInteger = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidPageSize,
		Message: "pageSize must be a positive integer",
	}
	// AdminDocumentErrDocumentIDRequired 对应场景：document id is required
	AdminDocumentErrDocumentIDRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidDocumentID,
		Message: "document id is required",
	}
	// AdminDocumentErrRequestBody 对应场景：invalid request body
	AdminDocumentErrRequestBody = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "invalid request body",
	}
)
