package handler

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/service"
)

const (
	remoteImageDownloadTimeout        = 12 * time.Second
	defaultRemoteImageFailureCooldown = 10 * time.Minute
	remoteImageBrowserUserAgent       = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
		"(KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	remoteImageBrowserAccept         = "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8"
	remoteImageBrowserAcceptLanguage = "zh-CN,zh;q=0.9,en;q=0.8"
)

var markdownImagePattern = regexp.MustCompile(`!\[([^\]]*)\]\((\S+?)(?:\s+"([^"]*)")?\)`)

// localizeRemoteImageURLsInMarkdown 将 Markdown 中可访问的外链图片转存到本地并替换 URL。
// 失败场景按“忽略并保留原链接”处理，不中断文档保存流程。
func (h *workspaceHandler) localizeRemoteImageURLsInMarkdown(
	ctx context.Context,
	documentID string,
	markdownContent string,
) string {
	if h == nil || strings.TrimSpace(markdownContent) == "" {
		return markdownContent
	}

	config := service.DefaultImageHostingConfig()
	if h.imageHostingService != nil {
		loadedConfig, err := h.imageHostingService.GetConfig(ctx)
		if err == nil {
			config = loadedConfig
		}
	}
	remoteImageURLs := extractRemoteImageURLsFromMarkdown(markdownContent)
	if len(remoteImageURLs) == 0 {
		return markdownContent
	}

	imageURLMapping := make(map[string]string, len(remoteImageURLs))
	for _, remoteImageURL := range remoteImageURLs {
		if h.shouldSkipRemoteImageLocalize(documentID, remoteImageURL) {
			continue
		}
		localURL, err := h.downloadAndPersistRemoteImage(ctx, remoteImageURL, config.Local.PublicBaseURL)
		if err != nil {
			h.recordRemoteImageLocalizeFailure(documentID, remoteImageURL)
			continue
		}
		h.clearRemoteImageLocalizeFailure(documentID, remoteImageURL)
		imageURLMapping[remoteImageURL] = localURL
	}

	if len(imageURLMapping) == 0 {
		return markdownContent
	}
	return replaceMarkdownImageURLs(markdownContent, imageURLMapping)
}

func (h *workspaceHandler) downloadAndPersistRemoteImage(
	ctx context.Context,
	remoteImageURL string,
	localPublicBaseURL string,
) (string, error) {
	content, contentType, sourceFileName, err := h.fetchRemoteImageWithRetry(ctx, remoteImageURL)
	if err != nil {
		return "", err
	}

	objectKey, err := buildImageObjectKey(sourceFileName, contentType, time.Now().UTC())
	if err != nil {
		return "", err
	}

	localRootDir := strings.TrimSpace(h.localImageRootDir)
	if localRootDir == "" {
		localRootDir = defaultLocalImageStorageRoot
	}
	targetPath := filepath.Join(localRootDir, filepath.FromSlash(objectKey))
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(targetPath, content, 0o644); err != nil {
		return "", err
	}

	return resolvePublicURL(localPublicBaseURL, objectKey, "/uploads"), nil
}

func (h *workspaceHandler) fetchRemoteImageWithRetry(
	ctx context.Context,
	remoteImageURL string,
) ([]byte, string, string, error) {
	referer := buildImageHostReferer(remoteImageURL)
	content, contentType, sourceFileName, err := h.fetchRemoteImageOnce(ctx, remoteImageURL, referer)
	if err == nil {
		return content, contentType, sourceFileName, nil
	}
	if referer == "" {
		return nil, "", "", err
	}

	// 防盗链常见场景：先带 host referer，失败后回退无 referer。
	content, contentType, sourceFileName, fallbackErr := h.fetchRemoteImageOnce(ctx, remoteImageURL, "")
	if fallbackErr == nil {
		return content, contentType, sourceFileName, nil
	}
	return nil, "", "", fmt.Errorf("download with referer failed: %w; without referer failed: %v", err, fallbackErr)
}

