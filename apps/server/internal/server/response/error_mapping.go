package response

import (
	"errors"

	"github.com/gin-gonic/gin"
)

// ErrorTemplateMapping 定义 sentinel 错误到错误模板的映射规则。
type ErrorTemplateMapping struct {
	Target   error
	Template ErrorTemplate
}

// WriteMappedError 按映射规则输出错误响应；命中返回 true，未命中返回 false。
func WriteMappedError(c *gin.Context, err error, mappings ...ErrorTemplateMapping) bool {
	if c == nil || err == nil {
		return false
	}

	for _, mapping := range mappings {
		if mapping.Target == nil {
			continue
		}
		if errors.Is(err, mapping.Target) {
			mapping.Template.Write(c)
			return true
		}
	}
	return false
}
