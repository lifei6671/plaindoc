package handler

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
)

const (
	documentShareAccessCookieNamePrefix = "pd_share_access_"
)

type documentSharePageHandler struct {
	documentShareService *service.DocumentShareService
	accessTokenService   *service.DocumentShareAccessTokenService
	readerRenderer       *readerPageHandler
}

type verifyDocumentShareRequest struct {
	Password string `json:"password"`
}

type documentShareAttachmentAccessLinkResponse struct {
	URL            string `json:"url"`
	Purpose        string `json:"purpose"`
	PreviewKind    string `json:"previewKind"`
	PreviewEnabled bool   `json:"previewEnabled"`
}

// NewDocumentSharePageHandler 创建分享阅读页处理器。
func NewDocumentSharePageHandler(
	documentShareService *service.DocumentShareService,
	accessTokenService *service.DocumentShareAccessTokenService,
	readerRenderer *readerPageHandler,
) *documentSharePageHandler {
	return &documentSharePageHandler{
		documentShareService: documentShareService,
		accessTokenService:   accessTokenService,
		readerRenderer:       readerRenderer,
	}
}

// Page 渲染文档分享页。
func (h *documentSharePageHandler) Page(c *gin.Context) {
	if h == nil || h.documentShareService == nil || h.readerRenderer == nil {
		response.InternalError(c)
		return
	}

	spaceID := strings.TrimSpace(c.Param("spaceId"))
	docKey := strings.TrimSpace(c.Param("docKey"))
	if spaceID == "" {
		response.DocumentShareErrSpaceIDRequired.Write(c)
		return
	}
	if docKey == "" {
		response.DocumentShareErrShareNotFound.Write(c)
		return
	}

	resolvedShare, err := h.documentShareService.ResolveActiveShareByRouteKey(c.Request.Context(), spaceID, docKey)
	if err != nil {
		setRequestErrmsg(c, err, "解析分享访问配置失败")
		h.renderShareFriendlyError(c, http.StatusNotFound, "页面不存在", "分享不存在、已取消或已过期。")
		return
	}

	canonicalDocKey := resolveDocumentShareCanonicalDocKey(resolvedShare)
	if canonicalDocKey != "" && canonicalDocKey != docKey {
		c.Redirect(http.StatusSeeOther, buildDocumentSharePagePath(spaceID, canonicalDocKey))
		return
	}
	if resolvedShare.Mode == models.DocumentShareModePassword {
		isAuthorized := h.isShareAccessAuthorized(c, resolvedShare)
		if !isAuthorized {
			h.renderSharePasswordGate(c, resolvedShare)
			return
		}
	}

	pageResult, err := h.documentShareService.BuildReaderPageByRouteKey(c.Request.Context(), spaceID, canonicalDocKey)
	if err != nil {
		setRequestErrmsg(c, err, "构建分享阅读页数据失败")
		if errors.Is(err, service.ErrDocumentShareNotFound) {
			h.renderShareFriendlyError(c, http.StatusNotFound, "页面不存在", "分享不存在、已取消或已过期。")
			return
		}
		h.renderShareFriendlyError(c, http.StatusInternalServerError, "加载失败", "分享页面暂时不可用，请稍后再试。")
		return
	}

	shareDocKey := resolveDocumentShareCanonicalDocKey(pageResult.Share)
	payload := readerPagePayload{
		Space:         pageResult.Page.Space,
		Document:      pageResult.Page.Document,
		Attachments:   pageResult.Page.Attachments,
		Tree:          pageResult.Page.Tree,
		ActiveDocID:   pageResult.Page.ActiveDocID,
		RequestOrigin: resolveRequestOrigin(c),
		Viewer:        h.readerRenderer.resolveOptionalViewerIdentity(c),
		Share: &readerPageShareState{
			Enabled:            true,
			ShareID:            strings.TrimSpace(pageResult.Share.ShareID),
			SpaceID:            strings.TrimSpace(spaceID),
			DocumentRouteKey:   strings.TrimSpace(shareDocKey),
			BasePath:           buildDocumentShareBasePath(spaceID),
			AttachmentBasePath: buildDocumentShareAttachmentBasePath(spaceID, shareDocKey),
		},
	}
	h.readerRenderer.renderReaderPayload(
		c,
		http.StatusOK,
		spaceID,
		strings.TrimSpace(pageResult.Share.DocumentID),
		payload,
		false,
	)
}

