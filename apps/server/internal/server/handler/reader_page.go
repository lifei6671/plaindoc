package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/rendercache"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
	"github.com/lifei6671/plaindoc/apps/server/internal/ssr/pool"
	"github.com/lifei6671/plaindoc/apps/server/internal/ssr/protocol"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/oklog/ulid/v2"
)

type readerPageHandler struct {
	authService                  *service.AuthService
	readerPageService            *service.ReaderPageService
	dispatcher                   *pool.Dispatcher
	renderCache                  *rendercache.Cache
	logger                       *slog.Logger
	webOrigin                    string
	onlyOfficeConfigService      *service.OnlyOfficeConfigService
	officeRenderingConfigService *service.OfficeRenderingConfigService
	onlyOfficeTokenService       *service.OnlyOfficeDocumentTokenService
}

type readerPageViewerIdentity struct {
	UserID        string `json:"userId,omitempty"`
	Name          string `json:"name,omitempty"`
	Authenticated bool   `json:"authenticated"`
}

type readerPagePayload struct {
	Space           service.ReaderSpaceViewModel                `json:"space"`
	Document        service.ReaderDocumentViewModel             `json:"document"`
	Attachments     []service.ReaderDocumentAttachmentViewModel `json:"attachments"`
	Tree            []service.ReaderTreeNodeViewModel           `json:"tree"`
	ActiveDocID     string                                      `json:"activeDocId"`
	RequestOrigin   string                                      `json:"requestOrigin,omitempty"`
	Viewer          readerPageViewerIdentity                    `json:"viewer"`
	OfficeRendering readerPageOfficeRenderingState              `json:"officeRendering"`
	Access          *readerPageAccessState                      `json:"access,omitempty"`
	Share           *readerPageShareState                       `json:"share,omitempty"`
}

