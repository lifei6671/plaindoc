package handler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"
)

const (
	onlyOfficeAccessTokenQueryKey = "accessToken"
)

type workspaceOnlyOfficeEditConfigResponse struct {
	DocumentServerURL string         `json:"documentServerUrl"`
	Config            map[string]any `json:"config"`
}

type onlyOfficeCallbackRequest struct {
	Status int      `json:"status"`
	URL    string   `json:"url"`
	Key    string   `json:"key"`
	Users  []string `json:"users"`
}

// GetOnlyOfficeEditConfig 返回工作区 Office 文档编辑配置。
func (h *workspaceHandler) GetOnlyOfficeEditConfig(c *gin.Context) {
	session, ok := h.requireActorSession(c)
	if !ok {
		return
	}
	if h == nil || h.workspaceRepo == nil || h.onlyOfficeConfigService == nil || h.onlyOfficeTokenService == nil {
		response.InternalError(c)
		return
	}

	documentID := strings.TrimSpace(c.Param("docId"))
	if documentID == "" {
		response.WorkspaceErrDocumentIDRequired.Write(c)
		return
	}

	document, err := h.workspaceRepo.GetDocumentByDocumentID(c.Request.Context(), documentID)
	if err != nil {
		if errorsIsRecordNotFound(err) {
			response.WorkspaceErrDocumentNotFound.Write(c)
			return
		}
		setRequestErrmsg(c, err, "查询 ONLYOFFICE 文档失败")
		response.InternalError(c)
		return
	}
	if _, err := h.ensureSpaceWritable(c.Request.Context(), strings.TrimSpace(document.SpaceID), strings.TrimSpace(session.User.ID)); err != nil {
		setRequestErrmsg(c, err, "校验 ONLYOFFICE 文档写权限失败")
		switch {
		case errorsIsSpaceNotFound(err):
			response.WorkspaceErrSpaceNotFound.Write(c)
		case errorsIsSpaceAccessDenied(err):
			response.WorkspaceErrInsufficientSpacePermission.Write(c)
		default:
			response.InternalError(c)
		}
		return
	}
	if !models.IsOfficeDocumentFormat(document.Format) {
		response.WorkspaceErrMarkdownOnlyOperation.Write(c)
		return
	}
	if strings.TrimSpace(derefOptionalString(document.SourceBlobID)) == "" {
		setRequestErrmsgText(c, "office document source blob is empty")
		response.InternalError(c)
		return
	}

	onlyOfficeConfig, err := h.onlyOfficeConfigService.GetConfig(c.Request.Context())
	if err != nil {
		setRequestErrmsg(c, err, "读取 ONLYOFFICE 配置失败")
		response.InternalError(c)
		return
	}
	if !onlyOfficeConfig.Enabled {
		response.WorkspaceErrOnlyOfficeDisabled.Write(c)
		return
	}
	if strings.TrimSpace(onlyOfficeConfig.DocumentServerURL) == "" ||
		strings.TrimSpace(onlyOfficeConfig.CallbackPublicBaseURL) == "" {
		setRequestErrmsgText(c, "onlyoffice config is enabled but required urls are empty")
		response.InternalError(c)
		return
	}

	contentVersion := normalizeWorkspaceContentVersion(document.ContentVersion, document.Version)
	actorUserID := strings.TrimSpace(session.User.ID)
	sourceToken, _, err := h.onlyOfficeTokenService.Issue(service.IssueOnlyOfficeDocumentTokenInput{
		DocumentID:     document.DocumentID,
		ContentVersion: contentVersion,
		ActorUserID:    actorUserID,
		Purpose:        service.OnlyOfficeDocumentTokenPurposeSource,
	})
	if err != nil {
		setRequestErrmsg(c, err, "签发 ONLYOFFICE 源文件令牌失败")
		response.InternalError(c)
		return
	}
	callbackToken, _, err := h.onlyOfficeTokenService.Issue(service.IssueOnlyOfficeDocumentTokenInput{
		DocumentID:     document.DocumentID,
		ContentVersion: contentVersion,
		ActorUserID:    actorUserID,
		Purpose:        service.OnlyOfficeDocumentTokenPurposeCallback,
	})
	if err != nil {
		setRequestErrmsg(c, err, "签发 ONLYOFFICE callback 令牌失败")
		response.InternalError(c)
		return
	}

	sourceURL := buildOnlyOfficeAbsoluteURL(
		onlyOfficeConfig.CallbackPublicBaseURL,
		"/api/docs/"+neturl.PathEscape(document.DocumentID)+"/onlyoffice/source",
		neturl.Values{onlyOfficeAccessTokenQueryKey: []string{sourceToken}},
	)
	callbackURL := buildOnlyOfficeAbsoluteURL(
		onlyOfficeConfig.CallbackPublicBaseURL,
		"/api/docs/"+neturl.PathEscape(document.DocumentID)+"/onlyoffice/callback",
		neturl.Values{onlyOfficeAccessTokenQueryKey: []string{callbackToken}},
	)

	fileName := strings.TrimSpace(derefOptionalString(document.SourceFileName))
	if fileName == "" {
		fileName = resolveOfficeSourceFileName(document.Title, document.Format)
	}
	configPayload := map[string]any{
		"documentType": onlyOfficeDocumentType(document.Format),
		"document": map[string]any{
			"title":       fileName,
			"url":         sourceURL,
			"fileType":    officeDocumentFileExtension(document.Format),
			"key":         buildOnlyOfficeDocumentKey(document.DocumentID, contentVersion),
			"permissions": map[string]any{"edit": true, "download": true, "print": true},
		},
		"editorConfig": map[string]any{
			"mode":        "edit",
			"callbackUrl": callbackURL,
			"lang":        "zh-CN",
			"user": map[string]any{
				"id":   actorUserID,
				"name": buildOnlyOfficeUserDisplayName(session),
			},
			"customization": map[string]any{
				"autosave": true,
			},
		},
	}
	if strings.TrimSpace(onlyOfficeConfig.JWTSecret) != "" {
		token, err := signOnlyOfficeConfig(configPayload, onlyOfficeConfig.JWTSecret)
		if err != nil {
			setRequestErrmsg(c, err, "签发 ONLYOFFICE Docs JWT 失败")
			response.InternalError(c)
			return
		}
		configPayload["token"] = token
	}

	response.JSON(c, http.StatusOK, workspaceOnlyOfficeEditConfigResponse{
		DocumentServerURL: onlyOfficeConfig.DocumentServerURL,
		Config:            configPayload,
	})
}

