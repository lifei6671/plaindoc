package server

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/lifei6671/plaindoc/apps/server/internal/config"
	"github.com/lifei6671/plaindoc/apps/server/internal/logit"
	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/captchastore"
	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/rendercache"
	searchprovider "github.com/lifei6671/plaindoc/apps/server/internal/search/provider"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/handler"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/middleware"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/view"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
	ssrpool "github.com/lifei6671/plaindoc/apps/server/internal/ssr/pool"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
)

const (
	defaultReaderRenderCacheMaxBytes      = 64 << 20 // 64MB
	defaultReaderRenderCacheMaxEntryBytes = 2 << 20  // 2MB
	defaultReaderRenderCacheTTL           = 15 * time.Minute
)

// NewRouter 构建并返回服务唯一的 HTTP 路由入口。
//
// 这里集中完成：
// 1) 全局中间件链装配（请求上下文、日志、恢复、超时、CORS）；
// 2) Repository -> Service -> Handler 的依赖注入；
// 3) SSR 页面路由、业务 API 路由、后台管理路由注册；
// 4) 统一 404/405 错误响应兜底。
//
// 这样做的目的是把“请求处理入口”和“依赖关系拓扑”固定在一个文件，避免路由分散导致权限边界不清、
// 审计链路遗漏或中间件顺序回归。
func NewRouter(cfg config.Config, logger *slog.Logger, db *gorm.DB) *gin.Engine {
	return newRouter(cfg, logger, db, nil)
}

// NewRouterWithSSR 构建并返回启用 SSR 分发器的 HTTP 路由入口。
func NewRouterWithSSR(
	cfg config.Config,
	logger *slog.Logger,
	db *gorm.DB,
	readerSSRDispatcher *ssrpool.Dispatcher,
) *gin.Engine {
	return newRouter(cfg, logger, db, readerSSRDispatcher)
}

