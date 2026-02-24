package handler

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/view"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
)

type homeHandler struct {
	authService *service.AuthService
	homeService *service.HomeService
	webOrigin   string
}

const exploreAllCategoryRouteToken = "all"
const seoTitleSuffix = "PlainDoc - 一个适合中小团队文档在线管理系统"

type homeViewerIdentity struct {
	UserID        string
	Name          string
	Email         string
	AvatarURL     string
	Authenticated bool
}

// NewHomeHandler 创建首页/分类页 SSR 处理器。
func NewHomeHandler(
	authService *service.AuthService,
	homeService *service.HomeService,
	webOrigin string,
) *homeHandler {
	return &homeHandler{
		authService: authService,
		homeService: homeService,
		webOrigin:   normalizeWebOrigin(webOrigin),
	}
}

// Home 渲染首页（左侧分类 + 可见空间列表）。
func (h *homeHandler) Home(c *gin.Context) {
	h.renderPage(c, false)
}

// Explore 渲染分类页（顶部分类导航 + 分类空间列表）。
func (h *homeHandler) Explore(c *gin.Context) {
	h.renderPage(c, true)
}

// Logout 处理首页导航发起的退出登录。
func (h *homeHandler) Logout(c *gin.Context) {
	if h != nil && h.authService != nil {
		if rawToken, ok := optionalAccessTokenFromRequest(c); ok {
			_ = h.authService.Logout(c.Request.Context(), rawToken)
		}
	}

	clearSessionCookies(c)
	c.Redirect(http.StatusSeeOther, "/")
}

func (h *homeHandler) renderPage(c *gin.Context, explore bool) {
	if h == nil || h.homeService == nil {
		renderHomeFallback(c)
		return
	}

	page := parsePositiveInt(c.Query("page"), 1)
	pageSize := parsePositiveInt(c.Query("pageSize"), 20)
	categoryID := ""
	rawCategoryID := ""
	isExploreAll := false
	if explore {
		rawCategoryID = strings.TrimSpace(c.Param("categoryId"))
		categoryID, isExploreAll = normalizeExploreCategoryID(rawCategoryID)
	}

	viewerIdentity := h.resolveOptionalViewerIdentity(c)
	record, err := h.homeService.GetPage(c.Request.Context(), service.HomepageQueryInput{
		ViewerUserID: viewerIdentity.UserID,
		CategoryID:   categoryID,
		Page:         page,
		PageSize:     pageSize,
	})
	if err != nil {
		renderHomeFallback(c)
		return
	}

	c.Header("Cache-Control", strings.TrimSpace(record.CacheControl))
	appendVaryHeader(c, "Authorization")
	appendVaryHeader(c, "Cookie")

	canonicalURL := "/"
	title := "空间探索"
	description := "探索你当前可见的空间，按发布时间倒序浏览。"
	if explore {
		canonicalURL = "/explore/" + rawCategoryID
		title = "分类探索"
		description = "按分类浏览你当前可见的空间。"
		if isExploreAll {
			title = "全部空间 - 空间探索"
			description = "浏览你可见的全部空间。"
		} else if strings.TrimSpace(record.ActiveCategory) != "" {
			title = record.ActiveCategory + " - 空间探索"
		}
	}

	categoryItems := make([]view.HomeCategoryViewData, 0, len(record.Categories)+1)
	allCategoryURL := "/"
	if explore {
		allCategoryURL = "/explore/" + exploreAllCategoryRouteToken
	}
	categoryItems = append(categoryItems, view.HomeCategoryViewData{
		CategoryID: "",
		Name:       "全部",
		IsDefault:  false,
		IsActive:   (!explore) || isExploreAll,
		URL:        allCategoryURL,
	})
	for _, item := range record.Categories {
		normalizedCategoryID := strings.TrimSpace(item.CategoryID)
		categoryItems = append(categoryItems, view.HomeCategoryViewData{
			CategoryID: normalizedCategoryID,
			Name:       strings.TrimSpace(item.Name),
			IsDefault:  item.IsDefault,
			IsActive:   explore && normalizedCategoryID == strings.TrimSpace(record.ActiveCategoryID),
			URL:        "/explore/" + normalizedCategoryID,
		})
	}

	spaceItems := make([]view.HomeSpaceViewData, 0, len(record.Spaces))
	for _, item := range record.Spaces {
		spaceItems = append(spaceItems, view.HomeSpaceViewData{
			SpaceID:     strings.TrimSpace(item.SpaceID),
			Name:        strings.TrimSpace(item.Name),
			Description: strings.TrimSpace(item.Description),
			CoverURL:    strings.TrimSpace(item.CoverURL),
		})
	}
	loginURL := h.buildWebAuthEntryURL(c, "/login")
	registerURL := h.buildWebAuthEntryURL(c, "/register")
	activeCategoryName := strings.TrimSpace(record.ActiveCategory)
	if isExploreAll {
		activeCategoryName = "全部"
	}

	htmlBytes, err := view.RenderHomePage(view.HomePageViewData{
		Title:                title,
		Description:          description,
		CanonicalURL:         view.NormalizeCanonicalPath(canonicalURL),
		SiteName:             "PlainDoc",
		IsExplore:            explore,
		IsAuthenticated:      viewerIdentity.Authenticated,
		CurrentUserName:      viewerIdentity.Name,
		CurrentUserAvatar:    buildUserAvatarText(viewerIdentity.Name, viewerIdentity.Email),
		CurrentUserAvatarURL: strings.TrimSpace(viewerIdentity.AvatarURL),
		LoginURL:             loginURL,
		RegisterURL:          registerURL,
		AdminURL:             "/admin",
		LogoutURL:            "/logout",
		ActiveCategoryID:     strings.TrimSpace(record.ActiveCategoryID),
		ActiveCategoryName:   activeCategoryName,
		Categories:           categoryItems,
		Spaces:               spaceItems,
		Page:                 record.Pagination.Page,
		PageSize:             record.Pagination.PageSize,
		Total:                record.Pagination.Total,
	})
	if err != nil {
		renderHomeFallback(c)
		return
	}

	c.Data(http.StatusOK, "text/html; charset=utf-8", htmlBytes)
}