type readerPageAccessState struct {
	Code          string `json:"code"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	RequiresLogin bool   `json:"requiresLogin"`
}

type readerPageShareState struct {
	Enabled            bool   `json:"enabled"`
	ShareID            string `json:"shareId,omitempty"`
	SpaceID            string `json:"spaceId,omitempty"`
	DocumentRouteKey   string `json:"documentRouteKey,omitempty"`
	BasePath           string `json:"basePath,omitempty"`
	AttachmentBasePath string `json:"attachmentBasePath,omitempty"`
}

type readerPageOfficeRenderingState struct {
	IndependentRenderEnabled            bool `json:"independentRenderEnabled"`
	FallbackToOnlyOfficeOnRenderFailure bool `json:"fallbackToOnlyOfficeOnRenderFailure"`
}

const (
	readerPublicBrowserMaxAgeSeconds = 60
	readerPublicBrowserSWRSeconds    = 120
)

// NewReaderPageHandler 创建阅读页 SSR 处理器。
func NewReaderPageHandler(
	authService *service.AuthService,
	readerPageService *service.ReaderPageService,
	dispatcher *pool.Dispatcher,
	renderCache *rendercache.Cache,
	logger *slog.Logger,
	webOrigin string,
	onlyOfficeConfigService *service.OnlyOfficeConfigService,
	officeRenderingConfigService *service.OfficeRenderingConfigService,
	onlyOfficeTokenService *service.OnlyOfficeDocumentTokenService,
) *readerPageHandler {
	return &readerPageHandler{
		authService:                  authService,
		readerPageService:            readerPageService,
		dispatcher:                   dispatcher,
		renderCache:                  renderCache,
		logger:                       logger,
		webOrigin:                    normalizeWebOrigin(webOrigin),
		onlyOfficeConfigService:      onlyOfficeConfigService,
		officeRenderingConfigService: officeRenderingConfigService,
		onlyOfficeTokenService:       onlyOfficeTokenService,
	}
}

// Space 渲染空间阅读入口：自动跳转到首篇可读文档。
func (h *readerPageHandler) Space(c *gin.Context) {
	if h == nil || h.readerPageService == nil {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(readerFallbackUnavailableHTML))
		return
	}

	spaceID := strings.TrimSpace(c.Param("spaceId"))
	if spaceID == "" {
		h.renderFriendlyErrorPage(
			c,
			http.StatusBadRequest,
			"链接无效",
			"空间标识不正确，请检查链接后重试。",
		)
		return
	}

	viewer := h.resolveOptionalViewerIdentity(c)
	documentID, err := h.readerPageService.ResolveLandingDocumentID(
		c.Request.Context(),
		spaceID,
		viewer.UserID,
	)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrSpaceNotFound), errors.Is(err, service.ErrDocumentNotFound):
			h.renderFriendlyErrorPage(
				c,
				http.StatusNotFound,
				"页面不存在",
				"空间不存在，或该空间下暂无可访问文档。",
			)
		case errors.Is(err, service.ErrViewerLoginRequired):
			h.renderFriendlyErrorPage(
				c,
				http.StatusForbidden,
				"无权限访问",
				"当前空间需要登录后访问，且本页面不会自动跳转登录。",
			)
		case errors.Is(err, service.ErrSpaceAccessDenied), errors.Is(err, service.ErrDocumentAccessDenied):
			h.renderFriendlyErrorPage(
				c,
				http.StatusForbidden,
				"无权限访问",
				"你没有权限访问该空间或文档，请联系空间管理员。",
			)
		default:
			h.logError("resolve reader landing document failed", err, "space_id", spaceID)
			h.renderFriendlyErrorPage(
				c,
				http.StatusInternalServerError,
				"加载失败",
				"阅读页面暂时不可用，请稍后刷新重试。",
			)
		}
		return
	}

	targetPath := "/r/" + url.PathEscape(spaceID) + "/" + url.PathEscape(documentID)
	c.Redirect(http.StatusSeeOther, targetPath)
}

// Page 渲染空间阅读页。
func (h *readerPageHandler) Page(c *gin.Context) {
	if h == nil || h.readerPageService == nil {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(readerFallbackUnavailableHTML))
		return
	}

	spaceID := strings.TrimSpace(c.Param("spaceId"))
	documentID := strings.TrimSpace(c.Param("docId"))
	if spaceID == "" {
		h.renderFriendlyErrorPage(
			c,
			http.StatusBadRequest,
			"链接无效",
			"空间标识不正确，请检查链接后重试。",
		)
		return
	}
	if documentID == "" {
		h.renderFriendlyErrorPage(
			c,
			http.StatusBadRequest,
			"链接无效",
			"文档标识不正确，请检查链接后重试。",
		)
		return
	}

	viewer := h.resolveOptionalViewerIdentity(c)
	viewModel, err := h.readerPageService.BuildPage(
		c.Request.Context(),
		spaceID,
		documentID,
		viewer.UserID,
	)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrSpaceNotFound), errors.Is(err, service.ErrDocumentNotFound):
			h.renderFriendlyErrorPage(
				c,
				http.StatusNotFound,
				"页面不存在",
				"文档不存在，或已被删除。",
			)
		case errors.Is(err, service.ErrViewerLoginRequired):
			h.renderReaderAccessDeniedPage(
				c,
				spaceID,
				documentID,
				viewer,
				"当前文档需要登录后访问。",
				true,
			)
		case errors.Is(err, service.ErrSpaceAccessDenied), errors.Is(err, service.ErrDocumentAccessDenied):
			h.renderReaderAccessDeniedPage(
				c,
				spaceID,
				documentID,
				viewer,
				"你没有权限访问该文档，请联系空间管理员。",
				false,
			)
		default:
			h.logError("reader page build failed", err, "space_id", spaceID, "document_id", documentID)
			h.renderFriendlyErrorPage(
				c,
				http.StatusInternalServerError,
				"加载失败",
				"阅读页面暂时不可用，请稍后刷新重试。",
			)
		}
		return
	}

	canonicalDocumentRouteKey := strings.TrimSpace(viewModel.Document.RouteKey)
	if canonicalDocumentRouteKey == "" {
		canonicalDocumentRouteKey = strings.TrimSpace(viewModel.Document.ID)
	}
	if canonicalDocumentRouteKey != "" && canonicalDocumentRouteKey != documentID {
		// 兼容历史 node_id/document_id 入参：统一重定向到 canonical 文档路由 key（slug 或 document_id）。
		targetPath := "/r/" + url.PathEscape(spaceID) + "/" + url.PathEscape(canonicalDocumentRouteKey)
		c.Redirect(http.StatusSeeOther, targetPath)
		return
	}

	payload := readerPagePayload{
		Space:           viewModel.Space,
		Document:        viewModel.Document,
		Attachments:     viewModel.Attachments,
		Tree:            viewModel.Tree,
		ActiveDocID:     viewModel.ActiveDocID,
		RequestOrigin:   resolveRequestOrigin(c),
		Viewer:          viewer,
		OfficeRendering: h.resolveOfficeRenderingState(c.Request.Context()),
	}
	h.renderReaderPayload(c, http.StatusOK, spaceID, documentID, payload, true)
}

// GetOnlyOfficeViewConfig 返回阅读页 Office 文档只读配置。
func (h *readerPageHandler) GetOnlyOfficeViewConfig(c *gin.Context) {
	if h == nil || h.readerPageService == nil || h.onlyOfficeConfigService == nil || h.onlyOfficeTokenService == nil {
		response.InternalError(c)
		return
	}

	spaceID := strings.TrimSpace(c.Param("spaceId"))
	documentID := strings.TrimSpace(c.Param("docId"))
	if spaceID == "" {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidSpaceID, "空间 ID 不能为空")
		return
	}
	if documentID == "" {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidDocumentID, "文档 ID 不能为空")
		return
	}

	appendVaryHeader(c, "Authorization")
	appendVaryHeader(c, "Cookie")
	c.Header("Cache-Control", "private, no-store, max-age=0")

	viewer := h.resolveOptionalViewerIdentity(c)
	viewModel, err := h.readerPageService.BuildPage(
		c.Request.Context(),
		spaceID,
		documentID,
		viewer.UserID,
	)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrSpaceNotFound), errors.Is(err, service.ErrDocumentNotFound):
			response.Error(c, http.StatusNotFound, response.CodeDocumentNotFound, "文档不存在，或已被删除")
		case errors.Is(err, service.ErrViewerLoginRequired):
			response.Error(c, http.StatusForbidden, response.CodeUnauthorized, "当前文档需要登录后访问")
		case errors.Is(err, service.ErrSpaceAccessDenied), errors.Is(err, service.ErrDocumentAccessDenied):
			response.Error(c, http.StatusForbidden, response.CodeForbidden, "你没有权限访问该文档")
		default:
			h.logError("reader onlyoffice view config build failed", err, "space_id", spaceID, "document_id", documentID)
			response.InternalError(c)
		}
		return
	}

	configPayload, err := buildOnlyOfficeViewConfig(
		c.Request.Context(),
		h.onlyOfficeConfigService,
		h.onlyOfficeTokenService,
		viewModel.Document,
		viewer.UserID,
		viewer.Name,
	)
	if err != nil {
		h.logError("reader onlyoffice view config resolve failed", err, "space_id", spaceID, "document_id", documentID)
		switch {
		case errors.Is(err, errOnlyOfficeViewConfigNotOfficeDocument):
			response.Error(c, http.StatusBadRequest, response.CodeInvalidOperation, "当前文档不是 Office 文档")
		case errors.Is(err, errOnlyOfficeViewConfigDisabled):
			response.Error(c, http.StatusBadRequest, response.CodeInvalidRequest, "ONLYOFFICE 阅读能力未启用")
		default:
			response.InternalError(c)
		}
		return
	}

	response.JSON(c, http.StatusOK, configPayload)
}

// CreateOfficeSourceAccessLink 返回阅读页 Office 原文件下载链接。
func (h *readerPageHandler) CreateOfficeSourceAccessLink(c *gin.Context) {
	if h == nil || h.readerPageService == nil || h.onlyOfficeTokenService == nil {
		response.InternalError(c)
		return
	}

	spaceID := strings.TrimSpace(c.Param("spaceId"))
	documentID := strings.TrimSpace(c.Param("docId"))
	if spaceID == "" {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidSpaceID, "空间 ID 不能为空")
		return
	}
	if documentID == "" {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidDocumentID, "文档 ID 不能为空")
		return
	}

	appendVaryHeader(c, "Authorization")
	appendVaryHeader(c, "Cookie")
	c.Header("Cache-Control", "private, no-store, max-age=0")

	viewer := h.resolveOptionalViewerIdentity(c)
	viewModel, err := h.readerPageService.BuildPage(
		c.Request.Context(),
		spaceID,
		documentID,
		viewer.UserID,
	)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrSpaceNotFound), errors.Is(err, service.ErrDocumentNotFound):
			response.Error(c, http.StatusNotFound, response.CodeDocumentNotFound, "文档不存在，或已被删除")
		case errors.Is(err, service.ErrViewerLoginRequired):
			response.Error(c, http.StatusForbidden, response.CodeUnauthorized, "当前文档需要登录后访问")
		case errors.Is(err, service.ErrSpaceAccessDenied), errors.Is(err, service.ErrDocumentAccessDenied):
			response.Error(c, http.StatusForbidden, response.CodeForbidden, "你没有权限访问该文档")
		default:
			h.logError("reader office source access link build failed", err, "space_id", spaceID, "document_id", documentID)
			response.InternalError(c)
		}
		return
	}

	downloadURL, fileName, expiresAt, err := buildOfficeSourceAccessLink(
		h.onlyOfficeTokenService,
		viewModel.Document,
		viewer.UserID,
	)
	if err != nil {
		h.logError("reader office source access link resolve failed", err, "space_id", spaceID, "document_id", documentID)
		switch {
		case errors.Is(err, errOnlyOfficeViewConfigNotOfficeDocument):
			response.Error(c, http.StatusBadRequest, response.CodeInvalidOperation, "当前文档不是 Office 文档")
		case errors.Is(err, errOnlyOfficeViewConfigSourceBlobEmpty):
			response.Error(c, http.StatusNotFound, response.CodeDocumentNotFound, "原文件不存在，无法下载")
		default:
			response.InternalError(c)
		}
		return
	}

	response.JSON(c, http.StatusOK, officeSourceAccessLinkResponse{
		URL:       downloadURL,
		FileName:  fileName,
		ExpiresAt: expiresAt.UTC().Format(time.RFC3339),
	})
}

func (h *readerPageHandler) renderReaderPayload(
	c *gin.Context,
	statusCode int,
	spaceID string,
	documentID string,
	payload readerPagePayload,
	cacheable bool,
) {
	applyReaderBrowserCacheHeaders(c, statusCode, payload, cacheable)

	if h.dispatcher != nil {
		renderPayload := payload
		if cacheable {
			// 鉴权在上游已完成，渲染缓存不携带用户态，避免缓存碎片与用户信息污染。
			renderPayload.Viewer = readerPageViewerIdentity{}
		}
		payloadBytes, marshalErr := json.Marshal(renderPayload)
		if marshalErr != nil {
			h.logError("marshal reader page payload failed", marshalErr, "space_id", spaceID, "document_id", documentID)
		} else {
			if cacheable && h.renderCache != nil {
				cacheKey := buildReaderRenderCacheKey(h.renderCache, documentID, payloadBytes)
				if cacheKey != "" {
					cachedHTML, cacheErr := h.renderCache.GetOrRender(
						c.Request.Context(),
						strings.TrimSpace(documentID),
						cacheKey,
						func(ctx context.Context) ([]byte, error) {
							return h.renderReaderPayloadByDispatcher(ctx, payloadBytes)
						},
					)
					if cacheErr == nil && len(cachedHTML) > 0 {
						c.Data(statusCode, "text/html; charset=utf-8", cachedHTML)
						return
					}
					if cacheErr != nil {
						h.logWarn(
							"reader page render cache bypassed",
							"space_id",
							spaceID,
							"document_id",
							documentID,
							"error",
							cacheErr.Error(),
						)
					}
				}
			}

			renderedHTML, renderResponse, renderErr := h.renderReaderPayloadByDispatcherWithState(
				c.Request.Context(),
				payloadBytes,
			)
			if renderErr == nil && len(renderedHTML) > 0 {
				c.Data(statusCode, "text/html; charset=utf-8", renderedHTML)
				return
			}
			if renderErr != nil {
				h.logError(
					"reader page ssr render failed",
					renderErr,
					"space_id",
					spaceID,
					"document_id",
					documentID,
				)
			} else if renderResponse != nil {
				h.logWarn(
					"reader page ssr render rejected",
					"space_id",
					spaceID,
					"document_id",
					documentID,
					"error_code",
					resolveReaderErrorCode(renderResponse.Error),
					"error_message",
					resolveReaderErrorMessage(renderResponse.Error),
				)
			}
		}
	}

	c.Data(statusCode, "text/html; charset=utf-8", []byte(buildReaderFallbackHTML(payload)))
}

func applyReaderBrowserCacheHeaders(
	c *gin.Context,
	statusCode int,
	payload readerPagePayload,
	cacheable bool,
) {
	appendVaryHeader(c, "Authorization")
	appendVaryHeader(c, "Cookie")
	if isReaderBrowserCacheEligible(statusCode, payload, cacheable) {
		c.Header(
			"Cache-Control",
			fmt.Sprintf(
				"public, max-age=%d, stale-while-revalidate=%d",
				readerPublicBrowserMaxAgeSeconds,
				readerPublicBrowserSWRSeconds,
			),
		)
		return
	}
	c.Header("Cache-Control", "private, no-store, max-age=0")
}

func isReaderBrowserCacheEligible(
	statusCode int,
	payload readerPagePayload,
	cacheable bool,
) bool {
	if !cacheable || statusCode != http.StatusOK {
		return false
	}
	if payload.Viewer.Authenticated {
		return false
	}
	if payload.Access != nil && strings.TrimSpace(payload.Access.Code) != "" {
		return false
	}
	if strings.TrimSpace(payload.Document.ID) == "" {
		return false
	}
	// 仅公开文档允许浏览器短缓存，降低权限变更后的可见性残留风险。
	if payload.Document.Visibility != models.VisibilityPublic {
		return false
	}
	return true
}

func (h *readerPageHandler) renderReaderPayloadByDispatcher(
	ctx context.Context,
	payloadBytes []byte,
) ([]byte, error) {
	renderedHTML, _, renderErr := h.renderReaderPayloadByDispatcherWithState(ctx, payloadBytes)
	return renderedHTML, renderErr
}

func (h *readerPageHandler) renderReaderPayloadByDispatcherWithState(
	ctx context.Context,
	payloadBytes []byte,
) ([]byte, *protocol.RenderResponse, error) {
	if h == nil || h.dispatcher == nil {
		return nil, nil, errors.New("reader page dispatcher not initialized")
	}
	renderResponse, renderErr := h.dispatcher.Render(ctx, protocol.RenderRequest{
		ID:      strings.ToLower(ulid.Make().String()),
		Type:    protocol.MessageTypeRender,
		Route:   "space-reader",
		Payload: payloadBytes,
	})
	if renderErr != nil {
		return nil, nil, renderErr
	}
	if !renderResponse.OK || strings.TrimSpace(renderResponse.HTML) == "" {
		return nil, &renderResponse, nil
	}
	return []byte(renderResponse.HTML), &renderResponse, nil
}

func buildReaderRenderCacheKey(
	cache *rendercache.Cache,
	documentID string,
	payloadBytes []byte,
) string {
	if cache == nil {
		return ""
	}
	normalizedDocumentID := strings.TrimSpace(documentID)
	if normalizedDocumentID == "" || len(payloadBytes) == 0 {
		return ""
	}
	payloadHash := sha256.Sum256(payloadBytes)
	return cache.BuildKey(
		normalizedDocumentID,
		time.Time{},
		hex.EncodeToString(payloadHash[:]),
	)
}

func (h *readerPageHandler) resolveOfficeRenderingState(ctx context.Context) readerPageOfficeRenderingState {
	if h == nil || h.officeRenderingConfigService == nil {
		return readerPageOfficeRenderingState{
			IndependentRenderEnabled:            false,
			FallbackToOnlyOfficeOnRenderFailure: true,
		}
	}
	config, err := h.officeRenderingConfigService.GetConfig(ctx)
	if err != nil {
		return readerPageOfficeRenderingState{
			IndependentRenderEnabled:            false,
			FallbackToOnlyOfficeOnRenderFailure: true,
		}
	}
	return readerPageOfficeRenderingState{
		IndependentRenderEnabled:            config.IndependentRenderEnabled,
		FallbackToOnlyOfficeOnRenderFailure: config.FallbackToOnlyOfficeOnRenderFailure,
	}
}

func (h *readerPageHandler) renderReaderAccessDeniedPage(
	c *gin.Context,
	spaceID string,
	documentID string,
	viewer readerPageViewerIdentity,
	description string,
	requiresLogin bool,
) {
	spaceTitle := "空间阅读"
	normalizedSpaceID := strings.TrimSpace(spaceID)
	normalizedDocumentID := strings.TrimSpace(documentID)
	if normalizedDocumentID == "" {
		normalizedDocumentID = "unknown"
	}
	accessDescription := strings.TrimSpace(description)
	if accessDescription == "" {
		accessDescription = "你没有权限访问该文档，请联系空间管理员。"
	}

	spaceViewModel := service.ReaderSpaceViewModel{
		ID:    normalizedSpaceID,
		Name:  spaceTitle,
		Title: "无权限访问 - " + spaceTitle,
	}
	tree := []service.ReaderTreeNodeViewModel{}
	activeDocID := normalizedDocumentID

	// 优先复用同空间下的空间上下文（空间名称与目录树），避免无权限页出现异常标题/空目录。
	if h != nil && h.readerPageService != nil {
		if resolvedSpace, resolvedTree, contextErr := h.readerPageService.BuildSpaceContext(
			c.Request.Context(),
			normalizedSpaceID,
			strings.TrimSpace(viewer.UserID),
		); contextErr == nil {
			if resolvedName := strings.TrimSpace(resolvedSpace.Name); resolvedName != "" {
				spaceTitle = resolvedName
			}
			spaceViewModel = service.ReaderSpaceViewModel{
				ID:    normalizedSpaceID,
				Name:  spaceTitle,
				Title: "无权限访问 - " + spaceTitle,
			}
			tree = resolvedTree
			if activeDocID == "" {
				activeDocID = normalizedDocumentID
			}
		}
	}

	payload := readerPagePayload{
		Space: spaceViewModel,
		Document: service.ReaderDocumentViewModel{
			ID:             normalizedDocumentID,
			NodeID:         normalizedDocumentID,
			ThemeID:        "default",
			Format:         models.DocumentFormatMarkdown,
			Visibility:     models.VisibilityMember,
			Title:          "无权限访问",
			ContentMD:      "",
			Version:        0,
			ContentVersion: 0,
			AuthorNickname: "未知作者",
			UpdatedAt:      time.Now().UTC().Format(time.RFC3339Nano),
		},
		Attachments:   []service.ReaderDocumentAttachmentViewModel{},
		Tree:          tree,
		ActiveDocID:   activeDocID,
		RequestOrigin: resolveRequestOrigin(c),
		Viewer:        viewer,
		Access: &readerPageAccessState{
			Code:          response.CodeForbidden,
			Title:         "无权限访问",
			Description:   accessDescription,
			RequiresLogin: requiresLogin,
		},
	}
	h.renderReaderPayload(
		c,
		http.StatusForbidden,
		normalizedSpaceID,
		normalizedDocumentID,
		payload,
		false,
	)
}

func (h *readerPageHandler) resolveOptionalViewerIdentity(c *gin.Context) readerPageViewerIdentity {
	if h == nil || h.authService == nil {
		return readerPageViewerIdentity{}
	}

	rawToken, ok := optionalAccessTokenFromRequest(c)
	if !ok {
		return readerPageViewerIdentity{}
	}

	session, err := h.authService.Me(c.Request.Context(), rawToken)
	if err != nil {
		return readerPageViewerIdentity{}
	}
	userID := strings.TrimSpace(session.User.ID)
	name := strings.TrimSpace(session.User.Name)
	if name == "" {
		name = strings.TrimSpace(session.User.Email)
	}
	if name == "" {
		name = "用户"
	}
	return readerPageViewerIdentity{
		UserID:        userID,
		Name:          name,
		Authenticated: userID != "",
	}
}

func (h *readerPageHandler) logError(message string, err error, attrs ...any) {
	if h == nil || h.logger == nil {
		return
	}
	baseAttrs := []any{"error", err.Error()}
	h.logger.Error(message, append(baseAttrs, attrs...)...)
}

func (h *readerPageHandler) logWarn(message string, attrs ...any) {
	if h == nil || h.logger == nil {
		return
	}
	h.logger.Warn(message, attrs...)
}

func (h *readerPageHandler) redirectToLogin(c *gin.Context) {
	if c == nil {
		return
	}
	c.Redirect(http.StatusSeeOther, h.buildWebAuthEntryURL(c, "/login"))
}

func (h *readerPageHandler) buildWebAuthEntryURL(c *gin.Context, targetPath string) string {
	path := strings.TrimSpace(targetPath)
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	baseOrigin := strings.TrimSpace(h.webOrigin)
	if baseOrigin == "" {
		return path
	}

	redirectURL := buildRequestAbsoluteURL(c)
	if redirectURL == "" {
		return baseOrigin + path
	}
	return baseOrigin + path + "?redirect=" + url.QueryEscape(redirectURL)
}

func resolveRequestOrigin(c *gin.Context) string {
	absoluteURL := strings.TrimSpace(buildRequestAbsoluteURL(c))
	if absoluteURL == "" {
		return ""
	}
	parsedURL, err := url.Parse(absoluteURL)
	if err != nil {
		return ""
	}
	scheme := strings.TrimSpace(parsedURL.Scheme)
	host := strings.TrimSpace(parsedURL.Host)
	if scheme == "" || host == "" {
		return ""
	}
	return scheme + "://" + host
}

func (h *readerPageHandler) renderFriendlyErrorPage(
	c *gin.Context,
	statusCode int,
	title string,
	description string,
) {
	if c == nil {
		return
	}

	appendVaryHeader(c, "Authorization")
	appendVaryHeader(c, "Cookie")
	c.Header("Cache-Control", "private, no-store, max-age=0")
	c.Data(
		statusCode,
		"text/html; charset=utf-8",
		[]byte(buildReaderErrorHTML(statusCode, title, description)),
	)
}

func resolveReaderErrorCode(renderError *protocol.RenderError) string {
	if renderError == nil {
		return ""
	}
	return strings.TrimSpace(renderError.Code)
}

func resolveReaderErrorMessage(renderError *protocol.RenderError) string {
	if renderError == nil {
		return ""
	}
	return strings.TrimSpace(renderError.Message)
}

func buildReaderFallbackHTML(payload readerPagePayload) string {
	pageTitle := strings.TrimSpace(payload.Space.Title)
	if pageTitle == "" {
		pageTitle = "PlainDoc 阅读页"
	}
	documentTitle := strings.TrimSpace(payload.Document.Title)
	if documentTitle == "" {
		documentTitle = "未命名文档"
	}
	spaceName := strings.TrimSpace(payload.Space.Name)
	if spaceName == "" {
		spaceName = "未命名空间"
	}
	accessTitle := ""
	accessDescription := ""
	isAccessDenied := false
	if payload.Access != nil {
		accessTitle = strings.TrimSpace(payload.Access.Title)
		accessDescription = strings.TrimSpace(payload.Access.Description)
		isAccessDenied = accessTitle != "" || accessDescription != ""
	}

	stateJSONBytes, err := json.Marshal(payload)
	if err != nil {
		stateJSONBytes = []byte("{}")
	}
	stateJSON := escapeJSONForHTMLScript(string(stateJSONBytes))

	escapedPageTitle := template.HTMLEscapeString(composeSEOTitle(pageTitle))
	escapedDocumentTitle := template.HTMLEscapeString(documentTitle)
	escapedSpaceName := template.HTMLEscapeString(spaceName)
	escapedDocument := template.HTMLEscapeString(payload.Document.ContentMD)
	escapedAccessTitle := template.HTMLEscapeString(accessTitle)
	escapedAccessDescription := template.HTMLEscapeString(accessDescription)
	robotsMeta := ""
	if models.IsOfficeDocumentFormat(payload.Document.Format) {
		robotsMeta = `    <meta name="robots" content="noindex, nofollow" />`
	}

	bodyContent := fmt.Sprintf("<pre>%s</pre>", escapedDocument)
	headerContent := fmt.Sprintf(
		`<p class="reader-meta">空间：%s</p><h1 class="reader-title">%s</h1>`,
		escapedSpaceName,
		escapedDocumentTitle,
	)
	if isAccessDenied {
		if accessTitle == "" {
			accessTitle = "无权限访问"
			escapedAccessTitle = template.HTMLEscapeString(accessTitle)
		}
		if accessDescription == "" {
			accessDescription = "你没有权限访问该文档，请联系空间管理员。"
			escapedAccessDescription = template.HTMLEscapeString(accessDescription)
		}
		headerContent = fmt.Sprintf(`<h1 class="reader-title">%s</h1>`, escapedAccessTitle)
		bodyContent = fmt.Sprintf(
			`<section class="reader-fallback-denied"><h2>%s</h2><p>%s</p></section>`,
			escapedAccessTitle,
			escapedAccessDescription,
		)
	}

	return fmt.Sprintf(`<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
%s
    <title>%s</title>
    <link rel="preload" href="/assets/google-sans-code-latin-400-normal.woff2" as="font" type="font/woff2" crossorigin />
    <link rel="stylesheet" href="/assets/font-google-sans-code.css?v=20260224" />
    <style>
      body { margin: 0; font-family: "Google Sans Code", "PingFang SC", "Microsoft YaHei", "Helvetica Neue", Arial, sans-serif; background: #f5f7fb; color: #1f2937; }
      .reader-shell { max-width: 980px; margin: 24px auto; background: #fff; border: 1px solid #dbe2ea; border-radius: 14px; padding: 20px; box-shadow: 0 10px 30px rgba(15, 23, 42, 0.06); }
      .reader-meta { font-size: 12px; color: #6b7280; margin-bottom: 12px; }
      .reader-title { font-size: 28px; font-weight: 700; margin: 0 0 18px; }
      .reader-fallback-notice { margin: 0 0 14px; color: #92400e; background: #fff7ed; border: 1px solid #fed7aa; border-radius: 8px; padding: 10px 12px; }
      .reader-fallback-denied { margin: 0; border: 1px solid #e2e8f0; background: #f8fafc; border-radius: 12px; padding: 16px; }
      .reader-fallback-denied h2 { margin: 0; color: #0f172a; font-size: 18px; line-height: 1.4; }
      .reader-fallback-denied p { margin: 8px 0 0; color: #475569; font-size: 14px; line-height: 1.7; }
      pre { white-space: pre-wrap; word-break: break-word; margin: 0; font-size: 14px; line-height: 1.7; font-family: "Google Sans Code", "SFMono-Regular", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace; }
    </style>
  </head>
  <body>
    <main class="reader-shell">
      %s
      <p class="reader-fallback-notice">SSR 渲染暂时不可用，已降级为基础阅读模式。</p>
      %s
    </main>
    <script id="plaindoc-reader-initial-state" type="application/json">%s</script>
  </body>
</html>`, robotsMeta, escapedPageTitle, headerContent, bodyContent, stateJSON)
}

func escapeJSONForHTMLScript(rawJSON string) string {
	return strings.NewReplacer(
		"<", "\\u003c",
		">", "\\u003e",
		"&", "\\u0026",
	).Replace(rawJSON)
}

func buildReaderErrorHTML(statusCode int, title string, description string) string {
	pageTitle := strings.TrimSpace(title)
	if pageTitle == "" {
		pageTitle = "页面异常"
	}
	pageDescription := strings.TrimSpace(description)
	if pageDescription == "" {
		pageDescription = "阅读页面暂时不可用，请稍后重试。"
	}

	escapedPageTitle := template.HTMLEscapeString(composeSEOTitle(pageTitle))
	escapedErrorTitle := template.HTMLEscapeString(pageTitle)
	escapedDescription := template.HTMLEscapeString(pageDescription)
	escapedStatusCode := template.HTMLEscapeString(fmt.Sprintf("%d", statusCode))

	return fmt.Sprintf(`<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>%s</title>
    <link rel="preload" href="/assets/google-sans-code-latin-400-normal.woff2" as="font" type="font/woff2" crossorigin />
    <link rel="stylesheet" href="/assets/font-google-sans-code.css?v=20260224" />
    <style>
      body { margin: 0; min-height: 100vh; display: flex; align-items: center; justify-content: center; padding: 24px; box-sizing: border-box; font-family: "Google Sans Code", "PingFang SC", "Microsoft YaHei", "Helvetica Neue", Arial, sans-serif; color: #1f2937; background: linear-gradient(180deg, #f8fafc 0%%, #eef2ff 100%%); }
      .error-shell { width: 100%%; max-width: 560px; background: #fff; border: 1px solid #dbe2ea; border-radius: 16px; box-shadow: 0 16px 36px rgba(15, 23, 42, 0.08); padding: 24px; }
      .error-code { margin: 0; font-size: 12px; color: #64748b; letter-spacing: 0.04em; text-transform: uppercase; }
      .error-title { margin: 10px 0 8px; font-size: 28px; line-height: 1.25; color: #0f172a; }
      .error-desc { margin: 0; color: #475569; font-size: 14px; line-height: 1.7; }
      .error-actions { margin-top: 18px; display: flex; gap: 10px; flex-wrap: wrap; }
      .error-link { display: inline-flex; align-items: center; justify-content: center; min-height: 36px; padding: 0 14px; border-radius: 10px; border: 1px solid #cbd5e1; color: #0f172a; text-decoration: none; font-size: 13px; font-weight: 600; background: #fff; }
      .error-link:hover { background: #f8fafc; }
      .error-link-primary { border-color: #2563eb; background: #2563eb; color: #fff; }
      .error-link-primary:hover { background: #1d4ed8; }
    </style>
  </head>
  <body>
    <main class="error-shell">
      <p class="error-code">HTTP %s</p>
      <h1 class="error-title">%s</h1>
      <p class="error-desc">%s</p>
      <div class="error-actions">
        <a class="error-link error-link-primary" href="/">返回首页</a>
        <a class="error-link" href="javascript:history.back()">返回上一页</a>
      </div>
    </main>
  </body>
</html>`, escapedPageTitle, escapedStatusCode, escapedErrorTitle, escapedDescription)
}

const readerFallbackUnavailableHTML = `<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>PlainDoc 阅读页 - PlainDoc - 一个适合中小团队文档在线管理系统</title>
  </head>
  <body>
    <p>reader page is temporarily unavailable</p>
  </body>
</html>`

func composeSEOTitle(pageTitle string) string {
	normalizedTitle := strings.TrimSpace(pageTitle)
	if normalizedTitle == "" {
		return seoTitleSuffix
	}
	return normalizedTitle + " - " + seoTitleSuffix
}