// GetOnlyOfficeViewConfig 返回分享页 Office 文档只读配置。
func (h *documentSharePageHandler) GetOnlyOfficeViewConfig(c *gin.Context) {
	if h == nil || h.documentShareService == nil || h.readerRenderer == nil ||
		h.readerRenderer.onlyOfficeConfigService == nil || h.readerRenderer.onlyOfficeTokenService == nil {
		response.InternalError(c)
		return
	}

	spaceID := strings.TrimSpace(c.Param("spaceId"))
	docKey := strings.TrimSpace(c.Param("docKey"))
	if spaceID == "" {
		response.DocumentShareErrSpaceIDRequired.Write(c)
		return
	}
	if docKey == "" {
		response.DocumentShareErrShareNotFound.Write(c)
		return
	}

	appendVaryHeader(c, "Cookie")
	appendVaryHeader(c, "Authorization")
	c.Header("Cache-Control", "private, no-store, max-age=0")

	resolvedShare, err := h.ensureShareAttachmentAccess(c, spaceID, docKey)
	if err != nil {
		setRequestErrmsg(c, err, "校验分享 Office 阅读权限失败")
		if !writeMappedDocumentShareError(c, err) {
			response.InternalError(c)
		}
		return
	}

	canonicalDocKey := resolveDocumentShareCanonicalDocKey(resolvedShare)
	pageResult, err := h.documentShareService.BuildReaderPageByRouteKey(c.Request.Context(), spaceID, canonicalDocKey)
	if err != nil {
		setRequestErrmsg(c, err, "构建分享 Office 阅读页数据失败")
		if errors.Is(err, service.ErrDocumentShareNotFound) {
			response.DocumentShareErrShareNotFound.Write(c)
			return
		}
		response.InternalError(c)
		return
	}

	viewer := h.readerRenderer.resolveOptionalViewerIdentity(c)
	configPayload, err := buildOnlyOfficeViewConfig(
		c.Request.Context(),
		h.readerRenderer.onlyOfficeConfigService,
		h.readerRenderer.onlyOfficeTokenService,
		pageResult.Page.Document,
		viewer.UserID,
		viewer.Name,
	)
	if err != nil {
		setRequestErrmsg(c, err, "生成分享 Office 只读配置失败")
		switch {
		case errors.Is(err, errOnlyOfficeViewConfigNotOfficeDocument):
			response.Error(c, http.StatusBadRequest, response.CodeInvalidOperation, "当前分享文档不是 Office 文档")
		case errors.Is(err, errOnlyOfficeViewConfigDisabled):
			response.Error(c, http.StatusBadRequest, response.CodeInvalidRequest, "ONLYOFFICE 阅读能力未启用")
		default:
			response.InternalError(c)
		}
		return
	}

	response.JSON(c, http.StatusOK, configPayload)
}

// Verify 校验密码分享访问密码并下发免密 Cookie。
func (h *documentSharePageHandler) Verify(c *gin.Context) {
	if h == nil || h.documentShareService == nil || h.accessTokenService == nil {
		response.InternalError(c)
		return
	}

	spaceID := strings.TrimSpace(c.Param("spaceId"))
	docKey := strings.TrimSpace(c.Param("docKey"))
	if spaceID == "" {
		response.DocumentShareErrSpaceIDRequired.Write(c)
		return
	}
	if docKey == "" {
		response.DocumentShareErrShareNotFound.Write(c)
		return
	}

	var req verifyDocumentShareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.DocumentShareErrPasswordRequired.Write(c)
		return
	}
	password := strings.TrimSpace(req.Password)
	if password == "" {
		response.DocumentShareErrPasswordRequired.Write(c)
		return
	}

	resolvedShare, err := h.documentShareService.VerifyPassword(c.Request.Context(), spaceID, docKey, password)
	if err != nil {
		setRequestErrmsg(c, err, "校验分享访问密码失败")
		if !writeMappedDocumentShareError(c, err) {
			response.InternalError(c)
		}
		return
	}

	ttl := time.Duration(0)
	if resolvedShare.ExpiresAt != nil {
		remaining := time.Until(resolvedShare.ExpiresAt.UTC())
		if remaining <= 0 {
			response.DocumentShareErrShareExpired.Write(c)
			return
		}
		ttl = remaining
	}

	token, tokenExpiresAt, err := h.accessTokenService.Issue(service.IssueDocumentShareAccessTokenInput{
		ShareID:       strings.TrimSpace(resolvedShare.ShareID),
		AccessVersion: resolvedShare.AccessVersion,
		TTL:           ttl,
	})
	if err != nil {
		setRequestErrmsg(c, err, "签发分享免密令牌失败")
		response.InternalError(c)
		return
	}

	h.writeShareAccessCookie(c, resolvedShare, token, tokenExpiresAt)
	response.JSON(c, http.StatusOK, map[string]any{
		"verified":    true,
		"redirectUrl": buildDocumentSharePagePath(spaceID, resolveDocumentShareCanonicalDocKey(resolvedShare)),
		"expiresAt":   tokenExpiresAt.UTC().Format(time.RFC3339Nano),
	})
}

