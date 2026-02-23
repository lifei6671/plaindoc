package handler

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
)

const (
	defaultLocalImageStorageRoot = "uploads/local"
	maxUploadImageSizeBytes      = 20 << 20
)

type imageHostingHandler struct {
	authService         *service.AuthService
	imageHostingService *service.ImageHostingService
	spaceRepo           repository.SpaceRepository
	localImageRootDir   string
}

// NewImageHostingHandler 创建图床配置与上传处理器。
func NewImageHostingHandler(
	authService *service.AuthService,
	imageHostingService *service.ImageHostingService,
	spaceRepo repository.SpaceRepository,
) *imageHostingHandler {
	return &imageHostingHandler{
		authService:         authService,
		imageHostingService: imageHostingService,
		spaceRepo:           spaceRepo,
		localImageRootDir:   defaultLocalImageStorageRoot,
	}
}

// GetConfig 返回当前生效图床配置。
func (h *imageHostingHandler) GetConfig(c *gin.Context) {
	if h == nil || h.authService == nil || h.imageHostingService == nil {
		response.InternalError(c)
		return
	}

	if _, ok := h.requireAuthenticatedUser(c); !ok {
		return
	}

	config, err := h.imageHostingService.GetConfig(c.Request.Context())
	if err != nil {
		response.InternalError(c)
		return
	}
	response.JSON(c, http.StatusOK, config)
}

// UploadImage 接收本地图片上传并返回可访问地址。
func (h *imageHostingHandler) UploadImage(c *gin.Context) {
	if h == nil || h.authService == nil || h.imageHostingService == nil || h.spaceRepo == nil {
		response.InternalError(c)
		return
	}

	actorUserID, ok := h.requireAuthenticatedUser(c)
	if !ok {
		return
	}

	spaceID := strings.TrimSpace(c.PostForm("spaceId"))
	if spaceID == "" {
		spaceID = strings.TrimSpace(c.Query("spaceId"))
	}
	if spaceID == "" {
		response.Error(c, http.StatusBadRequest, "INVALID_SPACE_ID", "spaceId is required")
		return
	}
	hasWriterAccess, err := h.spaceRepo.HasWriterAccess(c.Request.Context(), spaceID, actorUserID)
	if err != nil {
		response.InternalError(c)
		return
	}
	if !hasWriterAccess {
		response.Error(c, http.StatusForbidden, "FORBIDDEN", "insufficient space permission for image upload")
		return
	}

	config, err := h.imageHostingService.GetConfig(c.Request.Context())
	if err != nil {
		response.InternalError(c)
		return
	}
	if config.DefaultProvider != service.ImageHostingProviderLocal {
		response.Error(c, http.StatusBadRequest, "IMAGE_HOSTING_PROVIDER_DISABLED", "default provider is not local")
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil || fileHeader == nil {
		response.Error(c, http.StatusBadRequest, "INVALID_UPLOAD_FILE", "file is required")
		return
	}
	if fileHeader.Size <= 0 {
		response.Error(c, http.StatusBadRequest, "INVALID_UPLOAD_FILE", "file is empty")
		return
	}
	if fileHeader.Size > maxUploadImageSizeBytes {
		response.Error(c, http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", "file exceeds 20MB limit")
		return
	}

	contentType, err := detectUploadedFileContentType(fileHeader)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_UPLOAD_FILE", "cannot read upload file")
		return
	}
	if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		response.Error(c, http.StatusBadRequest, "INVALID_UPLOAD_FILE", "only image file is allowed")
		return
	}

	objectKey, err := buildImageObjectKey(fileHeader.Filename, contentType, time.Now().UTC())
	if err != nil {
		response.InternalError(c)
		return
	}

	localRootDir := strings.TrimSpace(h.localImageRootDir)
	if localRootDir == "" {
		localRootDir = defaultLocalImageStorageRoot
	}
	targetPath := filepath.Join(localRootDir, filepath.FromSlash(objectKey))
	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		response.InternalError(c)
		return
	}
	if err := c.SaveUploadedFile(fileHeader, targetPath); err != nil {
		response.InternalError(c)
		return
	}

	response.JSON(c, http.StatusOK, gin.H{
		"key": objectKey,
		"url": resolvePublicURL(config.Local.PublicBaseURL, objectKey, "/uploads"),
	})
}

