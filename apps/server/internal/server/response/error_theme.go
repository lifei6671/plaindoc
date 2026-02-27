package response

import "net/http"

// 当前文件收敛 `theme.go` 的错误响应模板。
// 统一在 response 层维护 status/code/message，避免在 handler 中散落字面量。
var (
	// ThemeErrDocumentIDRequired 对应场景：document id is required
	ThemeErrDocumentIDRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidDocumentID,
		Message: "文档 ID 不能为空",
	}
	// ThemeErrThemeIDRequired 对应场景：themeId is required
	ThemeErrThemeIDRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "themeId 参数不能为空",
	}
	// ThemeErrInvalidThemeIdThemeIDRequired 对应场景：theme id is required
	ThemeErrInvalidThemeIdThemeIDRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidThemeID,
		Message: "主题 ID 不能为空",
	}
	// ThemeErrThemeNotFound 对应场景：theme not found
	ThemeErrThemeNotFound = ErrorTemplate{
		Status:  http.StatusNotFound,
		Code:    CodeThemeNotFound,
		Message: "主题不存在",
	}
	// ThemeErrDocumentNotFound 对应场景：document not found
	ThemeErrDocumentNotFound = ErrorTemplate{
		Status:  http.StatusNotFound,
		Code:    CodeDocumentNotFound,
		Message: "文档不存在",
	}
)