// CreateAttachmentAccessLink 生成分享态附件访问链接。
func (h *documentSharePageHandler) CreateAttachmentAccessLink(c *gin.Context) {
	if h == nil || h.documentShareService == nil {
		response.InternalError(c)
		return
	}

	spaceID := strings.TrimSpace(c.Param("spaceId"))
	docKey := strings.TrimSpace(c.Param("docKey"))
	attachmentID := strings.TrimSpace(c.Param("attachmentId"))
	if spaceID == "" {
		response.DocumentShareErrSpaceIDRequired.Write(c)
		return
	}
	if docKey == "" {
		response.DocumentShareErrShareNotFound.Write(c)
		return
	}
	if attachmentID == "" {
		response.DocumentShareErrAttachmentIDRequired.Write(c)
		return
	}

	resolvedShare, err := h.ensureShareAttachmentAccess(c, spaceID, docKey)
	if err != nil {
		setRequestErrmsg(c, err, "校验分享附件访问权限失败")
		if !writeMappedDocumentShareError(c, err) {
			response.InternalError(c)
		}
		return
	}

	purpose, err := parseDocumentShareAttachmentPurpose(c.Query("purpose"))
	if err != nil {
		response.DocumentShareErrAttachmentPreviewUnsupported.Write(c)
		return
	}

	accessLink, err := h.documentShareService.BuildAttachmentAccessURLByRouteKey(
		c.Request.Context(),
		spaceID,
		resolveDocumentShareCanonicalDocKey(resolvedShare),
		attachmentID,
		purpose,
	)
	if err != nil {
		setRequestErrmsg(c, err, "生成分享附件访问链接失败")
		if errors.Is(err, service.ErrDocumentSharePreviewDenied) {
			response.DocumentShareErrAttachmentPreviewUnsupported.Write(c)
			return
		}
		if !writeMappedDocumentShareError(c, err) {
			response.InternalError(c)
		}
		return
	}

	response.JSON(c, http.StatusOK, documentShareAttachmentAccessLinkResponse{
		URL:            resolveDocumentShareAttachmentAccessURL(spaceID, resolveDocumentShareCanonicalDocKey(resolvedShare), attachmentID, purpose, accessLink),
		Purpose:        string(accessLink.Purpose),
		PreviewKind:    strings.TrimSpace(accessLink.PreviewKind),
		PreviewEnabled: accessLink.PreviewEnabled,
	})
}

