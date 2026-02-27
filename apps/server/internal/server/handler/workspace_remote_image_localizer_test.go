package handler

import (
	"context"
	"encoding/base64"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/service"
)

const tinyPNGBase64ForWorkspaceLocalizer = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+X7uQAAAAASUVORK5CYII="

func TestWorkspaceRemoteImageLocalizer_RetryWithoutReferer(t *testing.T) {
	imageBytes := decodeTinyPNGForWorkspaceLocalizer(t)
	requestReferers := make([]string, 0, 2)
	requestUserAgents := make([]string, 0, 2)
	requestAccepts := make([]string, 0, 2)

	imageServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestReferers = append(requestReferers, request.Header.Get("Referer"))
		requestUserAgents = append(requestUserAgents, request.Header.Get("User-Agent"))
		requestAccepts = append(requestAccepts, request.Header.Get("Accept"))

		if len(requestReferers) == 1 {
			http.Error(writer, "forbidden with referer", http.StatusForbidden)
			return
		}
		writer.Header().Set("Content-Type", "image/png")
		_, _ = writer.Write(imageBytes)
	}))
	defer imageServer.Close()

	localImageRootDir := t.TempDir()
	handler := &workspaceHandler{
		imageHostingService: service.NewImageHostingService(nil),
		localImageRootDir:   localImageRootDir,
		remoteImageHTTPClient: &http.Client{
			Timeout: 3 * time.Second,
		},
		remoteImageFailureCooldown: time.Hour,
		remoteImageFailureUntil:    make(map[string]time.Time),
	}
	remoteImageURL := imageServer.URL + "/blocked.png"
	inputMarkdown := "before\n\n![demo](" + remoteImageURL + ")\n\nafter"

	outputMarkdown := handler.localizeRemoteImageURLsInMarkdown(context.Background(), "doc-retry", inputMarkdown)

	if outputMarkdown == inputMarkdown {
		t.Fatalf("expected markdown to be localized, got unchanged content")
	}
	if !strings.Contains(outputMarkdown, "![demo](/uploads/") {
		t.Fatalf("expected localized markdown image url under /uploads, got %q", outputMarkdown)
	}
	if len(requestReferers) != 2 {
		t.Fatalf("expected 2 download attempts, got %d", len(requestReferers))
	}
	expectedReferer := imageServer.URL + "/"
	if requestReferers[0] != expectedReferer {
		t.Fatalf("expected first attempt referer %q, got %q", expectedReferer, requestReferers[0])
	}
	if requestReferers[1] != "" {
		t.Fatalf("expected second attempt without referer, got %q", requestReferers[1])
	}
	if !strings.Contains(requestUserAgents[0], "Mozilla/5.0") {
		t.Fatalf("expected browser user-agent, got %q", requestUserAgents[0])
	}
	if !strings.Contains(requestAccepts[0], "image/") {
		t.Fatalf("expected browser image accept header, got %q", requestAccepts[0])
	}

	fileCount := 0
	if err := filepath.WalkDir(localImageRootDir, func(_ string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		fileCount += 1
		return nil
	}); err != nil {
		t.Fatalf("walk localized image directory failed: %v", err)
	}
	if fileCount != 1 {
		t.Fatalf("expected one localized image file, got %d", fileCount)
	}
}

func TestWorkspaceRemoteImageLocalizer_FallbackFailsThenIgnore(t *testing.T) {
	requestReferers := make([]string, 0, 2)

	imageServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestReferers = append(requestReferers, request.Header.Get("Referer"))
		http.Error(writer, "forbidden", http.StatusForbidden)
	}))
	defer imageServer.Close()

	handler := &workspaceHandler{
		imageHostingService: service.NewImageHostingService(nil),
		localImageRootDir:   t.TempDir(),
		remoteImageHTTPClient: &http.Client{
			Timeout: 3 * time.Second,
		},
		remoteImageFailureCooldown: time.Hour,
		remoteImageFailureUntil:    make(map[string]time.Time),
	}
	inputMarkdown := "![blocked](" + imageServer.URL + "/blocked.png)"

	outputMarkdown := handler.localizeRemoteImageURLsInMarkdown(context.Background(), "doc-fail", inputMarkdown)

	if outputMarkdown != inputMarkdown {
		t.Fatalf("expected markdown unchanged when both attempts fail, got %q", outputMarkdown)
	}
	if len(requestReferers) != 2 {
		t.Fatalf("expected 2 attempts for referer fallback, got %d", len(requestReferers))
	}
	if requestReferers[0] == "" {
		t.Fatalf("expected first attempt to carry host referer")
	}
	if requestReferers[1] != "" {
		t.Fatalf("expected second attempt without referer, got %q", requestReferers[1])
	}
}

func TestWorkspaceRemoteImageLocalizer_SkipRetryWithinCooldown(t *testing.T) {
	requestCount := 0

	imageServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestCount += 1
		http.Error(writer, "forbidden", http.StatusForbidden)
	}))
	defer imageServer.Close()

	handler := &workspaceHandler{
		imageHostingService: service.NewImageHostingService(nil),
		localImageRootDir:   t.TempDir(),
		remoteImageHTTPClient: &http.Client{
			Timeout: 3 * time.Second,
		},
		remoteImageFailureCooldown: time.Hour,
		remoteImageFailureUntil:    make(map[string]time.Time),
	}
	markdownContent := "![blocked](" + imageServer.URL + "/blocked.png)"

	firstOutput := handler.localizeRemoteImageURLsInMarkdown(context.Background(), "doc-cooldown", markdownContent)
	secondOutput := handler.localizeRemoteImageURLsInMarkdown(context.Background(), "doc-cooldown", markdownContent)

	if firstOutput != markdownContent || secondOutput != markdownContent {
		t.Fatalf("expected markdown unchanged when localization fails")
	}
	// 首次失败会执行“带 referer + 无 referer”两次；冷却期间第二次调用应直接跳过。
	if requestCount != 2 {
		t.Fatalf("expected 2 download attempts with cooldown skip, got %d", requestCount)
	}
}

func decodeTinyPNGForWorkspaceLocalizer(t *testing.T) []byte {
	t.Helper()
	decoded, err := base64.StdEncoding.DecodeString(tinyPNGBase64ForWorkspaceLocalizer)
	if err != nil {
		t.Fatalf("decode tiny png failed: %v", err)
	}
	return decoded
}
