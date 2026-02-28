package response

import "net/http"

// 文档附件链路错误模板。
var (
	DocumentAttachmentErrAttachmentIDRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "附件 ID 不能为空",
	}
	DocumentAttachmentErrUploadFileRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidUploadFile,
		Message: "附件文件不能为空",
	}
	DocumentAttachmentErrUploadFileEmpty = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidUploadFile,
		Message: "附件文件为空",
	}
	DocumentAttachmentErrUploadFileTooLarge = ErrorTemplate{
		Status:  http.StatusRequestEntityTooLarge,
		Code:    CodeFileTooLarge,
		Message: "附件文件超过大小限制",
	}
	DocumentAttachmentErrAttachmentNotFound = ErrorTemplate{
		Status:  http.StatusNotFound,
		Code:    CodeFileNotFound,
		Message: "附件不存在",
	}
	DocumentAttachmentErrInvalidPurpose = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "附件访问用途无效",
	}
	DocumentAttachmentErrDownloadLinkInvalid = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "附件下载链接无效",
	}
	DocumentAttachmentErrDownloadLinkExpired = ErrorTemplate{
		Status:  http.StatusForbidden,
		Code:    CodeForbidden,
		Message: "附件下载链接已过期",
	}
	DocumentAttachmentErrPreviewUnsupported = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "该附件类型暂不支持在线预览",
	}
)
