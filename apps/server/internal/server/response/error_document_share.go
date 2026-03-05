package response

import "net/http"

// 当前文件收敛文档分享相关错误响应模板。
var (
	DocumentShareErrDocumentIDRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidDocumentID,
		Message: "文档 ID 不能为空",
	}
	DocumentShareErrSpaceIDRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidSpaceID,
		Message: "空间 ID 不能为空",
	}
	DocumentShareErrShareNotFound = ErrorTemplate{
		Status:  http.StatusNotFound,
		Code:    CodeDocumentShareNotFound,
		Message: "分享不存在",
	}
	DocumentShareErrShareExpired = ErrorTemplate{
		Status:  http.StatusGone,
		Code:    CodeDocumentShareExpired,
		Message: "分享已过期",
	}
	DocumentShareErrShareDisabled = ErrorTemplate{
		Status:  http.StatusNotFound,
		Code:    CodeDocumentShareDisabled,
		Message: "分享已取消",
	}
	DocumentShareErrPasswordRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeDocumentSharePasswordRequired,
		Message: "请输入访问密码",
	}
	DocumentShareErrPasswordTooShort = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeDocumentSharePasswordTooShort,
		Message: "密码长度至少 6 位",
	}
	DocumentShareErrPasswordInvalid = ErrorTemplate{
		Status:  http.StatusForbidden,
		Code:    CodeDocumentSharePasswordInvalid,
		Message: "密码错误",
	}
	DocumentShareErrAccessDenied = ErrorTemplate{
		Status:  http.StatusForbidden,
		Code:    CodeDocumentShareAccessDenied,
		Message: "无权访问该分享",
	}
	DocumentShareErrInvalidMode = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "分享模式无效",
	}
	DocumentShareErrAttachmentIDRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "附件 ID 不能为空",
	}
	DocumentShareErrAttachmentPreviewUnsupported = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "该附件不支持在线预览",
	}
)
