package response

import "net/http"

// 当前文件收敛 `middleware/timeout.go` 的错误响应模板。
// 统一在 response 层维护 status/code/message，避免在中间件中散落字面量。
var (
	// MiddlewareTimeoutErrRequestTimeout 对应场景：request timeout
	// 当请求链路超时且下游尚未写入响应时，返回统一超时错误。
	MiddlewareTimeoutErrRequestTimeout = ErrorTemplate{
		Status:  http.StatusGatewayTimeout,
		Code:    CodeRequestTimeout,
		Message: "请求超时",
	}
)
