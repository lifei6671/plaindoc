package server

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/config"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/handler"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/middleware"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

// NewRouter 按统一中间件链装配路由，作为服务入口。
func NewRouter(cfg config.Config, logger *slog.Logger, db *gorm.DB) *gin.Engine {
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
		authService := service.NewAuthService(
			repository.NewGormUserRepository(db),
			repository.NewGormUserSessionRepository(db),
			cfg.JWT,
		)
		authHandler := handler.NewAuthHandler(authService)
		api.POST("/auth/register", authHandler.Register)
		api.POST("/auth/login", authHandler.Login)
		api.POST("/auth/refresh", authHandler.Refresh)
		api.GET("/auth/me", authHandler.Me)
		api.POST("/auth/logout", authHandler.Logout)

		themeHandler := handler.NewThemeHandler(db)
		documentThemeHandler := handler.NewDocumentThemeHandler(db)
		api.GET("/themes", themeHandler.List)
		api.PUT("/docs/:docId/theme", documentThemeHandler.UpdateTheme)
	}

	router.NoRoute(func(c *gin.Context) {
		response.Error(c, http.StatusNotFound, "ROUTE_NOT_FOUND", "route not found")
	})
	router.NoMethod(func(c *gin.Context) {
		response.Error(c, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	})

	return router
}