func (h *homeHandler) resolveOptionalViewerIdentity(c *gin.Context) homeViewerIdentity {
	if h == nil || h.authService == nil {
		return homeViewerIdentity{}
	}

	rawToken, ok := optionalAccessTokenFromRequest(c)
	if !ok {
		return homeViewerIdentity{}
	}

	session, err := h.authService.Me(c.Request.Context(), rawToken)
	if err != nil {
		return homeViewerIdentity{}
	}

	name := strings.TrimSpace(session.User.Name)
	email := strings.TrimSpace(session.User.Email)
	if name == "" {
		name = email
	}
	if name == "" {
		name = "用户"
	}
	return homeViewerIdentity{
		UserID:        strings.TrimSpace(session.User.ID),
		Name:          name,
		Email:         email,
		AvatarURL:     strings.TrimSpace(session.User.AvatarURL),
		Authenticated: strings.TrimSpace(session.User.ID) != "",
	}
}

func parsePositiveInt(raw string, fallback int) int {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func appendVaryHeader(c *gin.Context, token string) {
	existing := strings.TrimSpace(c.Writer.Header().Get("Vary"))
	if existing == "" {
		c.Header("Vary", token)
		return
	}

	parts := strings.Split(existing, ",")
	for _, item := range parts {
		if strings.EqualFold(strings.TrimSpace(item), token) {
			return
		}
	}

	c.Header("Vary", existing+", "+token)
}

func optionalAccessTokenFromRequest(c *gin.Context) (string, bool) {
	rawToken, ok := bearerTokenFromRequest(c)
	if ok {
		return rawToken, true
	}

	tokenFromCookie, err := c.Cookie("accessToken")
	if err != nil {
		return "", false
	}
	trimmed := strings.TrimSpace(tokenFromCookie)
	if trimmed == "" {
		return "", false
	}
	return trimmed, true
}

func buildUserAvatarText(name string, email string) string {
	display := strings.TrimSpace(name)
	if display == "" {
		display = strings.TrimSpace(email)
	}
	if display == "" {
		return "U"
	}

	r, _ := utf8.DecodeRuneInString(display)
	if r == utf8.RuneError {
		return "U"
	}
	return strings.ToUpper(string(r))
}

func normalizeWebOrigin(raw string) string {
	return strings.TrimRight(strings.TrimSpace(raw), "/")
}

func normalizeExploreCategoryID(raw string) (string, bool) {
	normalized := strings.TrimSpace(raw)
	if strings.EqualFold(normalized, exploreAllCategoryRouteToken) {
		return "", true
	}
	return normalized, false
}

func (h *homeHandler) buildWebAuthEntryURL(c *gin.Context, targetPath string) string {
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

func buildRequestAbsoluteURL(c *gin.Context) string {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return ""
	}

	scheme := firstForwardedValue(c.GetHeader("X-Forwarded-Proto"))
	if scheme == "" {
		if c.Request.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}

	host := firstForwardedValue(c.GetHeader("X-Forwarded-Host"))
	if host == "" {
		host = strings.TrimSpace(c.Request.Host)
	}
	if host == "" {
		return ""
	}

	requestURI := c.Request.URL.RequestURI()
	if strings.TrimSpace(requestURI) == "" {
		requestURI = "/"
	}
	return scheme + "://" + host + requestURI
}

func firstForwardedValue(raw string) string {
	for _, item := range strings.Split(raw, ",") {
		value := strings.TrimSpace(item)
		if value != "" {
			return value
		}
	}
	return ""
}

func renderHomeFallback(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store, max-age=0")
	appendVaryHeader(c, "Authorization")
	appendVaryHeader(c, "Cookie")
	pageTitle := "空间探索"
	seoTitle := pageTitle + " - " + seoTitleSuffix
	c.Data(
		http.StatusOK,
		"text/html; charset=utf-8",
		[]byte("<!doctype html><html lang=\"zh-CN\"><head><meta charset=\"utf-8\"><title>"+seoTitle+"</title><link rel=\"preload\" href=\"/assets/google-sans-code-latin-400-normal.woff2\" as=\"font\" type=\"font/woff2\" crossorigin><link rel=\"stylesheet\" href=\"/assets/font-google-sans-code.css?v=20260224\"><style>body{margin:0;padding:24px;font-family:\"Google Sans Code\",\"PingFang SC\",\"Microsoft YaHei\",\"Helvetica Neue\",Arial,sans-serif;background:#f8fafc;color:#1f2937}main{max-width:720px;margin:40px auto;background:#fff;border:1px solid #dbe2ea;border-radius:12px;padding:18px 20px}</style></head><body><main><h1>空间探索</h1><p>页面正在初始化，请稍后刷新。</p></main></body></html>"),
	)
}