// ServeOnlyOfficeSourceDocument 提供 ONLYOFFICE Document Server 拉取的正文文件。
func (h *workspaceHandler) ServeOnlyOfficeSourceDocument(c *gin.Context) {
	if h == nil || h.workspaceRepo == nil || h.documentAttachmentRepo == nil || h.onlyOfficeTokenService == nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	claims, ok := h.parseOnlyOfficeDocumentToken(c, service.OnlyOfficeDocumentTokenPurposeSource)
	if !ok {
		c.Status(http.StatusForbidden)
		return
	}

	documentID := strings.TrimSpace(c.Param("docId"))
	if documentID == "" || documentID != claims.DocumentID {
		c.Status(http.StatusForbidden)
		return
	}

	document, err := h.workspaceRepo.GetDocumentByDocumentID(c.Request.Context(), documentID)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	if !models.IsOfficeDocumentFormat(document.Format) {
		c.Status(http.StatusNotFound)
		return
	}
	if normalizeWorkspaceContentVersion(document.ContentVersion, document.Version) != claims.ContentVersion {
		c.Status(http.StatusNotFound)
		return
	}

	blobID := strings.TrimSpace(derefOptionalString(document.SourceBlobID))
	if blobID == "" {
		c.Status(http.StatusNotFound)
		return
	}
	blob, err := h.documentAttachmentRepo.GetBlobByBlobID(c.Request.Context(), blobID)
	if err != nil || blob == nil {
		c.Status(http.StatusNotFound)
		return
	}

	fileName := strings.TrimSpace(derefOptionalString(document.SourceFileName))
	if fileName == "" {
		fileName = resolveOfficeSourceFileName(document.Title, document.Format)
	}
	mimeType := resolveOnlyOfficeSourceMimeType(document.Format, document.SourceMimeType)

	if strings.EqualFold(strings.TrimSpace(blob.StorageProvider), string(service.ImageHostingProviderLocal)) {
		h.serveLocalOnlyOfficeSourceBlob(c, blob.ObjectKey, fileName, mimeType)
		return
	}

	targetURL, err := h.resolveOnlyOfficeBlobDownloadURL(c.Request.Context(), *blob, fileName)
	if err != nil || strings.TrimSpace(targetURL) == "" {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Redirect(http.StatusTemporaryRedirect, targetURL)
}

// HandleOnlyOfficeCallback 处理 ONLYOFFICE Docs 保存回调。
func (h *workspaceHandler) HandleOnlyOfficeCallback(c *gin.Context) {
	if h == nil || h.workspaceRepo == nil || h.documentAttachmentRepo == nil || h.onlyOfficeTokenService == nil {
		writeOnlyOfficeCallbackResult(c, 1)
		return
	}

	claims, ok := h.parseOnlyOfficeDocumentToken(c, service.OnlyOfficeDocumentTokenPurposeCallback)
	if !ok {
		writeOnlyOfficeCallbackResult(c, 1)
		return
	}

	documentID := strings.TrimSpace(c.Param("docId"))
	if documentID == "" || documentID != claims.DocumentID {
		writeOnlyOfficeCallbackResult(c, 1)
		return
	}

	var req onlyOfficeCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		setRequestErrmsg(c, err, "解析 ONLYOFFICE callback 请求失败")
		writeOnlyOfficeCallbackResult(c, 1)
		return
	}
	if !shouldPersistOnlyOfficeCallbackStatus(req.Status) {
		writeOnlyOfficeCallbackResult(c, 0)
		return
	}
	callbackFileURL := strings.TrimSpace(req.URL)
	if callbackFileURL == "" {
		writeOnlyOfficeCallbackResult(c, 1)
		return
	}

	current, err := h.workspaceRepo.GetDocumentByDocumentID(c.Request.Context(), documentID)
	if err != nil || current == nil || !models.IsOfficeDocumentFormat(current.Format) {
		writeOnlyOfficeCallbackResult(c, 0)
		return
	}

	currentContentVersion := normalizeWorkspaceContentVersion(current.ContentVersion, current.Version)
	if currentContentVersion != claims.ContentVersion {
		writeOnlyOfficeCallbackResult(c, 0)
		return
	}
	if rawKey := strings.TrimSpace(req.Key); rawKey != "" &&
		!matchesOnlyOfficeDocumentKey(rawKey, current.DocumentID, currentContentVersion) {
		writeOnlyOfficeCallbackResult(c, 0)
		return
	}

	fileContent, err := h.downloadOnlyOfficeCallbackFile(c.Request.Context(), callbackFileURL)
	if err != nil {
		setRequestErrmsg(c, err, "下载 ONLYOFFICE 输出文件失败")
		writeOnlyOfficeCallbackResult(c, 1)
		return
	}

	now := time.Now().UTC()
	sourceFileName := strings.TrimSpace(derefOptionalString(current.SourceFileName))
	if sourceFileName == "" {
		sourceFileName = resolveOfficeSourceFileName(current.Title, current.Format)
	}
	sourceMimeType := resolveOnlyOfficeSourceMimeType(current.Format, current.SourceMimeType)
	actorUserID := resolveOnlyOfficeCallbackActorUserID(claims, req.Users)

	sourceBlob, err := h.ensureBlobForContent(
		c.Request.Context(),
		fileContent,
		sourceMimeType,
		sourceFileName,
		current.SpaceID,
		current.DocumentID,
		actorUserID,
		now,
	)
	if err != nil {
		setRequestErrmsg(c, err, "保存 ONLYOFFICE 输出文件 blob 失败")
		writeOnlyOfficeCallbackResult(c, 1)
		return
	}

	nextVersion := current.Version + 1
	if nextVersion <= 0 {
		nextVersion = currentContentVersion + 1
	}
	nextContentVersion := currentContentVersion + 1
	var editorUserID *string
	if actorUserID != "" {
		editorUserID = &actorUserID
	}
	fileRevision := &models.DocumentFileRevision{
		DocumentFileRevisionID: strings.ToLower(ulidMakeString()),
		DocumentID:             current.DocumentID,
		BlobID:                 sourceBlob.BlobID,
		FileName:               sourceFileName,
		MimeType:               sourceMimeType,
		Version:                nextContentVersion,
		BaseVersion:            currentContentVersion,
		EditorUserID:           editorUserID,
		Source:                 models.RevisionSourceRemote,
		CreatedAt:              now,
	}

	saved, err := h.workspaceRepo.SaveOfficeDocument(c.Request.Context(), repository.WorkspaceSaveOfficeDocumentParams{
		DocumentID:         current.DocumentID,
		BaseContentVersion: currentContentVersion,
		NextVersion:        nextVersion,
		NextContentVersion: nextContentVersion,
		SourceBlobID:       sourceBlob.BlobID,
		SourceFileName:     sourceFileName,
		SourceMimeType:     sourceMimeType,
		ActorUserID:        actorUserID,
		NodeID:             current.NodeID,
		SpaceID:            current.SpaceID,
		TouchedAt:          now,
		FileRevision:       fileRevision,
	})
	if err != nil {
		setRequestErrmsg(c, err, "写回 ONLYOFFICE 文档版本失败")
		writeOnlyOfficeCallbackResult(c, 1)
		return
	}
	if !saved {
		writeOnlyOfficeCallbackResult(c, 0)
		return
	}

	if h.renderCache != nil {
		h.renderCache.PurgeDoc(current.DocumentID)
	}
	writeOnlyOfficeCallbackResult(c, 0)
}

