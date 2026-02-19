package handler

import "github.com/gin-gonic/gin"

// Health 提供基础健康检查，便于本地联调和探针检测。
func Health(c *gin.Context) {
	c.JSON(200, gin.H{
		"ok": true,
	})
}