// AttachmentPreviewPage 渲染分享态附件在线预览页。
func (h *documentSharePageHandler) AttachmentPreviewPage(c *gin.Context) {
	if c == nil {
		return
	}
	if h == nil || h.documentShareService == nil {
		setRequestErrmsgText(c, "分享附件预览页初始化失败: document share service is nil")
		c.Data(http.StatusInternalServerError, "text/html; charset=utf-8", []byte(documentAttachmentPreviewPageUnavailableHTML))
		return
	}

	spaceID := strings.TrimSpace(c.Param("spaceId"))
	docKey := strings.TrimSpace(c.Param("docKey"))
	attachmentID := strings.TrimSpace(c.Param("attachmentId"))
	if spaceID == "" {
		response.DocumentShareErrSpaceIDRequired.Write(c)
		return
	}
	if docKey == "" {
		response.DocumentShareErrShareNotFound.Write(c)
		return
	}
	if attachmentID == "" {
		response.DocumentShareErrAttachmentIDRequired.Write(c)
		return
	}

	resolvedShare, err := h.ensureShareAttachmentAccess(c, spaceID, docKey)
	if err != nil {
		setRequestErrmsg(c, err, "校验分享附件预览访问权限失败")
		if !writeMappedDocumentShareError(c, err) {
			response.InternalError(c)
		}
		return
	}

	canonicalDocKey := resolveDocumentShareCanonicalDocKey(resolvedShare)
	if canonicalDocKey != "" && canonicalDocKey != docKey {
		c.Redirect(http.StatusSeeOther, buildDocumentShareAttachmentPreviewPagePath(spaceID, canonicalDocKey, attachmentID))
		return
	}

	appendVaryHeader(c, "Cookie")
	appendVaryHeader(c, "Authorization")
	c.Header("Cache-Control", "private, no-store, max-age=0")

	pageHTML, err := buildDocumentAttachmentPreviewPageHTML(documentAttachmentPreviewPageData{
		DocumentID:     strings.TrimSpace(resolvedShare.DocumentID),
		AttachmentID:   attachmentID,
		AccessLinkPath: buildDocumentShareAttachmentAccessLinkPath(spaceID, canonicalDocKey, attachmentID),
	})
	if err != nil {
		setRequestErrmsg(c, err, "渲染分享附件预览页失败")
		c.Data(http.StatusInternalServerError, "text/html; charset=utf-8", []byte(documentAttachmentPreviewPageUnavailableHTML))
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(pageHTML))
}

// RedirectAttachmentDownload 提供分享态附件下载跳转。
func (h *documentSharePageHandler) RedirectAttachmentDownload(c *gin.Context) {
	if h == nil || h.documentShareService == nil {
		response.InternalError(c)
		return
	}

	spaceID := strings.TrimSpace(c.Param("spaceId"))
	docKey := strings.TrimSpace(c.Param("docKey"))
	attachmentID := strings.TrimSpace(c.Param("attachmentId"))
	if spaceID == "" {
		response.DocumentShareErrSpaceIDRequired.Write(c)
		return
	}
	if docKey == "" {
		response.DocumentShareErrShareNotFound.Write(c)
		return
	}
	if attachmentID == "" {
		response.DocumentShareErrAttachmentIDRequired.Write(c)
		return
	}

	resolvedShare, err := h.ensureShareAttachmentAccess(c, spaceID, docKey)
	if err != nil {
		setRequestErrmsg(c, err, "校验分享附件访问权限失败")
		if !writeMappedDocumentShareError(c, err) {
			response.InternalError(c)
		}
		return
	}

	accessLink, err := h.documentShareService.BuildAttachmentAccessURLByRouteKey(
		c.Request.Context(),
		spaceID,
		resolveDocumentShareCanonicalDocKey(resolvedShare),
		attachmentID,
		service.DocumentShareAttachmentPurposeDownload,
	)
	if err != nil {
		setRequestErrmsg(c, err, "生成分享附件下载链接失败")
		if !writeMappedDocumentShareError(c, err) {
			response.InternalError(c)
		}
		return
	}
	if strings.EqualFold(
		strings.TrimSpace(accessLink.StorageProvider),
		string(service.ImageHostingProviderLocal),
	) {
		serveDocumentShareLocalAttachmentDownload(c, accessLink)
		return
	}
	if strings.TrimSpace(accessLink.URL) == "" {
		response.DocumentShareErrShareNotFound.Write(c)
		return
	}
	c.Redirect(http.StatusTemporaryRedirect, accessLink.URL)
}

func (h *documentSharePageHandler) ensureShareAttachmentAccess(
	c *gin.Context,
	spaceID string,
	docKey string,
) (service.ResolveDocumentShareResult, error) {
	resolvedShare, err := h.documentShareService.ResolveActiveShareByRouteKey(c.Request.Context(), spaceID, docKey)
	if err != nil {
		return service.ResolveDocumentShareResult{}, err
	}
	if resolvedShare.Mode == models.DocumentShareModePassword && !h.isShareAccessAuthorized(c, resolvedShare) {
		return service.ResolveDocumentShareResult{}, service.ErrDocumentShareAccessDenied
	}
	return resolvedShare, nil
}