func (h *workspaceHandler) parseOnlyOfficeDocumentToken(
	c *gin.Context,
	expectedPurpose service.OnlyOfficeDocumentTokenPurpose,
) (service.OnlyOfficeDocumentTokenClaims, bool) {
	if h == nil || h.onlyOfficeTokenService == nil {
		return service.OnlyOfficeDocumentTokenClaims{}, false
	}
	rawToken := strings.TrimSpace(c.Query(onlyOfficeAccessTokenQueryKey))
	if rawToken == "" {
		return service.OnlyOfficeDocumentTokenClaims{}, false
	}
	claims, err := h.onlyOfficeTokenService.Parse(rawToken)
	if err != nil {
		return service.OnlyOfficeDocumentTokenClaims{}, false
	}
	if claims.Purpose != expectedPurpose {
		return service.OnlyOfficeDocumentTokenClaims{}, false
	}
	return claims, true
}

func (h *workspaceHandler) serveLocalOnlyOfficeSourceBlob(
	c *gin.Context,
	objectKey string,
	fileName string,
	mimeType string,
) {
	targetPath, err := h.resolveLocalAttachmentTargetPath(objectKey)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header(
		"Content-Disposition",
		fmt.Sprintf("attachment; filename*=UTF-8''%s", neturl.PathEscape(fileName)),
	)
	c.Header("Content-Type", mimeType)
	c.Header("Cache-Control", "private, no-store, max-age=0")
	c.File(targetPath)
}

