package response

import "net/http"

// 当前文件收敛空间导出接口错误响应模板。
var (
	AdminSpaceExportErrSpaceIDRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidSpaceID,
		Message: "空间 ID 不能为空",
	}
	AdminSpaceExportErrRequestBody = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "请求体无效",
	}
	AdminSpaceExportErrFormatUnsupported = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "不支持的导出格式",
	}
	AdminSpaceExportErrJobNotFound = ErrorTemplate{
		Status:  http.StatusNotFound,
		Code:    CodeFileNotFound,
		Message: "导出任务不存在",
	}
	AdminSpaceExportErrJobTokenInvalid = ErrorTemplate{
		Status:  http.StatusForbidden,
		Code:    CodeForbidden,
		Message: "导出任务令牌无效",
	}
	AdminSpaceExportErrJobRunningLimit = ErrorTemplate{
		Status:  http.StatusTooManyRequests,
		Code:    CodeInvalidRequest,
		Message: "导出任务正在执行，请稍后再试",
	}
	AdminSpaceExportErrFileNotReady = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "导出文件尚未生成",
	}
	AdminSpaceExportErrFileExpired = ErrorTemplate{
		Status:  http.StatusGone,
		Code:    CodeFileNotFound,
		Message: "导出文件已过期",
	}
	AdminSpaceExportErrDownloadForbidden = ErrorTemplate{
		Status:  http.StatusForbidden,
		Code:    CodeForbidden,
		Message: "无权下载该导出文件",
	}
)
