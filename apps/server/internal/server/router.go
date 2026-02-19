package server

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/config"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/handler"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/middleware"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
)

// NewRouter 按统一中间件链装配路由，作为服务入口。
func NewRouter(cfg config.Config, logger *slog.Logger) *gin.Engine {
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.HandleMethodNotAllowed = true
	router.Use(middleware.RequestAttrsContext())
	router.Use(middleware.RequestID())
	router.Use(middleware.AccessLog(logger))
	router.Use(middleware.Recovery())
	router.Use(middleware.Timeout(cfg.RequestTimeout))
	router.Use(middleware.CORS(cfg.WebOrigin))

	api := router.Group("/api")
	{
		api.GET("/healthz", handler.Health)
	}

	router.NoRoute(func(c *gin.Context) {
		response.Error(c, http.StatusNotFound, "ROUTE_NOT_FOUND", "route not found")
	})
	router.NoMethod(func(c *gin.Context) {
		response.Error(c, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	})

	return router
}