func (h *workspaceHandler) resolveOnlyOfficeBlobDownloadURL(
	ctx context.Context,
	blob models.DocumentAttachmentBlob,
	fileName string,
) (string, error) {
	if h == nil || h.imageHostingService == nil {
		return "", fmt.Errorf("image hosting service is nil")
	}
	config := service.DefaultImageHostingConfig()
	loadedConfig, err := h.imageHostingService.GetConfig(ctx)
	if err != nil {
		return "", err
	}
	config = loadedConfig

	return h.imageHostingService.BuildObjectReadURL(ctx, config, service.BuildImageHostingObjectReadURLInput{
		Provider:  service.ImageHostingProvider(strings.ToLower(strings.TrimSpace(blob.StorageProvider))),
		ObjectKey: blob.ObjectKey,
		ObjectURL: blob.ObjectURL,
		FileName:  fileName,
		Purpose:   service.DocumentAttachmentLinkPurposeDownload,
	})
}

func (h *workspaceHandler) downloadOnlyOfficeCallbackFile(
	ctx context.Context,
	fileURL string,
) ([]byte, error) {
	httpClient := h.onlyOfficeHTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "*/*")

	response, err := httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("unexpected onlyoffice callback download status %d", response.StatusCode)
	}

	content, err := io.ReadAll(io.LimitReader(response.Body, maxWorkspaceAttachmentSizeBytes+1))
	if err != nil {
		return nil, err
	}
	if len(content) == 0 {
		return nil, fmt.Errorf("onlyoffice callback file is empty")
	}
	if len(content) > maxWorkspaceAttachmentSizeBytes {
		return nil, fmt.Errorf("onlyoffice callback file exceeds max size %d bytes", maxWorkspaceAttachmentSizeBytes)
	}
	return content, nil
}

func buildOnlyOfficeAbsoluteURL(baseURL string, routePath string, query neturl.Values) string {
	trimmedBaseURL := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmedBaseURL == "" {
		return ""
	}
	rawPath := strings.TrimSpace(routePath)
	if !strings.HasPrefix(rawPath, "/") {
		rawPath = "/" + rawPath
	}
	if encodedQuery := query.Encode(); encodedQuery != "" {
		return trimmedBaseURL + rawPath + "?" + encodedQuery
	}
	return trimmedBaseURL + rawPath
}

