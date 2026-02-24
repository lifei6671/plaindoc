package response

import "net/http"

// 当前文件收敛 `theme.go` 的错误响应模板。
// 统一在 response 层维护 status/code/message，避免在 handler 中散落字面量。
var (
	// ThemeErrDocumentIDRequired 对应场景：document id is required
	ThemeErrDocumentIDRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidDocumentID,
		Message: "document id is required",
	}
	// ThemeErrThemeIDRequired 对应场景：themeId is required
	ThemeErrThemeIDRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "themeId is required",
	}
	// ThemeErrInvalidThemeIdThemeIDRequired 对应场景：theme id is required
	ThemeErrInvalidThemeIdThemeIDRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidThemeID,
		Message: "theme id is required",
	}
	// ThemeErrThemeNotFound 对应场景：theme not found
	ThemeErrThemeNotFound = ErrorTemplate{
		Status:  http.StatusNotFound,
		Code:    CodeThemeNotFound,
		Message: "theme not found",
	}
	// ThemeErrDocumentNotFound 对应场景：document not found
	ThemeErrDocumentNotFound = ErrorTemplate{
		Status:  http.StatusNotFound,
		Code:    CodeDocumentNotFound,
		Message: "document not found",
	}
)
