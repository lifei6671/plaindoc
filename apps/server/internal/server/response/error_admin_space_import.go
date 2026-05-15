package response

import "net/http"

// 当前文件收敛空间导入接口错误响应模板。
var (
	AdminSpaceImportErrFileRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidUploadFile,
		Message: "导入文件不能为空",
	}
	AdminSpaceImportErrZipInvalid = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidUploadFile,
		Message: "导入 zip 无效",
	}
	AdminSpaceImportErrManifestMissing = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "导入包缺少 manifest",
	}
	AdminSpaceImportErrTreeMissing = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "导入包缺少 tree",
	}
	AdminSpaceImportErrPackageUnsupported = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "不支持的导入包格式",
	}
	AdminSpaceImportErrPackageNotImportable = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "导入包不可导入",
	}
	AdminSpaceImportErrStagingNotFound = ErrorTemplate{
		Status:  http.StatusNotFound,
		Code:    CodeFileNotFound,
		Message: "导入暂存不存在",
	}
	AdminSpaceImportErrStagingExpired = ErrorTemplate{
		Status:  http.StatusGone,
		Code:    CodeFileNotFound,
		Message: "导入暂存已过期",
	}
	AdminSpaceImportErrJobRunningLimit = ErrorTemplate{
		Status:  http.StatusTooManyRequests,
		Code:    CodeInvalidRequest,
		Message: "导入任务正在执行，请稍后再试",
	}
	AdminSpaceImportErrJobTokenInvalid = ErrorTemplate{
		Status:  http.StatusForbidden,
		Code:    CodeForbidden,
		Message: "导入任务令牌无效",
	}
	AdminSpaceImportErrCommitForbidden = ErrorTemplate{
		Status:  http.StatusForbidden,
		Code:    CodeForbidden,
		Message: "无权导入空间",
	}
)
