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
		userRepo := repository.NewGormUserRepository(db)
		userSessionRepo := repository.NewGormUserSessionRepository(db)
		spaceRepo := repository.NewGormSpaceRepository(db)
		documentRepo := repository.NewGormDocumentRepository(db)
		themeRepo := repository.NewGormThemeRepository(db)
		systemConfigRepo := repository.NewGormSystemConfigRepository(db)
		auditLogRepo := repository.NewGormAuditLogRepository(db)
		adminRoleRepo := repository.NewGormAdminRoleRepository(db)
		spaceAdminScopeRepo := repository.NewGormSpaceAdminScopeRepository(db)

		authService := service.NewAuthService(userRepo, userSessionRepo, cfg.JWT)
		authHandler := handler.NewAuthHandler(authService)
		api.POST("/auth/register", authHandler.Register)
		api.POST("/auth/login", authHandler.Login)
		api.POST("/auth/refresh", authHandler.Refresh)
		api.GET("/auth/me", authHandler.Me)
		api.POST("/auth/logout", authHandler.Logout)

		visibilityService := service.NewVisibilityService(spaceRepo, documentRepo)
		accessHandler := handler.NewAccessHandler(authService, visibilityService)
		api.GET("/spaces/:spaceId", accessHandler.GetSpace)
		api.PUT("/spaces/:spaceId/visibility", accessHandler.UpdateSpaceVisibility)
		api.GET("/docs/:docId", accessHandler.GetDocument)
		api.PUT("/docs/:docId/visibility", accessHandler.UpdateDocumentVisibility)

		adminAccessService := service.NewAdminAccessService(adminRoleRepo, spaceAdminScopeRepo)
		adminHandler := handler.NewAdminHandler(adminAccessService)
		adminAuditService := service.NewAdminAuditService(auditLogRepo, adminAccessService)
		adminAuditHandler := handler.NewAdminAuditHandler(adminAuditService)
		adminUserService := service.NewAdminUserService(userRepo, userSessionRepo, adminAccessService, adminAuditService)
		adminUserHandler := handler.NewAdminUserHandler(adminUserService)
		adminSpaceService := service.NewAdminSpaceService(spaceRepo, userRepo, adminAccessService, adminAuditService)
		adminSpaceHandler := handler.NewAdminSpaceHandler(adminSpaceService)
		adminDocumentService := service.NewAdminDocumentService(documentRepo, userRepo, adminAccessService, adminAuditService)
		adminDocumentHandler := handler.NewAdminDocumentHandler(adminDocumentService)
		adminThemeService := service.NewAdminThemeService(themeRepo, adminAccessService, adminAuditService)
		adminThemeHandler := handler.NewAdminThemeHandler(adminThemeService)
		adminSystemConfigService := service.NewAdminSystemConfigService(systemConfigRepo, adminAccessService, adminAuditService)
		adminSystemConfigHandler := handler.NewAdminSystemConfigHandler(adminSystemConfigService)
		adminAPI := api.Group("/admin")
		adminAPI.Use(middleware.RequireAdmin(authService, adminAccessService))
		{
			adminAPI.GET("/me", adminHandler.Me)
			adminAPI.GET(
				"/spaces/:spaceId/check",
				middleware.RequireSpaceManagement(adminAccessService, "spaceId"),
				adminHandler.CheckSpace,
			)
			adminAPI.GET(
				"/users",
				middleware.RequirePlatformAdmin(adminAccessService),
				adminUserHandler.ListUsers,
			)
			adminAPI.PATCH(
				"/users/:userId/status",
				middleware.RequirePlatformAdmin(adminAccessService),
				adminUserHandler.UpdateStatus,
			)
			adminAPI.DELETE(
				"/users/:userId",
				middleware.RequirePlatformAdmin(adminAccessService),
				adminUserHandler.DeleteUser,
			)
			adminAPI.GET("/spaces", adminSpaceHandler.ListSpaces)
			adminAPI.PATCH(
				"/spaces/:spaceId/status",
				middleware.RequireSpaceManagement(adminAccessService, "spaceId"),
				adminSpaceHandler.UpdateStatus,
			)
			adminAPI.PATCH(
				"/spaces/:spaceId/metadata",
				middleware.RequireSpaceManagement(adminAccessService, "spaceId"),
				adminSpaceHandler.UpdateMetadata,
			)
			adminAPI.DELETE(
				"/spaces/:spaceId",
				middleware.RequireSpaceManagement(adminAccessService, "spaceId"),
				adminSpaceHandler.DeleteSpace,
			)
			adminAPI.GET("/documents", adminDocumentHandler.ListDocuments)
			adminAPI.PATCH(
				"/documents/:documentId/status",
				adminDocumentHandler.UpdateStatus,
			)
			adminAPI.DELETE(
				"/documents/:documentId",
				adminDocumentHandler.DeleteDocument,
			)
			adminAPI.GET("/themes", adminThemeHandler.ListThemes)
			adminAPI.POST("/themes", adminThemeHandler.CreateTheme)
			adminAPI.PUT("/themes/:themeId", adminThemeHandler.UpdateTheme)
			adminAPI.DELETE("/themes/:themeId", adminThemeHandler.DeleteTheme)
			adminAPI.GET(
				"/system-configs",
				middleware.RequirePlatformAdmin(adminAccessService),
				adminSystemConfigHandler.ListConfigs,
			)
			adminAPI.PUT(
				"/system-configs/:key",
				middleware.RequirePlatformAdmin(adminAccessService),
				adminSystemConfigHandler.UpsertConfig,
			)
			adminAPI.GET("/audits", adminAuditHandler.ListAudits)
		}

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
