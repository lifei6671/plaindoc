package server

import (
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/config"
)

const webSPAAssetsRoutePrefix = "/web-assets"

// registerWebSPARoutes 注册 Web SPA（登录/编辑/后台）生产托管路由。
//
// 仅当 WEB_DIST_DIR 可解析到有效构建产物（index.html）时启用；
// 否则保持现状（常见于本地 Vite 开发场景）。
func registerWebSPARoutes(router *gin.Engine, cfg config.Config, logger *slog.Logger) {
	if router == nil {
		return
	}

	distDir, ok := resolveWebDistDir(cfg.WebDistDir)
	if !ok {
		logWebSPAWarn(logger, "web dist directory not found, skip web spa routes", "configured", cfg.WebDistDir)
		return
	}

	indexPath, ok := resolveExistingFile(filepath.Join(distDir, "index.html"))
	if !ok {
		logWebSPAWarn(logger, "web dist index.html not found, skip web spa routes", "dist_dir", distDir)
		return
	}

	webAssetsDirs := resolveWebAssetsDirs(distDir, cfg)
	if len(webAssetsDirs) > 0 {
		router.StaticFS(webSPAAssetsRoutePrefix, newLayeredHTTPFileSystem(webAssetsDirs))
	} else {
		logWebSPAWarn(
			logger,
			"web-assets directory not found in dist/dist-ssr, spa shell may fail to load scripts",
			"dist_dir", distDir,
		)
	}

	serveIndex := func(c *gin.Context) {
		// SPA Shell 保持 no-cache，避免部署后命中过期入口文档。
		c.Header("Cache-Control", "no-cache")
		c.File(indexPath)
	}

	router.GET("/login", serveIndex)
	router.GET("/register", serveIndex)
	router.GET("/forgot-password", serveIndex)
	router.GET("/reset-password", serveIndex)
	router.GET("/editor", serveIndex)
	router.GET("/editor/*path", serveIndex)
	router.GET("/admin", serveIndex)
	router.GET("/admin/*path", serveIndex)

	logWebSPAInfo(logger, "web spa routes enabled", "dist_dir", distDir)
}

func resolveWebAssetsDirs(distDir string, cfg config.Config) []string {
	dirs := make([]string, 0, 3)
	seen := make(map[string]struct{}, 4)
	appendIfExists := func(candidate string) {
		absolutePath, ok := resolveExistingDir(candidate)
		if !ok {
			return
		}
		if _, exists := seen[absolutePath]; exists {
			return
		}
		seen[absolutePath] = struct{}{}
		dirs = append(dirs, absolutePath)
	}

	// 客户端构建产物目录（优先级最高）。
	appendIfExists(filepath.Join(distDir, "web-assets"))
	// SSR worker 构建产物目录（补足 SSR 内联字体/脚本依赖）。
	appendIfExists(filepath.Join(filepath.Dir(distDir), "dist-ssr", "web-assets"))

	if entry := strings.TrimSpace(cfg.SSRWorker.Entry); entry != "" {
		absoluteEntry, err := filepath.Abs(filepath.Clean(entry))
		if err == nil {
			appendIfExists(filepath.Join(filepath.Dir(absoluteEntry), "web-assets"))
		}
	}

	return dirs
}

type layeredHTTPFileSystem struct {
	sources []http.FileSystem
}

func newLayeredHTTPFileSystem(dirs []string) http.FileSystem {
	sources := make([]http.FileSystem, 0, len(dirs))
	for _, dir := range dirs {
		normalized := strings.TrimSpace(dir)
		if normalized == "" {
			continue
		}
		sources = append(sources, http.Dir(normalized))
	}
	return layeredHTTPFileSystem{sources: sources}
}

func (l layeredHTTPFileSystem) Open(name string) (http.File, error) {
	var firstErr error
	for _, source := range l.sources {
		file, err := source.Open(name)
		if err == nil {
			return file, nil
		}
		if firstErr == nil {
			firstErr = err
		}
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, os.ErrNotExist) {
			continue
		}
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return nil, fs.ErrNotExist
}

func resolveWebDistDir(pathValue string) (string, bool) {
	trimmed := strings.TrimSpace(pathValue)
	if trimmed == "" {
		return "", false
	}

	seen := make(map[string]struct{}, 32)
	appendCandidate := func(candidates []string, candidate string) []string {
		normalized := strings.TrimSpace(candidate)
		if normalized == "" {
			return candidates
		}
		cleaned := filepath.Clean(normalized)
		if cleaned == "" || cleaned == "." {
			return candidates
		}
		if _, exists := seen[cleaned]; exists {
			return candidates
		}
		seen[cleaned] = struct{}{}
		return append(candidates, cleaned)
	}

	candidates := make([]string, 0, 16)
	candidates = appendCandidate(candidates, trimmed)
	candidates = appendCandidate(candidates, filepath.Join("apps", "web", "dist"))
	candidates = appendCandidate(candidates, filepath.Join("web", "dist"))
	candidates = appendCandidate(candidates, filepath.Join("..", "web", "dist"))
	candidates = appendCandidate(candidates, filepath.Join("..", "apps", "web", "dist"))
	candidates = appendCandidate(candidates, filepath.Join("..", "..", "web", "dist"))
	candidates = appendCandidate(candidates, filepath.Join("..", "..", "apps", "web", "dist"))
	candidates = appendCandidate(candidates, filepath.Join("..", "..", "..", "web", "dist"))
	candidates = appendCandidate(candidates, filepath.Join("..", "..", "..", "apps", "web", "dist"))

	cwd, err := os.Getwd()
	if err == nil {
		current := filepath.Clean(cwd)
		for i := 0; i < 8; i++ {
			candidates = appendCandidate(candidates, filepath.Join(current, trimmed))
			candidates = appendCandidate(candidates, filepath.Join(current, "apps", "web", "dist"))
			candidates = appendCandidate(candidates, filepath.Join(current, "web", "dist"))

			parent := filepath.Dir(current)
			if parent == current {
				break
			}
			current = parent
		}
	}

	for _, candidate := range candidates {
		absolutePath, absErr := filepath.Abs(candidate)
		if absErr != nil {
			continue
		}
		fileInfo, statErr := os.Stat(absolutePath)
		if statErr != nil || !fileInfo.IsDir() {
			continue
		}
		return absolutePath, true
	}

	return "", false
}

func resolveExistingDir(pathValue string) (string, bool) {
	trimmed := strings.TrimSpace(pathValue)
	if trimmed == "" {
		return "", false
	}

	absolutePath, err := filepath.Abs(filepath.Clean(trimmed))
	if err != nil {
		return "", false
	}
	fileInfo, statErr := os.Stat(absolutePath)
	if statErr != nil || !fileInfo.IsDir() {
		return "", false
	}

	return absolutePath, true
}

func resolveExistingFile(pathValue string) (string, bool) {
	trimmed := strings.TrimSpace(pathValue)
	if trimmed == "" {
		return "", false
	}

	absolutePath, err := filepath.Abs(filepath.Clean(trimmed))
	if err != nil {
		return "", false
	}
	fileInfo, statErr := os.Stat(absolutePath)
	if statErr != nil || !fileInfo.Mode().IsRegular() {
		return "", false
	}

	return absolutePath, true
}

func logWebSPAWarn(logger *slog.Logger, message string, attrs ...any) {
	if logger == nil {
		return
	}
	logger.Warn(message, attrs...)
}

func logWebSPAInfo(logger *slog.Logger, message string, attrs ...any) {
	if logger == nil {
		return
	}
	logger.Info(message, attrs...)
}