func (h *workspaceHandler) fetchRemoteImageOnce(
	ctx context.Context,
	remoteImageURL string,
	referer string,
) ([]byte, string, string, error) {
	httpClient := h.remoteImageHTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: remoteImageDownloadTimeout}
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, remoteImageURL, nil)
	if err != nil {
		return nil, "", "", err
	}
	applyRemoteImageBrowserHeaders(request, referer)

	response, err := httpClient.Do(request)
	if err != nil {
		return nil, "", "", err
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, "", "", fmt.Errorf("unexpected status code %d", response.StatusCode)
	}

	content, err := io.ReadAll(io.LimitReader(response.Body, maxUploadImageSizeBytes+1))
	if err != nil {
		return nil, "", "", err
	}
	if len(content) > maxUploadImageSizeBytes {
		return nil, "", "", fmt.Errorf("image exceeds max size %d bytes", maxUploadImageSizeBytes)
	}

	contentType := detectRemoteImageContentType(response.Header.Get("Content-Type"), content)
	if contentType == "" {
		return nil, "", "", fmt.Errorf("response content is not image")
	}
	sourceFileName := buildRemoteImageSourceFilename(remoteImageURL, contentType)

	return content, contentType, sourceFileName, nil
}

func applyRemoteImageBrowserHeaders(request *http.Request, referer string) {
	if request == nil {
		return
	}
	request.Header.Set("User-Agent", remoteImageBrowserUserAgent)
	request.Header.Set("Accept", remoteImageBrowserAccept)
	request.Header.Set("Accept-Language", remoteImageBrowserAcceptLanguage)
	request.Header.Set("Cache-Control", "no-cache")
	request.Header.Set("Pragma", "no-cache")
	request.Header.Set("Sec-Fetch-Dest", "image")
	request.Header.Set("Sec-Fetch-Mode", "no-cors")
	request.Header.Set("Sec-Fetch-Site", "cross-site")
	if strings.TrimSpace(referer) != "" {
		request.Header.Set("Referer", referer)
	}
}

func buildImageHostReferer(remoteImageURL string) string {
	parsedURL, err := url.Parse(strings.TrimSpace(remoteImageURL))
	if err != nil {
		return ""
	}
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return ""
	}
	return parsedURL.Scheme + "://" + parsedURL.Host + "/"
}

func detectRemoteImageContentType(responseContentType string, content []byte) string {
	if normalized := normalizeImageContentType(responseContentType); normalized != "" {
		return normalized
	}
	detectedContentType := strings.ToLower(strings.TrimSpace(http.DetectContentType(content)))
	if strings.HasPrefix(detectedContentType, "image/") {
		return detectedContentType
	}
	return ""
}

func normalizeImageContentType(rawContentType string) string {
	contentType, _, err := mime.ParseMediaType(strings.TrimSpace(rawContentType))
	if err != nil {
		return ""
	}
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if strings.HasPrefix(contentType, "image/") {
		return contentType
	}
	return ""
}

func buildRemoteImageSourceFilename(remoteImageURL string, contentType string) string {
	defaultFileName := "remote-image"
	parsedURL, err := url.Parse(strings.TrimSpace(remoteImageURL))
	if err != nil {
		return defaultFileName
	}
	baseName := strings.TrimSpace(path.Base(parsedURL.Path))
	if baseName == "" || baseName == "." || baseName == "/" {
		baseName = defaultFileName
	}
	if decodedBaseName, decodeErr := url.PathUnescape(baseName); decodeErr == nil && strings.TrimSpace(decodedBaseName) != "" {
		baseName = decodedBaseName
	}
	if strings.TrimSpace(path.Ext(baseName)) != "" {
		return baseName
	}
	extensions, err := mime.ExtensionsByType(contentType)
	if err != nil || len(extensions) == 0 {
		return baseName
	}
	return baseName + extensions[0]
}

