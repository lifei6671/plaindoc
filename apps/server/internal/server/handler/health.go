package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/buildinfo"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
)

// Health 提供基础健康检查，便于本地联调和探针检测。
func Health(c *gin.Context) {
	response.JSON(c, 200, gin.H{
		"ok":    true,
		"build": buildinfo.Current(),
	})
}