func (h *documentSharePageHandler) isShareAccessAuthorized(
	c *gin.Context,
	share service.ResolveDocumentShareResult,
) bool {
	if h == nil || h.accessTokenService == nil {
		return false
	}
	if share.Mode != models.DocumentShareModePassword {
		return true
	}
	cookieName := buildDocumentShareAccessCookieName(share.ShareID)
	rawToken, err := c.Cookie(cookieName)
	if err != nil {
		return false
	}
	claims, err := h.accessTokenService.Parse(rawToken)
	if err != nil {
		return false
	}
	return strings.TrimSpace(claims.ShareID) == strings.TrimSpace(share.ShareID) &&
		claims.AccessVersion == share.AccessVersion
}

func (h *documentSharePageHandler) writeShareAccessCookie(
	c *gin.Context,
	share service.ResolveDocumentShareResult,
	token string,
	expiresAt time.Time,
) {
	c.SetSameSite(http.SameSiteLaxMode)
	secure := requestUsesHTTPS(c)
	maxAge := int(time.Until(expiresAt).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}
	c.SetCookie(
		buildDocumentShareAccessCookieName(share.ShareID),
		token,
		maxAge,
		"/",
		"",
		secure,
		true,
	)
}

func (h *documentSharePageHandler) renderShareFriendlyError(
	c *gin.Context,
	statusCode int,
	title string,
	description string,
) {
	appendVaryHeader(c, "Cookie")
	appendVaryHeader(c, "Authorization")
	c.Header("Cache-Control", "private, no-store, max-age=0")
	c.Data(statusCode, "text/html; charset=utf-8", []byte(buildReaderErrorHTML(statusCode, title, description)))
}

func (h *documentSharePageHandler) renderSharePasswordGate(
	c *gin.Context,
	share service.ResolveDocumentShareResult,
) {
	appendVaryHeader(c, "Cookie")
	appendVaryHeader(c, "Authorization")
	c.Header("Cache-Control", "private, no-store, max-age=0")
	c.Data(
		http.StatusOK,
		"text/html; charset=utf-8",
		[]byte(buildDocumentSharePasswordGateHTML(
			strings.TrimSpace(share.SpaceID),
			resolveDocumentShareCanonicalDocKey(share),
			strings.TrimSpace(share.PasswordHint),
		)),
	)
}

func buildDocumentShareAccessCookieName(shareID string) string {
	normalizedShareID := strings.ToLower(strings.TrimSpace(shareID))
	if normalizedShareID == "" {
		return documentShareAccessCookieNamePrefix + "default"
	}
	suffix := normalizedShareID
	if len(suffix) > 12 {
		suffix = suffix[len(suffix)-12:]
	}
	return documentShareAccessCookieNamePrefix + suffix
}

func resolveDocumentShareCanonicalDocKey(share service.ResolveDocumentShareResult) string {
	docKey := strings.TrimSpace(share.DocumentRouteKey)
	if docKey != "" {
		return docKey
	}
	return strings.TrimSpace(share.DocumentID)
}

func buildDocumentShareBasePath(spaceID string) string {
	return "/s/" + url.PathEscape(strings.TrimSpace(spaceID))
}

func buildDocumentSharePagePath(spaceID string, docKey string) string {
	normalizedDocKey := strings.TrimSpace(docKey)
	if normalizedDocKey == "" {
		normalizedDocKey = "unknown"
	}
	return buildDocumentShareBasePath(spaceID) + "/" + url.PathEscape(normalizedDocKey)
}

func buildDocumentShareAttachmentBasePath(spaceID string, docKey string) string {
	return "/api/shares/" + url.PathEscape(strings.TrimSpace(spaceID)) + "/" + url.PathEscape(strings.TrimSpace(docKey)) + "/attachments"
}

func buildDocumentShareAttachmentAccessLinkPath(spaceID string, docKey string, attachmentID string) string {
	return buildDocumentShareAttachmentBasePath(spaceID, docKey) + "/" + url.PathEscape(strings.TrimSpace(attachmentID)) + "/access-link"
}

func buildDocumentShareAttachmentDownloadPath(spaceID string, docKey string, attachmentID string) string {
	return buildDocumentShareAttachmentBasePath(spaceID, docKey) + "/" + url.PathEscape(strings.TrimSpace(attachmentID)) + "/download"
}

func buildDocumentShareAttachmentPreviewPagePath(spaceID string, docKey string, attachmentID string) string {
	return "/preview/shares/" +
		url.PathEscape(strings.TrimSpace(spaceID)) +
		"/" +
		url.PathEscape(strings.TrimSpace(docKey)) +
		"/attachments/" +
		url.PathEscape(strings.TrimSpace(attachmentID))
}

