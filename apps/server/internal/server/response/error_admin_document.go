package response

import "net/http"

// 当前文件收敛 `admin_document.go` 的错误响应模板。
// 统一在 response 层维护 status/code/message，避免在 handler 中散落字面量。
var (
	// AdminDocumentErrAdminActorMissing 对应场景：admin actor is missing
	AdminDocumentErrAdminActorMissing = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "缺少管理员身份",
	}
	// AdminDocumentErrPagePositiveInteger 对应场景：page must be a positive integer
	AdminDocumentErrPagePositiveInteger = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidPage,
		Message: "页码参数（page）必须是正整数",
	}
	// AdminDocumentErrPageSizePositiveInteger 对应场景：pageSize must be a positive integer
	AdminDocumentErrPageSizePositiveInteger = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidPageSize,
		Message: "每页数量参数（pageSize）必须是正整数",
	}
	// AdminDocumentErrDocumentIDRequired 对应场景：document id is required
	AdminDocumentErrDocumentIDRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidDocumentID,
		Message: "文档 ID 不能为空",
	}
	// AdminDocumentErrRequestBody 对应场景：invalid request body
	AdminDocumentErrRequestBody = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "请求体无效",
	}
)
