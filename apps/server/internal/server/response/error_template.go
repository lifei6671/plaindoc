package response

import "github.com/gin-gonic/gin"

// ErrorTemplate 定义可复用的响应错误模板。
type ErrorTemplate struct {
	Status  int
	Code    string
	Message string
}

// Write 将模板输出为统一错误响应。
func (e ErrorTemplate) Write(c *gin.Context) {
	Error(c, e.Status, e.Code, e.Message)
}

// WriteWithStatus 将模板输出为统一错误响应，并保留模板 HTTP 状态码。
func (e ErrorTemplate) WriteWithStatus(c *gin.Context) {
	ErrorWithStatus(c, e.Status, e.Code, e.Message)
}
