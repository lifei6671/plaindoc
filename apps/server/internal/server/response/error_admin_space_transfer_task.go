package response

import "net/http"

// 当前文件收敛全局空间导入/导出任务接口错误响应模板。
var (
	AdminSpaceTransferTaskErrNotFound = ErrorTemplate{
		Status:  http.StatusNotFound,
		Code:    CodeFileNotFound,
		Message: "空间传输任务不存在",
	}
	AdminSpaceTransferTaskErrKindUnsupported = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "不支持的空间传输任务类型",
	}
	AdminSpaceTransferTaskErrStreamURLUnavailable = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "任务订阅链接不可用",
	}
	AdminSpaceTransferTaskErrDownloadUnavailable = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "任务下载链接不可用",
	}
)
