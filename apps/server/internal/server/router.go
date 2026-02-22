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
		authRegistrationPolicyService := service.NewAuthRegistrationPolicyService(systemConfigRepo)
		authHandler := handler.NewAuthHandler(authService, authRegistrationPolicyService)
		imageHostingService := service.NewImageHostingService(systemConfigRepo)
		imageHostingHandler := handler.NewImageHostingHandler(authService, imageHostingService)
		api.POST("/auth/register", authHandler.Register)
		api.POST("/auth/login", authHandler.Login)
		api.POST("/auth/refresh", authHandler.Refresh)
		api.GET("/auth/me", authHandler.Me)
		api.POST("/auth/logout", authHandler.Logout)
		api.GET("/image-hosting", imageHostingHandler.GetConfig)
		api.POST("/uploads/images", imageHostingHandler.UploadImage)
		api.GET("/uploads/local/*path", imageHostingHandler.ServeLocalImage)

		visibilityService := service.NewVisibilityService(spaceRepo, documentRepo)
		accessHandler := handler.NewAccessHandler(authService, visibilityService)
		api.GET("/spaces/:spaceId", accessHandler.GetSpace)
		api.PUT("/spaces/:spaceId/visibility", accessHandler.UpdateSpaceVisibility)
		api.GET("/docs/:docId", accessHandler.GetDocument)
		api.PUT("/docs/:docId/visibility", accessHandler.UpdateDocumentVisibility)

		adminAccessService := service.NewAdminAccessService(adminRoleRepo, spaceAdminScopeRepo, spaceRepo)
		adminHandler := handler.NewAdminHandler(adminAccessService)
		adminOperationTokenService := service.NewAdminOperationTokenService(adminAccessService, 0)
		adminOperationTokenHandler := handler.NewAdminOperationTokenHandler(adminOperationTokenService)
		adminAuditService := service.NewAdminAuditService(auditLogRepo, adminAccessService)
		adminAuditHandler := handler.NewAdminAuditHandler(adminAuditService)
		adminUserService := service.NewAdminUserService(userRepo, userSessionRepo, adminRoleRepo, adminAccessService, adminAuditService)
		adminUserHandler := handler.NewAdminUserHandler(adminUserService)
		adminSpaceService := service.NewAdminSpaceService(spaceRepo, userRepo, adminRoleRepo, spaceAdminScopeRepo, adminAccessService, adminAuditService)
		adminSpaceHandler := handler.NewAdminSpaceHandler(adminSpaceService)
		adminDocumentService := service.NewAdminDocumentService(documentRepo, userRepo, adminAccessService, adminAuditService)
		adminDocumentHandler := handler.NewAdminDocumentHandler(adminDocumentService)
		adminThemeService := service.NewAdminThemeService(themeRepo, adminAccessService, adminAuditService)
		adminThemeHandler := handler.NewAdminThemeHandler(adminThemeService)
		adminSystemConfigService := service.NewAdminSystemConfigService(systemConfigRepo, adminAccessService, adminAuditService)
		adminSystemConfigHandler := handler.NewAdminSystemConfigHandler(adminSystemConfigService)
		adminAPI := api.Group("/admin")
		adminAPI.Use(middleware.RequireAdmin(authService, adminAccessService))
		adminAPI.Use(middleware.AttachAdminAuditContext())
		{
			adminAPI.GET("/me", adminHandler.Me)
			adminAPI.POST("/operation-tokens", adminOperationTokenHandler.Issue)
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
			adminAPI.POST(
				"/users",
				middleware.RequirePlatformAdmin(adminAccessService),
				adminUserHandler.CreateUser,
			)
			adminAPI.PATCH(
				"/users/:userId/role",
				middleware.RequirePlatformAdmin(adminAccessService),
				middleware.RequireAdminOperationToken(
					adminOperationTokenService,
					middleware.AdminOperationTokenBinding{
						Operation:     "user.update_role",
						TargetType:    "user",
						TargetIDParam: "userId",
					},
				),
				adminUserHandler.UpdateRole,
			)
			adminAPI.PATCH(
				"/users/:userId/status",
				middleware.RequirePlatformAdmin(adminAccessService),
				middleware.RequireAdminOperationToken(
					adminOperationTokenService,
					middleware.AdminOperationTokenBinding{
						Operation:     "user.update_status",
						TargetType:    "user",
						TargetIDParam: "userId",
					},
				),
				adminUserHandler.UpdateStatus,
			)
			adminAPI.DELETE(
				"/users/:userId",
				middleware.RequirePlatformAdmin(adminAccessService),
				middleware.RequireAdminOperationToken(
					adminOperationTokenService,
					middleware.AdminOperationTokenBinding{
						Operation:     "user.delete",
						TargetType:    "user",
						TargetIDParam: "userId",
					},
				),
				adminUserHandler.DeleteUser,
			)
			adminAPI.POST("/spaces", adminSpaceHandler.CreateSpace)
			adminAPI.POST("/spaces/cover-assets", adminSpaceHandler.CreateCoverAsset)
			adminAPI.GET("/spaces", adminSpaceHandler.ListSpaces)
			adminAPI.GET(
				"/spaces/:spaceId/members",
				middleware.RequireSpaceManagement(adminAccessService, "spaceId"),
				adminSpaceHandler.ListMembers,
			)
			adminAPI.POST(
				"/spaces/:spaceId/members",
				middleware.RequireSpaceManagement(adminAccessService, "spaceId"),
				adminSpaceHandler.UpsertMember,
			)
			adminAPI.PATCH(
				"/spaces/:spaceId/members/:userId",
				middleware.RequireSpaceManagement(adminAccessService, "spaceId"),
				adminSpaceHandler.UpdateMemberRole,
			)
			adminAPI.DELETE(
				"/spaces/:spaceId/members/:userId",
				middleware.RequireSpaceManagement(adminAccessService, "spaceId"),
				adminSpaceHandler.DeleteMember,
			)
			adminAPI.PATCH(
				"/spaces/:spaceId/status",
				middleware.RequireSpaceManagement(adminAccessService, "spaceId"),
				middleware.RequireAdminOperationToken(
					adminOperationTokenService,
					middleware.AdminOperationTokenBinding{
						Operation:     "space.update_status",
						TargetType:    "space",
						TargetIDParam: "spaceId",
					},
				),
				adminSpaceHandler.UpdateStatus,
			)
			adminAPI.PATCH(
				"/spaces/:spaceId/metadata",
				middleware.RequireSpaceManagement(adminAccessService, "spaceId"),
				adminSpaceHandler.UpdateMetadata,
			)
			adminAPI.POST(
				"/spaces/:spaceId/transfer",
				middleware.RequireSpaceManagement(adminAccessService, "spaceId"),
				middleware.RequireAdminOperationToken(
					adminOperationTokenService,
					middleware.AdminOperationTokenBinding{
						Operation:     "space.transfer",
						TargetType:    "space",
						TargetIDParam: "spaceId",
					},
				),
				adminSpaceHandler.TransferOwnership,
			)
			adminAPI.DELETE(
				"/spaces/:spaceId",
				middleware.RequireSpaceManagement(adminAccessService, "spaceId"),
				middleware.RequireAdminOperationToken(
					adminOperationTokenService,
					middleware.AdminOperationTokenBinding{
						Operation:     "space.delete",
						TargetType:    "space",
						TargetIDParam: "spaceId",
					},
				),
				adminSpaceHandler.DeleteSpace,
			)
			adminAPI.GET("/documents", adminDocumentHandler.ListDocuments)
			adminAPI.PATCH(
				"/documents/:documentId/status",
				middleware.RequireAdminOperationToken(
					adminOperationTokenService,
					middleware.AdminOperationTokenBinding{
						Operation:     "document.update_status",
						TargetType:    "document",
						TargetIDParam: "documentId",
					},
				),
				adminDocumentHandler.UpdateStatus,
			)
			adminAPI.DELETE(
				"/documents/:documentId",
				middleware.RequireAdminOperationToken(
					adminOperationTokenService,
					middleware.AdminOperationTokenBinding{
						Operation:     "document.delete",
						TargetType:    "document",
						TargetIDParam: "documentId",
					},
				),
				adminDocumentHandler.DeleteDocument,
			)
			adminAPI.GET("/themes", adminThemeHandler.ListThemes)
			adminAPI.POST("/themes", adminThemeHandler.CreateTheme)
			adminAPI.PUT(
				"/themes/:themeId",
				middleware.RequireAdminOperationToken(
					adminOperationTokenService,
					middleware.AdminOperationTokenBinding{
						Operation:     "theme.update",
						TargetType:    "theme",
						TargetIDParam: "themeId",
					},
				),
				adminThemeHandler.UpdateTheme,
			)
			adminAPI.DELETE(
				"/themes/:themeId",
				middleware.RequireAdminOperationToken(
					adminOperationTokenService,
					middleware.AdminOperationTokenBinding{
						Operation:     "theme.delete",
						TargetType:    "theme",
						TargetIDParam: "themeId",
					},
				),
				adminThemeHandler.DeleteTheme,
			)
			adminAPI.GET(
				"/system-configs",
				middleware.RequirePlatformAdmin(adminAccessService),
				adminSystemConfigHandler.ListConfigs,
			)
			adminAPI.PUT(
				"/system-configs/:key",
				middleware.RequirePlatformAdmin(adminAccessService),
				middleware.RequireAdminOperationToken(
					adminOperationTokenService,
					middleware.AdminOperationTokenBinding{
						Operation:     "system_config.upsert",
						TargetType:    "system_config",
						TargetIDParam: "key",
					},
				),
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