// ServeLocalImage 提供本地图片静态访问。
func (h *imageHostingHandler) ServeLocalImage(c *gin.Context) {
	if h == nil {
		response.InternalError(c)
		return
	}

	rawPath := strings.TrimSpace(c.Param("path"))
	trimmedPath := strings.TrimPrefix(rawPath, "/")
	if trimmedPath == "" {
		response.Error(c, http.StatusNotFound, "FILE_NOT_FOUND", "file not found")
		return
	}

	cleanPath := path.Clean(trimmedPath)
	if cleanPath == "." || cleanPath == "/" || strings.HasPrefix(cleanPath, "../") {
		response.Error(c, http.StatusBadRequest, "INVALID_FILE_PATH", "invalid file path")
		return
	}
	// 兼容历史公开路径 /uploads/local/*：统一映射到本地存储根 uploads/local。
	cleanPath = strings.TrimPrefix(cleanPath, "local/")
	if cleanPath == "local" {
		response.Error(c, http.StatusNotFound, "FILE_NOT_FOUND", "file not found")
		return
	}

	localRootDir := strings.TrimSpace(h.localImageRootDir)
	if localRootDir == "" {
		localRootDir = defaultLocalImageStorageRoot
	}

	targetPath := filepath.Join(localRootDir, filepath.FromSlash(cleanPath))
	targetAbsPath, err := filepath.Abs(targetPath)
	if err != nil {
		response.InternalError(c)
		return
	}
	rootAbsPath, err := filepath.Abs(localRootDir)
	if err != nil {
		response.InternalError(c)
		return
	}
	if !isPathWithinRoot(rootAbsPath, targetAbsPath) {
		response.Error(c, http.StatusBadRequest, "INVALID_FILE_PATH", "invalid file path")
		return
	}

	fileInfo, err := os.Stat(targetAbsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			response.Error(c, http.StatusNotFound, "FILE_NOT_FOUND", "file not found")
			return
		}
		response.InternalError(c)
		return
	}
	if fileInfo.IsDir() {
		response.Error(c, http.StatusNotFound, "FILE_NOT_FOUND", "file not found")
		return
	}

	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.File(targetAbsPath)
}

func (h *imageHostingHandler) requireAuthenticatedUser(c *gin.Context) (string, bool) {
	accessToken, ok := bearerTokenFromRequest(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "authorization token is required")
		return "", false
	}

	session, err := h.authService.Me(c.Request.Context(), accessToken)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid access token")
		return "", false
	}

	return session.User.ID, true
}

func detectUploadedFileContentType(fileHeader *multipart.FileHeader) (string, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()

	buffer := make([]byte, 512)
	readCount, readErr := file.Read(buffer)
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return "", readErr
	}
	return strings.TrimSpace(http.DetectContentType(buffer[:readCount])), nil
}

func buildImageObjectKey(fileName string, contentType string, now time.Time) (string, error) {
	extension := resolveFileExtension(fileName, contentType)
	randomSuffix, err := randomHex(4)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"plaindoc/%04d/%02d/%02d/%d-%s.%s",
		now.Year(),
		int(now.Month()),
		now.Day(),
		now.UnixMilli(),
		randomSuffix,
		extension,
	), nil
}

func resolveFileExtension(fileName string, contentType string) string {
	extension := strings.TrimSpace(strings.ToLower(path.Ext(fileName)))
	if extension != "" {
		extension = strings.TrimPrefix(extension, ".")
		if isSafeFileExtension(extension) {
			return extension
		}
	}

	extensions, err := mime.ExtensionsByType(contentType)
	if err == nil {
		for _, item := range extensions {
			candidate := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(item)), ".")
			if isSafeFileExtension(candidate) {
				return candidate
			}
		}
	}

	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpeg":
		return "jpg"
	case "image/png":
		return "png"
	case "image/gif":
		return "gif"
	case "image/webp":
		return "webp"
	case "image/svg+xml":
		return "svg"
	case "image/bmp":
		return "bmp"
	case "image/tiff":
		return "tif"
	default:
		return "png"
	}
}

func isSafeFileExtension(extension string) bool {
	if extension == "" {
		return false
	}
	for _, char := range extension {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			continue
		}
		return false
	}
	return true
}

func randomHex(lengthBytes int) (string, error) {
	if lengthBytes <= 0 {
		return "", errors.New("random length must be positive")
	}
	raw := make([]byte, lengthBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func resolvePublicURL(baseURL string, objectPath string, fallbackBaseURL string) string {
	base := strings.TrimSpace(baseURL)
	if base == "" {
		base = strings.TrimSpace(fallbackBaseURL)
	}
	if base == "" {
		base = "/uploads"
	}
	// 兼容历史配置：将 /api/uploads/local 或 /uploads/local 归一到 /uploads。
	if strings.EqualFold(strings.TrimRight(base, "/"), "/api/uploads/local") {
		base = "/uploads"
	}
	if strings.EqualFold(strings.TrimRight(base, "/"), "/uploads/local") {
		base = "/uploads"
	}
	if strings.HasPrefix(base, "/api/uploads/") {
		base = strings.TrimPrefix(base, "/api")
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(objectPath, "/")
}

func isPathWithinRoot(rootAbsPath string, targetAbsPath string) bool {
	if rootAbsPath == targetAbsPath {
		return true
	}
	prefix := rootAbsPath + string(filepath.Separator)
	return strings.HasPrefix(targetAbsPath, prefix)
}
