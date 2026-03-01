package response

import "net/http"

// 当前文件收敛 `admin_document_attachment.go` 的错误响应模板。
// 统一在 response 层维护 status/code/message，避免在 handler 中散落字面量。
var (
	// AdminDocumentAttachmentErrAdminActorMissing 对应场景：admin actor is missing
	AdminDocumentAttachmentErrAdminActorMissing = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "缺少管理员身份",
	}
	// AdminDocumentAttachmentErrPagePositiveInteger 对应场景：page must be a positive integer
	AdminDocumentAttachmentErrPagePositiveInteger = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidPage,
		Message: "页码参数（page）必须是正整数",
	}
	// AdminDocumentAttachmentErrPageSizePositiveInteger 对应场景：pageSize must be a positive integer
	AdminDocumentAttachmentErrPageSizePositiveInteger = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidPageSize,
		Message: "每页数量参数（pageSize）必须是正整数",
	}
	// AdminDocumentAttachmentErrAttachmentIDRequired 对应场景：attachment id is required
	AdminDocumentAttachmentErrAttachmentIDRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "附件 ID 不能为空",
	}
)
