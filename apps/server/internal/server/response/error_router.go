package response

import "net/http"

// 当前文件收敛 `router.go` 的兜底路由错误响应模板。
var (
	// RouterErrRouteNotFound 对应场景：route not found
	RouterErrRouteNotFound = ErrorTemplate{
		Status:  http.StatusNotFound,
		Code:    CodeRouteNotFound,
		Message: "路由不存在",
	}
	// RouterErrMethodNotAllowed 对应场景：method not allowed
	RouterErrMethodNotAllowed = ErrorTemplate{
		Status:  http.StatusMethodNotAllowed,
		Code:    CodeMethodNotAllowed,
		Message: "请求方法不被允许",
	}
)