func resolveDocumentShareAttachmentAccessURL(
	spaceID string,
	docKey string,
	attachmentID string,
	purpose service.DocumentShareAttachmentPurpose,
	accessLink service.DocumentShareAttachmentAccessResult,
) string {
	if purpose == service.DocumentShareAttachmentPurposeDownload && strings.EqualFold(
		strings.TrimSpace(accessLink.StorageProvider),
		string(service.ImageHostingProviderLocal),
	) {
		return buildDocumentShareAttachmentDownloadPath(spaceID, docKey, attachmentID)
	}
	return strings.TrimSpace(accessLink.URL)
}

func serveDocumentShareLocalAttachmentDownload(
	c *gin.Context,
	accessLink service.DocumentShareAttachmentAccessResult,
) {
	objectKey := strings.TrimSpace(accessLink.ObjectKey)
	targetPath, err := resolveDocumentShareLocalAttachmentTargetPath(objectKey)
	if err != nil {
		setRequestErrmsg(c, err, "解析分享本地附件目标路径失败")
		response.DocumentShareErrShareNotFound.Write(c)
		return
	}
	fileInfo, err := os.Stat(targetPath)
	if err != nil {
		setRequestErrmsg(c, err, "读取分享本地附件文件失败")
		if errors.Is(err, os.ErrNotExist) {
			response.DocumentShareErrShareNotFound.Write(c)
			return
		}
		response.InternalError(c)
		return
	}
	if fileInfo.IsDir() {
		response.DocumentShareErrShareNotFound.Write(c)
		return
	}

	fileName := strings.TrimSpace(accessLink.FileName)
	if fileName == "" {
		fileName = strings.TrimSpace(path.Base(objectKey))
	}
	if fileName == "" {
		fileName = "attachment"
	}
	mimeType := strings.TrimSpace(accessLink.MimeType)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	c.Header(
		"Content-Disposition",
		fmt.Sprintf("attachment; filename*=UTF-8''%s", url.PathEscape(fileName)),
	)
	c.Header("Content-Type", mimeType)
	c.Header("Cache-Control", "private, no-store, max-age=0")
	c.File(targetPath)
}

func resolveDocumentShareLocalAttachmentTargetPath(objectKey string) (string, error) {
	normalizedObjectKey := strings.TrimSpace(strings.TrimPrefix(objectKey, "/"))
	if normalizedObjectKey == "" {
		return "", errors.New("object key is empty")
	}
	cleanObjectKey := path.Clean(normalizedObjectKey)
	if cleanObjectKey == "." || cleanObjectKey == "/" || strings.HasPrefix(cleanObjectKey, "../") {
		return "", errors.New("object key is invalid")
	}
	localRootDir := defaultLocalImageStorageRoot
	targetPath := filepath.Join(localRootDir, filepath.FromSlash(cleanObjectKey))
	targetAbsPath, err := filepath.Abs(targetPath)
	if err != nil {
		return "", err
	}
	rootAbsPath, err := filepath.Abs(localRootDir)
	if err != nil {
		return "", err
	}
	if !isPathWithinRoot(rootAbsPath, targetAbsPath) {
		return "", errors.New("object key is out of root")
	}
	return targetAbsPath, nil
}

func parseDocumentShareAttachmentPurpose(raw string) (service.DocumentShareAttachmentPurpose, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "download":
		return service.DocumentShareAttachmentPurposeDownload, nil
	case "preview":
		return service.DocumentShareAttachmentPurposePreview, nil
	default:
		return "", errors.New("invalid document share attachment purpose")
	}
}

