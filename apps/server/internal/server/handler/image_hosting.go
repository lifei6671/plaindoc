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

	// 先确认用户已登录，后续权限与上传行为都依赖用户身份。
	actorUserID, ok := h.requireAuthenticatedUser(c)
	if !ok {
		return
	}

	// 兼容 form 与 query 两种 spaceId 传参方式。
	spaceID := strings.TrimSpace(c.PostForm("spaceId"))
	if spaceID == "" {
		spaceID = strings.TrimSpace(c.Query("spaceId"))
	}
	if spaceID == "" {
		response.ImageHostingInvalidSpaceID(c)
		return
	}
	// 上传入口按“空间写权限”做鉴权，避免越权写入。
	hasWriterAccess, err := h.spaceRepo.HasWriterAccess(c.Request.Context(), spaceID, actorUserID)
	if err != nil {
		response.InternalError(c)
		return
	}
	if !hasWriterAccess {
		response.ImageHostingUploadForbidden(c)
		return
	}

	config, err := h.imageHostingService.GetConfig(c.Request.Context())
	if err != nil {
		response.InternalError(c)
		return
	}
	// 当前上传链路仅支持本地存储，其他 provider 直接拒绝。
	if config.DefaultProvider != service.ImageHostingProviderLocal {
		response.ImageHostingProviderDisabled(c)
		return
	}

	// 先做基础文件校验（存在、非空、大小限制）。
	fileHeader, err := c.FormFile("file")
	if err != nil || fileHeader == nil {
		response.ImageHostingUploadFileRequired(c)
		return
	}
	if fileHeader.Size <= 0 {
		response.ImageHostingUploadFileEmpty(c)
		return
	}
	if fileHeader.Size > maxUploadImageSizeBytes {
		response.ImageHostingUploadFileTooLarge(c)
		return
	}

	// 读取文件头探测 MIME，防止仅凭扩展名绕过校验。
	contentType, err := detectUploadedFileContentType(fileHeader)
	if err != nil {
		response.ImageHostingUploadFileUnreadable(c)
		return
	}
	if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		response.ImageHostingUploadFileNotImage(c)
		return
	}

	// 生成按日期分层的对象 key，降低单目录文件数并便于管理。
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
	// 先确保目录存在，再落盘上传文件。
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
		response.ImageHostingFileNotFound(c)
		return
	}

	// 清理请求路径并拦截明显的目录穿越输入。
	cleanPath := path.Clean(trimmedPath)
	if cleanPath == "." || cleanPath == "/" || strings.HasPrefix(cleanPath, "../") {
		response.ImageHostingInvalidFilePath(c)
		return
	}
	// 兼容历史公开路径 /uploads/local/*：统一映射到本地存储根 uploads/local。
	cleanPath = strings.TrimPrefix(cleanPath, "local/")
	if cleanPath == "local" {
		response.ImageHostingFileNotFound(c)
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
	// 二次校验目标路径必须位于根目录下，阻断路径逃逸。
	if !isPathWithinRoot(rootAbsPath, targetAbsPath) {
		response.ImageHostingInvalidFilePath(c)
		return
	}

	fileInfo, err := os.Stat(targetAbsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			response.ImageHostingFileNotFound(c)
			return
		}
		response.InternalError(c)
		return
	}
	if fileInfo.IsDir() {
		response.ImageHostingFileNotFound(c)
		return
	}

	// 静态图片长期缓存，减少重复回源。
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.File(targetAbsPath)
}

func (h *imageHostingHandler) requireAuthenticatedUser(c *gin.Context) (string, bool) {
	accessToken, ok := bearerTokenFromRequest(c)
	if !ok {
		response.ImageHostingTokenRequired(c)
		return "", false
	}

	session, err := h.authService.Me(c.Request.Context(), accessToken)
	if err != nil {
		response.ImageHostingTokenInvalid(c)
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

	// 仅需读取前 512 字节即可做内容类型探测。
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

	// key 结构：业务前缀 + 日期分区 + 时间戳 + 随机后缀 + 扩展名。
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
	// 优先使用上传文件名中的扩展名，但必须通过安全字符校验。
	extension := strings.TrimSpace(strings.ToLower(path.Ext(fileName)))
	if extension != "" {
		extension = strings.TrimPrefix(extension, ".")
		if isSafeFileExtension(extension) {
			return extension
		}
	}

	// 再尝试通过 MIME 类型推导扩展名。
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