func buildOnlyOfficeDocumentKey(documentID string, contentVersion int) string {
	normalizedDocumentID := normalizeOnlyOfficeDocumentKeySegment(documentID)
	normalizedContentVersion := contentVersion
	if normalizedContentVersion <= 0 {
		normalizedContentVersion = 1
	}
	return fmt.Sprintf("%s-v%d", normalizedDocumentID, normalizedContentVersion)
}

func matchesOnlyOfficeDocumentKey(rawKey string, documentID string, contentVersion int) bool {
	normalizedRawKey := strings.TrimSpace(rawKey)
	if normalizedRawKey == "" {
		return false
	}
	if normalizedRawKey == buildOnlyOfficeDocumentKey(documentID, contentVersion) {
		return true
	}
	// 兼容历史 key 形态，保障升级期间已打开会话仍可正常 callback。
	legacyKey := strings.TrimSpace(documentID) + ":" + fmt.Sprintf("%d", contentVersion)
	return normalizedRawKey == legacyKey
}

func normalizeOnlyOfficeDocumentKeySegment(value string) string {
	const maxSegmentLength = 96

	normalizedValue := strings.TrimSpace(value)
	if normalizedValue == "" {
		return "doc"
	}

	var builder strings.Builder
	builder.Grow(len(normalizedValue))
	for _, ch := range normalizedValue {
		if (ch >= '0' && ch <= '9') ||
			(ch >= 'A' && ch <= 'Z') ||
			(ch >= 'a' && ch <= 'z') ||
			ch == '-' ||
			ch == '.' ||
			ch == '_' ||
			ch == '=' {
			builder.WriteRune(ch)
			continue
		}
		builder.WriteRune('_')
	}
	sanitized := strings.Trim(builder.String(), "_")
	if sanitized == "" {
		sanitized = "doc"
	}
	if len(sanitized) > maxSegmentLength {
		sanitized = sanitized[:maxSegmentLength]
	}
	return sanitized
}

func buildOnlyOfficeUserDisplayName(session service.AuthSession) string {
	name := strings.TrimSpace(session.User.Name)
	if name != "" {
		return name
	}
	return strings.TrimSpace(session.User.Email)
}

func onlyOfficeDocumentType(format models.DocumentFormat) string {
	switch models.NormalizeDocumentFormat(format) {
	case models.DocumentFormatXLSX:
		return "spreadsheet"
	default:
		return "text"
	}
}

func resolveOnlyOfficeSourceMimeType(format models.DocumentFormat, storedMimeType *string) string {
	mimeType := strings.TrimSpace(derefOptionalString(storedMimeType))
	if mimeType != "" {
		return mimeType
	}
	switch models.NormalizeDocumentFormat(format) {
	case models.DocumentFormatXLSX:
		return officeDocumentMIMEXLSX
	default:
		return officeDocumentMIMEDOCX
	}
}

func signOnlyOfficeConfig(payload map[string]any, secret string) (string, error) {
	claims := jwt.MapClaims{}
	for key, value := range payload {
		claims[key] = value
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(strings.TrimSpace(secret)))
}

func shouldPersistOnlyOfficeCallbackStatus(status int) bool {
	switch status {
	case 2, 6:
		return true
	default:
		return false
	}
}

func resolveOnlyOfficeCallbackActorUserID(
	claims service.OnlyOfficeDocumentTokenClaims,
	users []string,
) string {
	if actorUserID := strings.TrimSpace(claims.ActorUserID); actorUserID != "" {
		return actorUserID
	}
	for _, userID := range users {
		if trimmedUserID := strings.TrimSpace(userID); trimmedUserID != "" {
			return trimmedUserID
		}
	}
	return ""
}

func writeOnlyOfficeCallbackResult(c *gin.Context, errorCode int) {
	if c == nil {
		return
	}
	c.JSON(http.StatusOK, gin.H{"error": errorCode})
}

func errorsIsRecordNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

func errorsIsSpaceNotFound(err error) bool {
	return errors.Is(err, service.ErrSpaceNotFound)
}

func errorsIsSpaceAccessDenied(err error) bool {
	return errors.Is(err, service.ErrSpaceAccessDenied)
}

func ulidMakeString() string {
	return ulid.Make().String()
}