func buildDocumentSharePasswordGateHTML(spaceID string, docKey string, passwordHint string) string {
	pageTitle := template.HTMLEscapeString("访问分享文档")
	hint := template.HTMLEscapeString(strings.TrimSpace(passwordHint))
	canonicalPath := template.HTMLEscapeString(buildDocumentSharePagePath(spaceID, docKey))
	verifyPath := template.HTMLEscapeString(canonicalPath + "/verify")
	hintBlock := ""
	if hint != "" {
		hintBlock = fmt.Sprintf(`<p class="share-password-page__hint"><span>密码提示：</span>%s</p>`, hint)
	}
	return fmt.Sprintf(`<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>%s</title>
    <style>
      :root { color-scheme: light; }
      body { margin: 0; min-height: 100dvh; display: flex; align-items: center; justify-content: center; padding: 16px; background: #f3f4f6; color: #111827; font-family: "Google Sans Code","PingFang SC","Microsoft YaHei",sans-serif; }
      .share-password-page { width: min(100%%, 420px); background: #fff; border: 1px solid #e5e7eb; border-radius: 14px; padding: 20px; box-shadow: 0 20px 44px rgba(15, 23, 42, 0.14); }
      .share-password-page h1 { margin: 0 0 8px; font-size: 20px; line-height: 1.35; }
      .share-password-page p { margin: 0; color: #4b5563; font-size: 14px; line-height: 1.6; }
      .share-password-page__hint { margin-top: 10px; padding: 8px 10px; border-radius: 10px; background: #f8fafc; border: 1px solid #e2e8f0; font-size: 13px; color: #334155; }
      .share-password-page__hint span { color: #64748b; margin-right: 4px; }
      .share-password-page__form { margin-top: 16px; display: grid; gap: 10px; }
      .share-password-page input { width: 100%%; height: 40px; border-radius: 10px; border: 1px solid #cbd5e1; padding: 0 12px; font-size: 14px; box-sizing: border-box; }
      .share-password-page input:focus { outline: none; border-color: #3b82f6; box-shadow: 0 0 0 3px rgba(59,130,246,0.15); }
      .share-password-page button { height: 40px; border: 0; border-radius: 10px; background: #2563eb; color: #fff; font-size: 14px; font-weight: 600; cursor: pointer; }
      .share-password-page button[disabled] { opacity: 0.65; cursor: not-allowed; }
      .share-password-page__error { min-height: 20px; font-size: 13px; color: #b91c1c; }
    </style>
  </head>
  <body>
    <section class="share-password-page" aria-label="分享密码验证">
      <h1>请输入访问密码</h1>
      <p>该文档已开启密码保护，验证后可在当前设备免密访问。</p>
      %s
      <form id="share-password-form" class="share-password-page__form" novalidate>
        <input id="share-password-input" type="password" name="password" autocomplete="current-password" placeholder="请输入访问密码" required />
        <button id="share-password-submit" type="submit">验证并访问</button>
        <div id="share-password-error" class="share-password-page__error" aria-live="polite"></div>
      </form>
    </section>
    <script>
      (function() {
        var form = document.getElementById("share-password-form");
        var input = document.getElementById("share-password-input");
        var submit = document.getElementById("share-password-submit");
        var error = document.getElementById("share-password-error");
        if (!(form instanceof HTMLFormElement) || !(input instanceof HTMLInputElement) || !(submit instanceof HTMLButtonElement) || !(error instanceof HTMLElement)) {
          return;
        }
        form.addEventListener("submit", async function(event) {
          event.preventDefault();
          var password = (input.value || "").trim();
          if (!password) {
            error.textContent = "请输入访问密码";
            input.focus();
            return;
          }
          error.textContent = "";
          submit.disabled = true;
          submit.textContent = "验证中...";
          try {
            var response = await fetch(%q, {
              method: "POST",
              credentials: "include",
              headers: {
                "Content-Type": "application/json",
                "Accept": "application/json",
                "X-Requested-With": "plaindoc-share-password-gate"
              },
              body: JSON.stringify({ password: password })
            });
            var payload = null;
            try { payload = await response.json(); } catch (_) {}
            var success = payload && typeof payload === "object" && payload.code === 0;
            if (!response.ok || !success) {
              var message = payload && typeof payload === "object" && typeof payload.message === "string" ? payload.message.trim() : "";
              error.textContent = message || "密码验证失败，请重试。";
              return;
            }
            var redirectURL = payload && typeof payload === "object" && payload.data && typeof payload.data.redirectUrl === "string"
              ? payload.data.redirectUrl.trim()
              : "";
            window.location.assign(redirectURL || %q);
          } catch (_) {
            error.textContent = "网络异常，请稍后重试。";
          } finally {
            submit.disabled = false;
            submit.textContent = "验证并访问";
          }
        });
      })();
    </script>
  </body>
</html>`, pageTitle, hintBlock, verifyPath, canonicalPath)
}