func newRouter(
	cfg config.Config,
	logger *slog.Logger,
	db *gorm.DB,
	readerSSRDispatcher *ssrpool.Dispatcher,
) *gin.Engine {
	// 生产环境切到 ReleaseMode，避免 debug 输出与额外开销。
	// 非生产环境保持默认模式，方便本地联调与故障排查。
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 使用 gin.New() 而不是 gin.Default()，确保中间件链完全由项目显式控制，
	// 防止默认 Logger/Recovery 与自定义中间件发生重复或顺序冲突。
	router := gin.New()

	// 1) 只信任来自 Caddy 的代理（Docker 网段或 Caddy 容器 IP）
	// 示例：你的 docker network subnet 是 172.18.0.0/16，就写它
	proxies := strings.Split(os.Getenv("GIN_TRUSTED_PROXIES"), ",")
	for i := range proxies {
		proxies[i] = strings.TrimSpace(proxies[i])
	}
	if len(proxies) == 1 && proxies[0] == "" {
		proxies = nil
	}
	_ = router.SetTrustedProxies(proxies)

	// 2) Cloudflare 场景：让 Gin 直接信任 CF-Connecting-IP（推荐）
	router.TrustedPlatform = gin.PlatformCloudflare

	// 开启 405（Method Not Allowed）处理，让同一路径错误方法命中 NoMethod。
	router.HandleMethodNotAllowed = true
	// 1) 预置请求属性容器：用于后续中间件/业务层向 context 写入结构化日志字段。
	router.Use(middleware.RequestAttrsContext())
	// 2) 注入 request_id：贯穿日志、审计、错误追踪，便于跨服务定位问题。
	router.Use(middleware.RequestID())
	// 3) 访问日志：记录请求生命周期基础指标（状态码、耗时、路径等）。
	router.Use(middleware.AccessLog(logger))
	// 4) panic 恢复：保证单请求异常不会拖垮整个进程。
	router.Use(middleware.Recovery())
	// 5) 全局超时：避免慢请求长期占用资源，防止雪崩。
	router.Use(middleware.Timeout(cfg.RequestTimeout))
	// 6) CORS：统一跨域来源控制，配合前端域名配置使用。
	router.Use(middleware.CORS(cfg.WebOrigin))

	// ---- Repository 装配层 ----
	// 所有仓储统一由 GORM 实现并在此注入，便于后续替换实现时维持依赖清晰。
	userRepo := repository.NewGormUserRepository(db)
	userIdentityRepo := repository.NewGormUserIdentityRepository(db)
	userSessionRepo := repository.NewGormUserSessionRepository(db)
	passwordResetTokenRepo := repository.NewGormPasswordResetTokenRepository(db)
	authRiskStateRepo := repository.NewGormAuthRiskStateRepository(db)
	authCaptchaChallengeRepo := repository.NewGormAuthCaptchaChallengeRepository(db)
	searchIndexJobRepo := repository.NewGormSearchIndexJobRepository(db)
	searchVisibilityRepo := repository.NewGormSearchVisibilityRepository(db)
	spaceRepo := repository.NewGormSpaceRepository(db, searchIndexJobRepo)
	spaceCategoryRepo := repository.NewGormSpaceCategoryRepository(db)
	documentRepo := repository.NewGormDocumentRepository(db, searchIndexJobRepo)
	documentShareRepo := repository.NewGormDocumentShareRepository(db)
	documentAttachmentRepo := repository.NewGormDocumentAttachmentRepository(db)
	documentImageAssetRepo := repository.NewGormDocumentImageAssetRepository(db)
	documentTemplateRepo := repository.NewGormDocumentTemplateRepository(db)
	documentTemplateSceneRepo := repository.NewGormDocumentTemplateSceneRepository(db)
	searchAnalyzerDictEntryRepo := repository.NewGormSearchAnalyzerDictEntryRepository(db)
	themeRepo := repository.NewGormThemeRepository(db)
	systemConfigRepo := repository.NewGormSystemConfigRepository(db)
	auditLogRepo := repository.NewGormAuditLogRepository(db)
	adminRoleRepo := repository.NewGormAdminRoleRepository(db)
	spaceAdminScopeRepo := repository.NewGormSpaceAdminScopeRepository(db)
	workspaceRepo := repository.NewGormWorkspaceRepository(db, searchIndexJobRepo)
	// adminAccessService 统一封装 platform_admin / space_admin(scope) 权限判定。
	adminAccessService := service.NewAdminAccessService(adminRoleRepo, spaceAdminScopeRepo, spaceRepo)

	// ---- 基础服务与页面 Handler 装配 ----
	// authService 是鉴权基石，后续 API/后台中间件都会复用。
	authService := service.NewAuthService(userRepo, userSessionRepo, cfg.JWT)
	emailConfigService := service.NewEmailConfigService(systemConfigRepo)
	passwordResetService := service.NewPasswordResetService(
		userRepo,
		userSessionRepo,
		passwordResetTokenRepo,
		emailConfigService,
		service.NewSMTPMailSender(),
		cfg.JWT.Secret,
	)
	// 首页 SSR 服务：负责首页/分类页可见性过滤、分类导航和缓存策略配置读取。
	homeService := service.NewHomeService(spaceRepo, spaceCategoryRepo, systemConfigRepo)
	// sitemap 服务：负责公开空间/文档 URL 的收敛输出。
	sitemapService := service.NewSitemapService(db, systemConfigRepo)
	// 搜索词典服务：负责从 DB 加载 jieba 用户词条。
	searchAnalyzerDictService := service.NewSearchAnalyzerDictService(searchAnalyzerDictEntryRepo)
	// 搜索配置服务：负责 search 配置解析与 active analyzer 选择。
	searchConfigService := service.NewSearchConfigService(systemConfigRepo, service.SearchConfigServiceOptions{
		JiebaDictLoader: searchAnalyzerDictService.BuildJiebaDictLoader(),
	})
	if _, err := searchConfigService.Resolve(context.Background()); err != nil && logger != nil {
		logger.Error("search config initialization failed", slog.String("error", err.Error()))
	}
	bleveProvider := searchprovider.NewBleveProvider(searchprovider.BleveProviderOptions{
		DB:             db,
		VisibilityRepo: searchVisibilityRepo,
		IndexPath:      resolveBleveIndexPath(cfg),
	})
	databaseProvider := searchprovider.NewDatabaseProvider(db)
	searchQueryService := service.NewSearchQueryService(
		searchConfigService,
		databaseProvider,
		bleveProvider,
	)
	searchQueryService.SetSearchVisibilityRepository(searchVisibilityRepo)
	searchIndexService := service.NewSearchIndexService(
		db,
		searchConfigService,
		databaseProvider,
		bleveProvider,
	)
	searchIndexService.SetSearchIndexJobRepository(searchIndexJobRepo)
	searchIndexJobService := service.NewSearchIndexJobService(searchIndexJobRepo, searchIndexService)
	// 启动阶段异步初始化索引，避免大数据量重建阻塞服务启动。
	go func() {
		if _, err := searchIndexService.Bootstrap(context.Background()); err != nil && logger != nil {
			logger.Error("search index bootstrap failed", slog.String("error", err.Error()))
		}
	}()
	go runSearchIndexJobLoop(context.Background(), logger, searchIndexJobService)

	// 首页全文检索服务：负责首页落地页检索结果读取与可见性过滤。
	homeSearchService := service.NewHomeSearchService(searchQueryService, db)
	// 可见性服务为“空间/文档可访问性”提供统一判定，避免 handler 里散落权限逻辑。
	visibilityService := service.NewVisibilityService(spaceRepo, documentRepo)
	// 阅读页服务：聚合空间树与文档正文，供 SSR worker 渲染。
	readerPageService := service.NewReaderPageService(db, visibilityService, documentAttachmentRepo)
	// 阅读页 SSR 渲染缓存：仅缓存 Node 渲染结果，不缓存鉴权流程。
	readerRenderCache := buildReaderRenderCache(cfg, logger, readerSSRDispatcher != nil)
	// 首页 Handler：承接 SSR 渲染与登录态相关行为。
	homeHandler := handler.NewHomeHandler(
		authService,
		homeService,
		homeSearchService,
		searchConfigService,
		adminAccessService,
		cfg.WebOrigin,
	)
	// sitemap Handler：承接 /sitemap.xml 输出。
	sitemapHandler := handler.NewSitemapHandler(sitemapService, cfg.WebOrigin)
	// 图床配置与上传服务：既服务 API 上传入口，也服务公开图片回源路径。
	imageHostingService := service.NewImageHostingService(systemConfigRepo)
	onlyOfficeConfigService := service.NewOnlyOfficeConfigService(systemConfigRepo)
	onlyOfficeDocumentTokenService := service.NewOnlyOfficeDocumentTokenService(cfg.JWT.Secret, 0)
	// 阅读页 Handler：承接 /r/:spaceId/:docId 渲染链路（可降级）。
	readerPageHandler := handler.NewReaderPageHandler(
		authService,
		readerPageService,
		readerSSRDispatcher,
		readerRenderCache,
		logger,
		cfg.WebOrigin,
		onlyOfficeConfigService,
		onlyOfficeDocumentTokenService,
	)
	documentShareService := service.NewDocumentShareService(
		db,
		documentShareRepo,
		documentAttachmentRepo,
		spaceRepo,
		imageHostingService,
	)
	documentShareAccessTokenService := service.NewDocumentShareAccessTokenService(cfg.JWT.Secret, 0)
	adminDocumentShareService := service.NewAdminDocumentShareService(
		documentShareRepo,
		documentShareService,
		adminAccessService,
	)
	documentAttachmentCleanupService := service.NewDocumentAttachmentCleanupService(
		db,
		documentAttachmentRepo,
		imageHostingService,
	)
	documentImageAssetService := service.NewDocumentImageAssetService(db, imageHostingService)
	documentTemplateService := service.NewDocumentTemplateService(documentTemplateRepo)
	imageHostingHandler := handler.NewImageHostingHandler(authService, imageHostingService, spaceRepo)
	onlyOfficeHandler := handler.NewOnlyOfficeHandler(authService, onlyOfficeConfigService)
	documentAttachmentTokenService := service.NewDocumentAttachmentDownloadTokenService(cfg.JWT.Secret, 24*time.Hour)
	accessHandler := handler.NewAccessHandler(authService, visibilityService, readerRenderCache, searchIndexService)
	documentTemplateHandler := handler.NewDocumentTemplateHandler(documentTemplateService)
	workspaceHandler := handler.NewWorkspaceHandler(
		workspaceRepo,
		documentAttachmentRepo,
		documentAttachmentCleanupService,
		documentImageAssetService,
		documentTemplateService,
		authService,
		visibilityService,
		imageHostingService,
		onlyOfficeConfigService,
		onlyOfficeDocumentTokenService,
		documentAttachmentTokenService,
		readerRenderCache,
		searchIndexService,
	)
	workspaceDocumentShareHandler := handler.NewWorkspaceDocumentShareHandler(workspaceHandler, documentShareService)
	documentSharePageHandler := handler.NewDocumentSharePageHandler(
		documentShareService,
		documentShareAccessTokenService,
		readerPageHandler,
	)

	// ---- SSR 页面与静态资源路由（不走 /api）----
	// 模板静态资源（CSS/字体/图片）统一挂在 /assets。
	router.StaticFS("/assets", http.FS(view.MustStaticFS()))
	// 站点 favicon 走根路径直出，避免 302 重定向带来的额外 RTT。
	router.GET("/favicon.ico", func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=2592000, immutable")
		c.FileFromFS("favicon.ico", http.FS(view.MustStaticFS()))
	})
	// robots.txt
	router.GET("/robots.txt", func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=3600, immutable")
		c.FileFromFS("robots.txt", http.FS(view.MustStaticFS()))
	})
	// sitemap.xml
	router.GET("/sitemap.xml", sitemapHandler.Sitemap)
	// 首页：服务端渲染（SEO 主入口）。
	router.GET("/", homeHandler.Home)
	// 分类探索页：服务端渲染分类导航与分类空间列表。
	router.GET("/explore/:categoryId", homeHandler.Explore)
	// 首页全文检索落地页：显示可见文档检索结果。
	router.GET("/search", homeHandler.Search)
	// 文档单页分享：支持公开/密码分享访问。
	router.GET("/s/:spaceId/:docKey", documentSharePageHandler.Page)
	// 分享密码校验接口：验证成功后写入免密 Cookie。
	router.POST("/s/:spaceId/:docKey/verify", documentSharePageHandler.Verify)
	// 空间阅读入口：自动跳转到首篇可读文档。
	router.GET("/r/:spaceId", readerPageHandler.Space)
	// 文档阅读页：优先走 SSR worker 渲染，失败时降级 HTML 壳页。
	router.GET("/r/:spaceId/:docId", readerPageHandler.Page)
	// 文档附件预览页：页面内部按 purpose=preview 动态获取可访问链接。
	router.GET("/preview/docs/:docId/attachments/:attachmentId", workspaceHandler.DocumentAttachmentPreviewPage)
	// 分享态附件预览页：页面内部按分享 access-link 获取可访问链接。
	router.GET("/preview/shares/:spaceId/:docKey/attachments/:attachmentId", documentSharePageHandler.AttachmentPreviewPage)
	// 页面层登出入口：与 SSR 页面交互保持一致。
	router.POST("/logout", homeHandler.Logout)
	// 本地图片公开访问路径（不带 /api），便于前端 Markdown 直接渲染。
	router.GET("/uploads/*path", imageHostingHandler.ServeLocalImage)
	// Web SPA（登录/编辑/后台）生产托管路由：仅在 dist 产物存在时启用。
	registerWebSPARoutes(router, cfg, logger)

	// ---- 业务 API 根组 ----
	api := router.Group("/api")
	{
		// 健康检查：用于容器探活与基础可用性监控。
		api.GET("/healthz", handler.Health)

		// ---- 认证与会话 API ----
		// 注册策略独立成服务，便于从系统配置动态控制开放注册。
		authRegistrationPolicyService := service.NewAuthRegistrationPolicyService(systemConfigRepo)
		authRiskPolicyService := service.NewAuthRiskPolicyService(systemConfigRepo)
		var authCaptchaStore captchastore.Store
		authCaptchaDatabaseBackend, captchaBackendErr := captchastore.NewDatabaseBackend(
			repository.NewGormCaptchaStoreRepository(db),
		)
		if captchaBackendErr != nil {
			if logger != nil {
				logger.Error("auth captcha database backend initialization failed", slog.String("error", captchaBackendErr.Error()))
			}
		} else {
			initializedStore, storeErr := captchastore.New(authCaptchaDatabaseBackend)
			if storeErr != nil {
				if logger != nil {
					logger.Error("auth captcha store initialization failed", slog.String("error", storeErr.Error()))
				}
			} else {
				authCaptchaStore = initializedStore
			}
		}
		authRiskControlService := service.NewAuthRiskControlService(
			authRiskPolicyService,
			authRiskStateRepo,
			authCaptchaChallengeRepo,
			cfg.JWT.Secret,
			authCaptchaStore,
		)
		authProviders := []service.AuthLoginProvider{
			service.NewLocalAuthLoginProvider(authService),
		}
		if cfg.Auth.LDAP.Enabled {
			ldapProvider, err := service.NewLDAPAuthLoginProvider(
				service.LDAPAuthProviderConfig{
					ProviderID:         cfg.Auth.LDAP.ProviderID,
					Host:               cfg.Auth.LDAP.Host,
					Port:               cfg.Auth.LDAP.Port,
					TLSMode:            service.LDAPTLSMode(cfg.Auth.LDAP.TLSMode),
					InsecureSkipVerify: cfg.Auth.LDAP.InsecureSkipVerify,
					BaseDN:             cfg.Auth.LDAP.BaseDN,
					BindDN:             cfg.Auth.LDAP.BindDN,
					BindPassword:       cfg.Auth.LDAP.BindPassword,
					UserFilter:         cfg.Auth.LDAP.UserFilter,
					IDAttribute:        cfg.Auth.LDAP.IDAttribute,
					EmailAttribute:     cfg.Auth.LDAP.EmailAttribute,
					NameAttribute:      cfg.Auth.LDAP.NameAttribute,
					ConnectTimeout:     cfg.Auth.LDAP.ConnectTimeout,
					ReadTimeout:        cfg.Auth.LDAP.ReadTimeout,
					HealthCheckBaseDN:  cfg.Auth.LDAP.BaseDN,
				},
				authService,
				userRepo,
				userIdentityRepo,
			)
			if err != nil {
				if logger != nil {
					logger.Error("ldap auth provider initialization failed", slog.String("error", err.Error()))
				}
			} else {
				authProviders = append(authProviders, ldapProvider)
			}
		}
		authLoginOrchestrator := service.NewAuthLoginOrchestrator(cfg.Auth.DefaultProvider, authProviders...)
		authHandler := handler.NewAuthHandler(
			authService,
			authRegistrationPolicyService,
			authLoginOrchestrator,
			authRiskControlService,
			passwordResetService,
			cfg.JWT,
		)

		// 登录策略选项（登录页模式、provider 列表与注册开关）。
		api.GET("/auth/options", authHandler.Options)
		// 用户注册。
		api.POST("/auth/register", authHandler.Register)
		// 用户登录，签发 access/refresh token。
		api.POST("/auth/login", authHandler.Login)
		// 刷新验证码图片（登录/注册风控验证）。
		api.POST("/auth/captcha/refresh", authHandler.RefreshCaptcha)
		// refresh token 续签并执行轮换策略。
		api.POST("/auth/refresh", authHandler.Refresh)
		// 查询当前会话用户信息。
		api.GET("/auth/me", authHandler.Me)
		// 主动退出当前会话（含服务端会话吊销语义）。
		api.POST("/auth/logout", authHandler.Logout)
		// 发送密码重置邮件（防枚举）。
		api.POST("/auth/password-reset/request", authHandler.RequestPasswordReset)
		// 校验密码重置令牌。
		api.POST("/auth/password-reset/verify", authHandler.VerifyPasswordResetToken)
		// 提交新密码完成重置。
		api.POST("/auth/password-reset/confirm", authHandler.ConfirmPasswordReset)

		// 读取图床配置（用于前端展示上传能力与限制）。
		api.GET("/image-hosting", imageHostingHandler.GetConfig)
		// 读取 ONLYOFFICE 运行时开关（用于前端决定是否展示 Office 创建入口）。
		api.GET("/onlyoffice", onlyOfficeHandler.GetConfig)
		// 由后端分配图片对象 key（所有存储 provider 统一由后端生成文件名）。
		api.POST("/uploads/images/object-key", imageHostingHandler.IssueImageObjectKey)
		// 统一图片上传入口（按配置决定走本地或外部托管）。
		api.POST("/uploads/images", imageHostingHandler.UploadImage)
		// 本地图片回源访问（当配置使用本地存储时生效）。
		api.GET("/uploads/*path", imageHostingHandler.ServeLocalImage)

		// ---- 工作区与文档协作 API（业务主链）----
		// 文档模板列表（编辑器模板选择器数据源，仅返回已启用模板）。
		api.GET("/document-templates", documentTemplateHandler.ListTemplates)
		// 文档模板详情（按 template_id 查询）。
		api.GET("/document-templates/:templateId", documentTemplateHandler.GetTemplate)
		// 空间列表（按访问者可见范围过滤）。
		api.GET("/spaces", workspaceHandler.ListSpaces)
		// 新建空间（业务端入口，不是后台治理入口）。
		api.POST("/spaces", workspaceHandler.CreateSpace)
		// 读取单个空间详情（含访问校验）。
		api.GET("/spaces/:spaceId", accessHandler.GetSpace)
		// 读取空间目录树（编辑器左侧目录树数据源）。
		api.GET("/spaces/:spaceId/tree", workspaceHandler.GetTree)
		// 在空间下创建目录/文档节点。
		api.POST("/spaces/:spaceId/nodes", workspaceHandler.CreateNode)
		// 更新节点（重命名、移动、排序等）。
		api.PATCH("/nodes/:nodeId", workspaceHandler.UpdateNode)
		// 移动节点（拖拽排序专用：同级重排 + 跨父级移动）。
		api.POST("/nodes/:nodeId/move", workspaceHandler.MoveNode)
		// 删除节点（目录递归删除与文档联动清理）。
		api.DELETE("/nodes/:nodeId", workspaceHandler.DeleteNode)
		// 更新空间可见性（public/authenticated/member）。
		api.PUT("/spaces/:spaceId/visibility", accessHandler.UpdateSpaceVisibility)
		// 读取文档正文。
		api.GET("/docs/:docId", accessHandler.GetDocument)
		// 阅读页 Office 文档只读配置。
		api.GET("/reader/spaces/:spaceId/docs/:docId/onlyoffice/view-config", readerPageHandler.GetOnlyOfficeViewConfig)
		// 生成 ONLYOFFICE 编辑配置。
		api.GET("/docs/:docId/onlyoffice/edit-config", workspaceHandler.GetOnlyOfficeEditConfig)
		// ONLYOFFICE Document Server 拉取正文文件。
		api.GET("/docs/:docId/onlyoffice/source", workspaceHandler.ServeOnlyOfficeSourceDocument)
		// ONLYOFFICE 保存回调。
		api.POST("/docs/:docId/onlyoffice/callback", workspaceHandler.HandleOnlyOfficeCallback)
		// 更新文档阅读标识（slug）。
		api.PATCH("/docs/:docId/identifier", workspaceHandler.UpdateDocumentIdentifier)
		// 获取文档分享配置。
		api.GET("/docs/:docId/share", workspaceDocumentShareHandler.GetDocumentShare)
		// 创建或更新文档分享配置。
		api.PUT("/docs/:docId/share", workspaceDocumentShareHandler.UpsertDocumentShare)
		// 取消文档分享。
		api.DELETE("/docs/:docId/share", workspaceDocumentShareHandler.DeleteDocumentShare)
		// 保存文档（包含版本冲突检测链路）。
		api.PUT("/docs/:docId", workspaceHandler.SaveDocument)
		// 外链图片转存（编辑器粘贴链路前端失败时的服务端兜底）。
		api.POST("/docs/:docId/remote-images/localize", workspaceHandler.LocalizeDocumentRemoteImages)
		// 获取文档历史修订列表。
		api.GET("/docs/:docId/revisions", workspaceHandler.ListRevisions)
		// 文档附件列表。
		api.GET("/docs/:docId/attachments", workspaceHandler.ListDocumentAttachments)
		// 上传文档附件（支持 local/cloudflare-r2/aliyun-oss provider）。
		api.POST("/docs/:docId/attachments", workspaceHandler.UploadDocumentAttachment)
		// 删除文档附件：
		// physicalDelete=false => 逻辑删除；physicalDelete=true => 删除文件并硬删除记录。
		api.DELETE("/docs/:docId/attachments/:attachmentId", workspaceHandler.DeleteDocumentAttachment)
		// 创建文档附件访问链接（下载/预览）。
		api.POST(
			"/docs/:docId/attachments/:attachmentId/access-link",
			workspaceHandler.CreateDocumentAttachmentAccessLink,
		)
		// 文档附件直接下载入口（适配无脚本场景，如导出 PDF 内的静态链接）。
		api.GET(
			"/docs/:docId/attachments/:attachmentId/download",
			workspaceHandler.RedirectDocumentAttachmentDownload,
		)
		// 更新文档可见性。
		api.PUT("/docs/:docId/visibility", accessHandler.UpdateDocumentVisibility)
		// 使用签名链接下载或预览文档附件。
		api.GET("/attachment-downloads/:token", workspaceHandler.ServeDocumentAttachmentByToken)
		// 分享态附件访问链接（下载/预览）。
		api.POST(
			"/shares/:spaceId/:docKey/attachments/:attachmentId/access-link",
			documentSharePageHandler.CreateAttachmentAccessLink,
		)
		// 分享页 Office 文档只读配置。
		api.GET(
			"/shares/:spaceId/:docKey/onlyoffice/view-config",
			documentSharePageHandler.GetOnlyOfficeViewConfig,
		)
		// 分享态附件下载入口（无脚本场景）。
		api.GET(
			"/shares/:spaceId/:docKey/attachments/:attachmentId/download",
			documentSharePageHandler.RedirectAttachmentDownload,
		)

		// ---- 后台治理 API 依赖装配 ----
		adminHandler := handler.NewAdminHandler(adminAccessService, userRepo)
		// 高风险操作一次性 token 服务；TTL=0 表示使用服务默认值。
		adminOperationTokenService := service.NewAdminOperationTokenService(adminAccessService, 0)
		adminOperationTokenHandler := handler.NewAdminOperationTokenHandler(adminOperationTokenService)
		// 审计服务：记录后台关键动作并支持检索。
		adminAuditService := service.NewAdminAuditService(auditLogRepo, adminAccessService)
		adminAuditHandler := handler.NewAdminAuditHandler(adminAuditService)
		adminProfileService := service.NewAdminProfileService(userRepo, adminAccessService, adminAuditService)
		adminProfileHandler := handler.NewAdminProfileHandler(adminProfileService, imageHostingService)
		adminUserService := service.NewAdminUserService(
			userRepo,
			userSessionRepo,
			adminRoleRepo,
			adminAccessService,
			adminAuditService,
			passwordResetService,
		)
		adminUserHandler := handler.NewAdminUserHandler(adminUserService)
		adminSpaceService := service.NewAdminSpaceService(
			spaceRepo,
			spaceCategoryRepo,
			userRepo,
			adminRoleRepo,
			spaceAdminScopeRepo,
			systemConfigRepo,
			adminAccessService,
			adminAuditService,
		)
		adminSpaceHandler := handler.NewAdminSpaceHandler(adminSpaceService, searchIndexService)
		adminDocumentService := service.NewAdminDocumentService(
			documentRepo,
			userRepo,
			workspaceRepo,
			adminAccessService,
			adminAuditService,
			documentAttachmentCleanupService,
		)
		adminDocumentHandler := handler.NewAdminDocumentHandler(adminDocumentService, searchIndexService)
		adminDocumentAttachmentService := service.NewAdminDocumentAttachmentService(
			documentAttachmentRepo,
			adminAccessService,
			adminAuditService,
			imageHostingService,
		)
		adminDocumentAttachmentHandler := handler.NewAdminDocumentAttachmentHandler(adminDocumentAttachmentService)
		adminDocumentImageAssetService := service.NewAdminDocumentImageAssetService(
			documentImageAssetRepo,
			adminAccessService,
			adminAuditService,
			imageHostingService,
		)
		adminDocumentImageAssetHandler := handler.NewAdminDocumentImageAssetHandler(adminDocumentImageAssetService)
		adminDocumentShareHandler := handler.NewAdminDocumentShareHandler(adminDocumentShareService)
		adminSearchAnalyzerService := service.NewAdminSearchAnalyzerService(
			searchAnalyzerDictEntryRepo,
			systemConfigRepo,
			adminAccessService,
			adminAuditService,
			searchConfigService,
		)
		adminSearchAnalyzerHandler := handler.NewAdminSearchAnalyzerHandler(adminSearchAnalyzerService)
		adminDocumentTemplateService := service.NewAdminDocumentTemplateService(
			documentTemplateRepo,
			documentTemplateSceneRepo,
			adminAccessService,
			adminAuditService,
		)
		adminDocumentTemplateHandler := handler.NewAdminDocumentTemplateHandler(adminDocumentTemplateService)
		adminDocumentTemplateSceneService := service.NewAdminDocumentTemplateSceneService(
			documentTemplateSceneRepo,
			adminAccessService,
			adminAuditService,
		)
		adminDocumentTemplateSceneHandler := handler.NewAdminDocumentTemplateSceneHandler(adminDocumentTemplateSceneService)
		adminThemeService := service.NewAdminThemeService(themeRepo, adminAccessService, adminAuditService)
		adminThemeHandler := handler.NewAdminThemeHandler(adminThemeService)
		dataRetentionCleanupService := service.NewDataRetentionCleanupService(db, systemConfigRepo)
		adminSystemConfigService := service.NewAdminSystemConfigService(
			systemConfigRepo,
			adminAccessService,
			adminAuditService,
			dataRetentionCleanupService,
			searchIndexService,
		)
		adminSystemConfigHandler := handler.NewAdminSystemConfigHandler(adminSystemConfigService)

		// 后台路由统一挂载在 /api/admin。
		adminAPI := api.Group("/admin")
		// 第一层守卫：要求已登录且具备管理员身份（platform_admin 或 space_admin）。
		adminAPI.Use(middleware.RequireAdmin(authService, adminAccessService))
		// 第二层守卫：向 context 注入审计公共字段（actor_user_id / request_id）。
		// 后续 service 记录审计时可自动从 context 取值，避免每个 handler 重复传参。
		adminAPI.Use(middleware.AttachAdminAuditContext())
		{
			// 当前管理员信息（用于前端初始化角色与菜单）。
			adminAPI.GET("/me", adminHandler.Me)
			// 当前管理员个人资料查询/更新。
			adminAPI.GET("/profile", adminProfileHandler.GetProfile)
			adminAPI.PATCH("/profile", adminProfileHandler.UpdateProfile)
			adminAPI.PUT("/profile/password", adminProfileHandler.UpdatePassword)
			adminAPI.POST("/profile/avatar", adminProfileHandler.UploadAvatar)
			// 签发高风险操作一次性 token（例如删除/封禁/配置变更前置校验）。
			adminAPI.POST("/operation-tokens", adminOperationTokenHandler.Issue)

			// space_admin 是否具备某空间治理权限检查；platform_admin 恒通过。
			adminAPI.GET(
				"/spaces/:spaceId/check",
				middleware.RequireSpaceManagement(adminAccessService, "spaceId"),
				adminHandler.CheckSpace,
			)

			// ---- 用户治理（仅平台管理员）----
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
			// 高风险写操作统一叠加 RequireAdminOperationToken：
			// token 绑定「操作者 + operation + targetType + targetID」，可防重放与跨目标复用。
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
			adminAPI.POST(
				"/users/:userId/password-reset-email",
				middleware.RequirePlatformAdmin(adminAccessService),
				middleware.RequireAdminOperationToken(
					adminOperationTokenService,
					middleware.AdminOperationTokenBinding{
						Operation:     "user.password_reset_email",
						TargetType:    "user",
						TargetIDParam: "userId",
					},
				),
				adminUserHandler.SendPasswordResetEmail,
			)

			// ---- 空间治理（platform_admin 全量 / space_admin 按 scope）----
			adminAPI.POST("/spaces", adminSpaceHandler.CreateSpace)
			// 空间封面资产上传或系统生成入口（创建空间前置资源准备）。
			adminAPI.POST("/spaces/cover-assets", adminSpaceHandler.CreateCoverAsset)
			adminAPI.GET("/spaces", adminSpaceHandler.ListSpaces)

			// 空间分类治理：分类是独立实体，不再依赖 system_configs 文本配置。
			adminAPI.GET("/spaces/categories", adminSpaceHandler.ListCategories)
			adminAPI.POST("/spaces/categories", adminSpaceHandler.CreateCategory)
			adminAPI.PATCH("/spaces/categories/:categoryId", adminSpaceHandler.RenameCategory)
			adminAPI.DELETE("/spaces/categories/:categoryId", adminSpaceHandler.DeleteCategory)

			// 空间成员治理（owner/collaborator/reader 关系维护）。
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

			// 空间状态治理（封禁/解封）。
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
			// 空间元数据更新（名称、描述、可见性、分类等）。
			adminAPI.PATCH(
				"/spaces/:spaceId/metadata",
				middleware.RequireSpaceManagement(adminAccessService, "spaceId"),
				adminSpaceHandler.UpdateMetadata,
			)
			// 空间所有权转移属于高风险治理动作，要求 operation token。
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

			// ---- 文档治理 ----
			adminAPI.GET("/documents", adminDocumentHandler.ListDocuments)
			adminAPI.GET("/document-shares", adminDocumentShareHandler.ListShares)
			adminAPI.PATCH("/document-shares/:shareId", adminDocumentShareHandler.UpdateShare)
			adminAPI.DELETE("/document-shares/:shareId", adminDocumentShareHandler.DisableShare)
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
			adminAPI.GET("/document-attachments", adminDocumentAttachmentHandler.ListAttachments)
			adminAPI.DELETE(
				"/document-attachments/:attachmentId",
				middleware.RequireAdminOperationToken(
					adminOperationTokenService,
					middleware.AdminOperationTokenBinding{
						Operation:     "document_attachment.delete",
						TargetType:    "document_attachment",
						TargetIDParam: "attachmentId",
					},
				),
				adminDocumentAttachmentHandler.DeleteAttachment,
			)
			adminAPI.GET("/document-images", adminDocumentImageAssetHandler.ListImageAssets)
			adminAPI.DELETE(
				"/document-images/:imageAssetId",
				middleware.RequireAdminOperationToken(
					adminOperationTokenService,
					middleware.AdminOperationTokenBinding{
						Operation:     "document_image.delete",
						TargetType:    "document_image",
						TargetIDParam: "imageAssetId",
					},
				),
				adminDocumentImageAssetHandler.DeleteImageAsset,
			)

			// ---- 文档模板治理（仅平台管理员）----
			adminAPI.GET(
				"/document-templates",
				middleware.RequirePlatformAdmin(adminAccessService),
				adminDocumentTemplateHandler.ListTemplates,
			)
			adminAPI.GET(
				"/document-templates/:templateId",
				middleware.RequirePlatformAdmin(adminAccessService),
				adminDocumentTemplateHandler.GetTemplate,
			)
			adminAPI.POST(
				"/document-templates",
				middleware.RequirePlatformAdmin(adminAccessService),
				adminDocumentTemplateHandler.CreateTemplate,
			)
			adminAPI.PUT(
				"/document-templates/:templateId",
				middleware.RequirePlatformAdmin(adminAccessService),
				middleware.RequireAdminOperationToken(
					adminOperationTokenService,
					middleware.AdminOperationTokenBinding{
						Operation:     "document_template.update",
						TargetType:    "document_template",
						TargetIDParam: "templateId",
					},
				),
				adminDocumentTemplateHandler.UpdateTemplate,
			)
			adminAPI.DELETE(
				"/document-templates/:templateId",
				middleware.RequirePlatformAdmin(adminAccessService),
				middleware.RequireAdminOperationToken(
					adminOperationTokenService,
					middleware.AdminOperationTokenBinding{
						Operation:     "document_template.delete",
						TargetType:    "document_template",
						TargetIDParam: "templateId",
					},
				),
				adminDocumentTemplateHandler.DeleteTemplate,
			)
			adminAPI.GET(
				"/document-template-scenes",
				middleware.RequirePlatformAdmin(adminAccessService),
				adminDocumentTemplateSceneHandler.ListScenes,
			)
			adminAPI.GET(
				"/document-template-scenes/:sceneKey",
				middleware.RequirePlatformAdmin(adminAccessService),
				adminDocumentTemplateSceneHandler.GetScene,
			)
			adminAPI.POST(
				"/document-template-scenes",
				middleware.RequirePlatformAdmin(adminAccessService),
				adminDocumentTemplateSceneHandler.CreateScene,
			)
			adminAPI.PUT(
				"/document-template-scenes/:sceneKey",
				middleware.RequirePlatformAdmin(adminAccessService),
				middleware.RequireAdminOperationToken(
					adminOperationTokenService,
					middleware.AdminOperationTokenBinding{
						Operation:     "document_template_scene.update",
						TargetType:    "document_template_scene",
						TargetIDParam: "sceneKey",
					},
				),
				adminDocumentTemplateSceneHandler.UpdateScene,
			)
			adminAPI.DELETE(
				"/document-template-scenes/:sceneKey",
				middleware.RequirePlatformAdmin(adminAccessService),
				middleware.RequireAdminOperationToken(
					adminOperationTokenService,
					middleware.AdminOperationTokenBinding{
						Operation:     "document_template_scene.delete",
						TargetType:    "document_template_scene",
						TargetIDParam: "sceneKey",
					},
				),
				adminDocumentTemplateSceneHandler.DeleteScene,
			)

			// ---- 主题治理（仅平台管理员）----
			// 根据当前项目策略，space_admin 无权访问主题管理接口。
			adminAPI.GET(
				"/themes",
				middleware.RequirePlatformAdmin(adminAccessService),
				adminThemeHandler.ListThemes,
			)
			adminAPI.POST(
				"/themes",
				middleware.RequirePlatformAdmin(adminAccessService),
				adminThemeHandler.CreateTheme,
			)
			adminAPI.PUT(
				"/themes/:themeId",
				middleware.RequirePlatformAdmin(adminAccessService),
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
				middleware.RequirePlatformAdmin(adminAccessService),
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

			// ---- 系统配置治理（仅平台管理员）----
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
			adminAPI.POST(
				"/system-configs/:key/cleanup/run",
				middleware.RequirePlatformAdmin(adminAccessService),
				middleware.RequireAdminOperationToken(
					adminOperationTokenService,
					middleware.AdminOperationTokenBinding{
						Operation:     "system_config.cleanup",
						TargetType:    "system_config",
						TargetIDParam: "key",
					},
				),
				adminSystemConfigHandler.RunDataRetentionCleanup,
			)
			adminAPI.POST(
				"/system-configs/:key/rebuild/run",
				middleware.RequirePlatformAdmin(adminAccessService),
				middleware.RequireAdminOperationToken(
					adminOperationTokenService,
					middleware.AdminOperationTokenBinding{
						Operation:     "system_config.rebuild",
						TargetType:    "system_config",
						TargetIDParam: "key",
					},
				),
				adminSystemConfigHandler.RunSearchIndexRebuild,
			)
			adminAPI.GET(
				"/system-configs/:key/rebuild/status",
				middleware.RequirePlatformAdmin(adminAccessService),
				adminSystemConfigHandler.GetSearchIndexStatus,
			)
			adminAPI.POST(
				"/system-configs/auth/providers/ldap/test",
				middleware.RequirePlatformAdmin(adminAccessService),
				adminSystemConfigHandler.TestLDAPConnection,
			)
			adminAPI.POST(
				"/system-configs/email/test-send",
				middleware.RequirePlatformAdmin(adminAccessService),
				adminSystemConfigHandler.TestEmailSend,
			)

			// ---- 全文检索分词治理（仅平台管理员）----
			adminAPI.GET(
				"/search/analyzers",
				middleware.RequirePlatformAdmin(adminAccessService),
				adminSearchAnalyzerHandler.ListAnalyzers,
			)
			adminAPI.GET(
				"/search/analyzers/:analyzer/dict",
				middleware.RequirePlatformAdmin(adminAccessService),
				adminSearchAnalyzerHandler.ListDictEntries,
			)
			adminAPI.POST(
				"/search/analyzers/:analyzer/dict",
				middleware.RequirePlatformAdmin(adminAccessService),
				middleware.RequireAdminOperationToken(
					adminOperationTokenService,
					middleware.AdminOperationTokenBinding{
						Operation:     "search_analyzer_dict.create",
						TargetType:    "search_analyzer",
						TargetIDParam: "analyzer",
					},
				),
				adminSearchAnalyzerHandler.CreateDictEntry,
			)
			adminAPI.PATCH(
				"/search/analyzers/:analyzer/dict/:entryId",
				middleware.RequirePlatformAdmin(adminAccessService),
				middleware.RequireAdminOperationToken(
					adminOperationTokenService,
					middleware.AdminOperationTokenBinding{
						Operation:     "search_analyzer_dict.update",
						TargetType:    "search_analyzer_dict",
						TargetIDParam: "entryId",
					},
				),
				adminSearchAnalyzerHandler.UpdateDictEntry,
			)
			adminAPI.DELETE(
				"/search/analyzers/:analyzer/dict/:entryId",
				middleware.RequirePlatformAdmin(adminAccessService),
				middleware.RequireAdminOperationToken(
					adminOperationTokenService,
					middleware.AdminOperationTokenBinding{
						Operation:     "search_analyzer_dict.delete",
						TargetType:    "search_analyzer_dict",
						TargetIDParam: "entryId",
					},
				),
				adminSearchAnalyzerHandler.DeleteDictEntry,
			)
			adminAPI.POST(
				"/search/analyzers/:analyzer/analyze-preview",
				middleware.RequirePlatformAdmin(adminAccessService),
				adminSearchAnalyzerHandler.AnalyzePreview,
			)
			adminAPI.POST(
				"/search/analyzers/:analyzer/reload",
				middleware.RequirePlatformAdmin(adminAccessService),
				middleware.RequireAdminOperationToken(
					adminOperationTokenService,
					middleware.AdminOperationTokenBinding{
						Operation:     "search_analyzer.reload",
						TargetType:    "search_analyzer",
						TargetIDParam: "analyzer",
					},
				),
				adminSearchAnalyzerHandler.ReloadAnalyzer,
			)

			// ---- 审计检索（仅平台管理员）----
			// 与主题治理一致，space_admin 在首期策略中不可访问全站审计日志。
			adminAPI.GET(
				"/audits",
				middleware.RequirePlatformAdmin(adminAccessService),
				adminAuditHandler.ListAudits,
			)
		}

		// ---- 业务端主题接口（非后台治理）----
		// 这里是普通业务用户使用的主题读取/应用入口，不要求 admin 身份。
		themeHandler := handler.NewThemeHandler(db)
		documentThemeHandler := handler.NewDocumentThemeHandler(db, readerRenderCache)
		// 获取前台可用主题列表（通常仅返回已启用主题）。
		api.GET("/themes", themeHandler.List)
		// 为指定文档设置主题。
		api.PUT("/docs/:docId/theme", documentThemeHandler.UpdateTheme)
	}

	// 统一未知路由错误协议，避免返回 HTML 默认页导致前端网关解析不一致。
	router.NoRoute(func(c *gin.Context) {
		response.RouterErrRouteNotFound.WriteWithStatus(c)
	})
	// 同一路径下方法不匹配时返回统一错误码，便于客户端按协议处理。
	router.NoMethod(func(c *gin.Context) {
		response.RouterErrMethodNotAllowed.WriteWithStatus(c)
	})

	// 所有依赖与路由装配完成后返回 engine，供 main 包启动 HTTP 服务。
	return router
}

