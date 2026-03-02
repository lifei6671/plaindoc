package response

import "net/http"

// 当前文件收敛 `admin_search_analyzer.go` 的错误响应模板。
var (
	// AdminSearchAnalyzerErrAdminActorMissing 对应场景：admin actor is missing
	AdminSearchAnalyzerErrAdminActorMissing = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "缺少管理员身份",
	}
	// AdminSearchAnalyzerErrRequestBody 对应场景：invalid request body
	AdminSearchAnalyzerErrRequestBody = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "请求体格式错误",
	}
	// AdminSearchAnalyzerErrAnalyzerRequired 对应场景：analyzer is required
	AdminSearchAnalyzerErrAnalyzerRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidSource,
		Message: "分词器名称不能为空",
	}
	// AdminSearchAnalyzerErrDictEntryIDRequired 对应场景：dict entry id is required
	AdminSearchAnalyzerErrDictEntryIDRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "词条 ID 不能为空",
	}
	// AdminSearchAnalyzerErrPagePositiveInteger 对应场景：page must be a positive integer
	AdminSearchAnalyzerErrPagePositiveInteger = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidPage,
		Message: "页码参数（page）必须是正整数",
	}
	// AdminSearchAnalyzerErrPageSizePositiveInteger 对应场景：pageSize must be a positive integer
	AdminSearchAnalyzerErrPageSizePositiveInteger = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidPageSize,
		Message: "每页数量参数（pageSize）必须是正整数",
	}
)