func extractRemoteImageURLsFromMarkdown(markdownContent string) []string {
	remoteImageURLs := make([]string, 0)
	seenImageURLSet := make(map[string]struct{})
	matches := markdownImagePattern.FindAllStringSubmatch(markdownContent, -1)
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		imageURL := strings.TrimSpace(match[2])
		if !isRemoteImageURL(imageURL) {
			continue
		}
		if _, exists := seenImageURLSet[imageURL]; exists {
			continue
		}
		seenImageURLSet[imageURL] = struct{}{}
		remoteImageURLs = append(remoteImageURLs, imageURL)
	}
	return remoteImageURLs
}

func replaceMarkdownImageURLs(markdownContent string, imageURLMapping map[string]string) string {
	if len(imageURLMapping) == 0 {
		return markdownContent
	}
	return markdownImagePattern.ReplaceAllStringFunc(markdownContent, func(fullMatch string) string {
		match := markdownImagePattern.FindStringSubmatch(fullMatch)
		if len(match) < 3 {
			return fullMatch
		}
		altText := match[1]
		imageURL := strings.TrimSpace(match[2])
		mappedURL, ok := imageURLMapping[imageURL]
		if !ok {
			return fullMatch
		}
		imageTitle := ""
		if len(match) >= 4 {
			imageTitle = strings.TrimSpace(match[3])
		}
		if imageTitle != "" {
			return fmt.Sprintf("![%s](%s \"%s\")", altText, mappedURL, imageTitle)
		}
		return fmt.Sprintf("![%s](%s)", altText, mappedURL)
	})
}

func isRemoteImageURL(rawURL string) bool {
	parsedURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	return parsedURL.Scheme == "http" || parsedURL.Scheme == "https"
}

func (h *workspaceHandler) shouldSkipRemoteImageLocalize(documentID string, remoteImageURL string) bool {
	if h == nil {
		return false
	}
	failureKey := buildRemoteImageFailureKey(documentID, remoteImageURL)
	if failureKey == "" {
		return false
	}
	now := time.Now().UTC()

	h.remoteImageFailureMu.Lock()
	defer h.remoteImageFailureMu.Unlock()

	failureUntil, exists := h.remoteImageFailureUntil[failureKey]
	if !exists {
		return false
	}
	if !failureUntil.After(now) {
		delete(h.remoteImageFailureUntil, failureKey)
		return false
	}
	return true
}

func (h *workspaceHandler) recordRemoteImageLocalizeFailure(documentID string, remoteImageURL string) {
	if h == nil {
		return
	}
	cooldown := h.remoteImageFailureCooldown
	if cooldown <= 0 {
		return
	}
	failureKey := buildRemoteImageFailureKey(documentID, remoteImageURL)
	if failureKey == "" {
		return
	}

	h.remoteImageFailureMu.Lock()
	defer h.remoteImageFailureMu.Unlock()

	if h.remoteImageFailureUntil == nil {
		h.remoteImageFailureUntil = make(map[string]time.Time)
	}
	h.remoteImageFailureUntil[failureKey] = time.Now().UTC().Add(cooldown)
}

func (h *workspaceHandler) clearRemoteImageLocalizeFailure(documentID string, remoteImageURL string) {
	if h == nil {
		return
	}
	failureKey := buildRemoteImageFailureKey(documentID, remoteImageURL)
	if failureKey == "" {
		return
	}

	h.remoteImageFailureMu.Lock()
	defer h.remoteImageFailureMu.Unlock()

	if h.remoteImageFailureUntil == nil {
		return
	}
	delete(h.remoteImageFailureUntil, failureKey)
}

func buildRemoteImageFailureKey(documentID string, remoteImageURL string) string {
	normalizedDocumentID := strings.TrimSpace(documentID)
	normalizedImageURL := strings.TrimSpace(remoteImageURL)
	if normalizedDocumentID == "" || normalizedImageURL == "" {
		return ""
	}
	return normalizedDocumentID + "\n" + normalizedImageURL
}