func runSearchIndexJobLoop(
	ctx context.Context,
	logger *slog.Logger,
	jobService *service.SearchIndexJobService,
) {
	if jobService == nil {
		return
	}

	nextInterval := 5 * time.Second
	waitTimer := time.NewTimer(nextInterval)
	defer waitTimer.Stop()

	for range waitTimer.C {
		runResult, err := jobService.RunOnce(ctx)
		if err != nil {
			nextInterval = 2 * time.Second
			if logger != nil {
				logger.Error(
					"search index outbox worker run failed",
					logit.Error("error", err),
					slog.Duration("next_run_in", nextInterval),
				)
			}
		} else {
			switch {
			case runResult.Claimed == 0:
				nextInterval = 5 * time.Second
			case runResult.Retried > 0:
				nextInterval = 1 * time.Second
			default:
				nextInterval = 150 * time.Millisecond
			}

			if logger != nil && runResult.Claimed > 0 {
				logger.Info(
					"search index outbox worker run finished",
					slog.Int("claimed", runResult.Claimed),
					slog.Int("succeeded", runResult.Succeeded),
					slog.Int("retried", runResult.Retried),
					slog.Duration("next_run_in", nextInterval),
				)
			}
		}
		waitTimer.Reset(nextInterval)
	}
}

func buildReaderRenderCache(
	cfg config.Config,
	logger *slog.Logger,
	enabled bool,
) *rendercache.Cache {
	if !enabled {
		return nil
	}
	rendererVersion := strings.TrimSpace(cfg.SSRWorker.ProtocolVersion)
	if rendererVersion == "" {
		rendererVersion = "v1"
	}
	cache, err := rendercache.New(rendererVersion, rendercache.Options{
		MaxBytes:      defaultReaderRenderCacheMaxBytes,
		MaxEntryBytes: defaultReaderRenderCacheMaxEntryBytes,
		TTL:           defaultReaderRenderCacheTTL,
	})
	if err != nil {
		if logger != nil {
			logger.Error("init reader render cache failed", "error", err.Error())
		}
		return nil
	}
	return cache
}

func resolveBleveIndexPath(cfg config.Config) string {
	configuredPath := strings.TrimSpace(os.Getenv("SEARCH_BLEVE_DIR"))
	if configuredPath != "" {
		return filepath.Clean(configuredPath)
	}
	if strings.EqualFold(strings.TrimSpace(cfg.Env), "test") {
		return filepath.Clean(filepath.Join(os.TempDir(), "plaindoc", "search", "bleve-test"))
	}
	return filepath.Clean(filepath.Join("data", "search", "bleve"))
}
