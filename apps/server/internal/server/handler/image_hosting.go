package handler

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"github.com/oklog/ulid/v2"
)

const (
	defaultLocalImageStorageRoot = "uploads"
	legacyLocalImageStorageRoot  = "uploads/local"
	maxUploadImageSizeBytes      = 20 << 20
)

var (
	errUploadedImageTooLarge      = errors.New("uploaded image file is too large")
	errLocalImagePathOutOfRootDir = errors.New("local image path is out of root dir")
)

type imageHostingHandler struct {
	authService         *service.AuthService
	imageHostingService *service.ImageHostingService
	spaceRepo           repository.SpaceRepository
	localImageRootDir   string
}

type imageHostingClientConfigResponse struct {
	DefaultProvider   service.ImageHostingProvider  `json:"defaultProvider"`
	ObjectKeyEndpoint string                        `json:"objectKeyEndpoint"`
	Local             imageHostingClientLocalConfig `json:"local"`
}

type imageHostingClientLocalConfig struct {
	UploadEndpoint string `json:"uploadEndpoint"`
	PublicBaseURL  string `json:"publicBaseUrl"`
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
		setRequestErrmsg(c, err, "读取图床配置失败")
		response.InternalError(c)
		return
	}
	response.JSON(c, http.StatusOK, mapImageHostingClientConfig(config))
}

func mapImageHostingClientConfig(config service.ImageHostingConfig) imageHostingClientConfigResponse {
	return imageHostingClientConfigResponse{
		DefaultProvider:   config.DefaultProvider,
		ObjectKeyEndpoint: "/api/uploads/images/object-key",
		Local: imageHostingClientLocalConfig{
			UploadEndpoint: config.Local.UploadEndpoint,
			PublicBaseURL:  config.Local.PublicBaseURL,
		},
	}
}

// IssueImageObjectKey 由后端统一分配图片对象 key，避免前端自行拼接文件名。
func (h *imageHostingHandler) IssueImageObjectKey(c *gin.Context) {
	if h == nil || h.authService == nil || h.imageHostingService == nil || h.spaceRepo == nil {
		setRequestErrmsgText(c, "图片对象 key 分配失败: handler or dependencies is nil")
		response.InternalError(c)
		return
	}

	actorUserID, ok := h.requireAuthenticatedUser(c)
	if !ok {
		return
	}

	requestPayload := readIssueImageObjectKeyRequest(c)
	spaceID := strings.TrimSpace(requestPayload.SpaceID)
	if spaceID == "" {
		response.ImageHostingInvalidSpaceID(c)
		return
	}

	hasWriterAccess, err := h.spaceRepo.HasWriterAccess(c.Request.Context(), spaceID, actorUserID)
	if err != nil {
		setRequestErrmsg(c, err, "校验图片对象 key 分配空间权限失败")
		response.InternalError(c)
		return
	}
	if !hasWriterAccess {
		response.ImageHostingUploadForbidden(c)
		return
	}

	config, err := h.imageHostingService.GetConfig(c.Request.Context())
	if err != nil {
		setRequestErrmsg(c, err, "读取图床配置失败")
		response.InternalError(c)
		return
	}

	targetProvider := normalizeImageObjectKeyProvider(requestPayload.Provider)
	if targetProvider == "" {
		targetProvider = config.DefaultProvider
	}
	if targetProvider == "" {
		targetProvider = service.ImageHostingProviderLocal
	}

	contentType := strings.TrimSpace(requestPayload.ContentType)
	if contentType != "" && !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		setRequestErrmsgText(c, "图片对象 key 分配失败: contentType must be image/*")
		response.ImageHostingUploadFileNotImage(c)
		return
	}
	if contentType == "" {
		contentType = "image/png"
	}
	if normalizeImageProcessingMode(config.ImageProcessing.Mode) == service.ImageHostingImageProcessingModeToWebP {
		contentType = "image/webp"
	}

	objectKey, err := buildImageObjectKey(
		requestPayload.FileName,
		contentType,
		spaceID,
		requestPayload.DocumentID,
		actorUserID,
		time.Now().UTC(),
		config.UploadPathTemplate(targetProvider),
	)
	if err != nil {
		setRequestErrmsg(c, err, "生成图片对象 key 失败")
		response.InternalError(c)
		return
	}

	response.JSON(c, http.StatusOK, gin.H{
		"provider": targetProvider,
		"key":      objectKey,
	})
}

type issueImageObjectKeyRequest struct {
	Provider    string `json:"provider"`
	SpaceID     string `json:"spaceId"`
	DocumentID  string `json:"docId"`
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
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
	documentID := strings.TrimSpace(c.PostForm("docId"))
	if documentID == "" {
		documentID = strings.TrimSpace(c.Query("docId"))
	}
	if spaceID == "" {
		response.ImageHostingInvalidSpaceID(c)
		return
	}
	// 上传入口按“空间写权限”做鉴权，避免越权写入。
	hasWriterAccess, err := h.spaceRepo.HasWriterAccess(c.Request.Context(), spaceID, actorUserID)
	if err != nil {
		setRequestErrmsg(c, err, "验证图片上传空间权限失败")
		response.InternalError(c)
		return
	}
	if !hasWriterAccess {
		response.ImageHostingUploadForbidden(c)
		return
	}

	config, err := h.imageHostingService.GetConfig(c.Request.Context())
	if err != nil {
		setRequestErrmsg(c, err, "读取图床配置失败")
		response.InternalError(c)
		return
	}
	targetProvider := config.DefaultProvider
	if targetProvider == "" {
		targetProvider = service.ImageHostingProviderLocal
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

	originalContent, err := readUploadedFileContent(fileHeader, maxUploadImageSizeBytes)
	if err != nil {
		if errors.Is(err, errUploadedImageTooLarge) {
			response.ImageHostingUploadFileTooLarge(c)
			return
		}
		setRequestErrmsg(c, err, "读取上传图片内容失败")
		response.ImageHostingUploadFileUnreadable(c)
		return
	}
	contentType := strings.TrimSpace(http.DetectContentType(firstBytes(originalContent, 512)))
	if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		response.ImageHostingUploadFileNotImage(c)
		return
	}
	processedImage, err := processUploadedImageForStorage(originalContent, contentType, config)
	if err != nil {
		setRequestErrmsg(c, err, "处理上传图片失败")
		response.ImageHostingUploadFileUnreadable(c)
		return
	}
	if len(processedImage.Content) == 0 {
		setRequestErrmsgText(c, "处理后的图片内容为空")
		response.ImageHostingUploadFileUnreadable(c)
		return
	}
	if int64(len(processedImage.Content)) > maxUploadImageSizeBytes {
		response.ImageHostingUploadFileTooLarge(c)
		return
	}

	// 生成按日期分层的对象 key，降低单目录文件数并便于管理。
	objectKey, err := buildImageObjectKey(
		fileHeader.Filename,
		processedImage.ContentType,
		spaceID,
		documentID,
		actorUserID,
		time.Now().UTC(),
		config.UploadPathTemplate(targetProvider),
	)
	if err != nil {
		setRequestErrmsg(c, err, "生成图片对象 key 失败")
		response.InternalError(c)
		return
	}

	accessURL := ""
	switch targetProvider {
	case service.ImageHostingProviderLocal:
		localRootDir := strings.TrimSpace(h.localImageRootDir)
		if localRootDir == "" {
			localRootDir = defaultLocalImageStorageRoot
		}
		targetPath := filepath.Join(localRootDir, filepath.FromSlash(objectKey))
		targetDir := filepath.Dir(targetPath)
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			setRequestErrmsg(c, err, "创建图片目录失败")
			response.InternalError(c)
			return
		}
		if err := os.WriteFile(targetPath, processedImage.Content, 0o644); err != nil {
			setRequestErrmsg(c, err, "保存图片文件失败")
			response.InternalError(c)
			return
		}
		accessURL = resolvePublicURL(config.Local.PublicBaseURL, objectKey, "/uploads")
	case service.ImageHostingProviderCloudflareR2:
		uploadedURL, uploadErr := uploadImageToCloudflareR2(
			c.Request.Context(),
			processedImage.Content,
			processedImage.ContentType,
			objectKey,
			config,
		)
		if uploadErr != nil {
			setRequestErrmsg(c, uploadErr, "上传图片到 Cloudflare R2 失败")
			response.InternalError(c)
			return
		}
		accessURL = uploadedURL
	case service.ImageHostingProviderAliyunOSS:
		uploadedURL, uploadErr := uploadImageToAliyunOSS(
			processedImage.Content,
			processedImage.ContentType,
			objectKey,
			config,
		)
		if uploadErr != nil {
			setRequestErrmsg(c, uploadErr, "上传图片到阿里云 OSS 失败")
			response.InternalError(c)
			return
		}
		accessURL = uploadedURL
	default:
		setRequestErrmsgText(c, "当前默认图床 provider 不受支持")
		response.ImageHostingProviderDisabled(c)
		return
	}
	if strings.TrimSpace(accessURL) == "" {
		setRequestErrmsgText(c, "图片上传成功但访问地址为空")
		response.InternalError(c)
		return
	}

	response.JSON(c, http.StatusOK, gin.H{
		"key":      objectKey,
		"url":      accessURL,
		"provider": targetProvider,
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
	targetAbsPath, err := resolveLocalImageAbsolutePath(localRootDir, cleanPath)
	if err != nil {
		if errors.Is(err, errLocalImagePathOutOfRootDir) {
			response.ImageHostingInvalidFilePath(c)
			return
		}
		setRequestErrmsg(c, err, "解析图片目标路径失败")
		response.InternalError(c)
		return
	}

	fileInfo, err := os.Stat(targetAbsPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			setRequestErrmsg(c, err, "读取图片文件信息失败")
			response.InternalError(c)
			return
		}

		// 兼容历史落盘目录 uploads/local/*（旧版默认目录）。
		// 新版默认目录为 uploads/*。
		legacyRootDir := strings.TrimSpace(legacyLocalImageStorageRoot)
		if filepath.Clean(localRootDir) != filepath.Clean(legacyRootDir) {
			legacyAbsPath, legacyResolveErr := resolveLocalImageAbsolutePath(legacyRootDir, cleanPath)
			if legacyResolveErr != nil {
				if errors.Is(legacyResolveErr, errLocalImagePathOutOfRootDir) {
					response.ImageHostingInvalidFilePath(c)
					return
				}
				setRequestErrmsg(c, legacyResolveErr, "解析历史图片目标路径失败")
				response.InternalError(c)
				return
			}

			legacyFileInfo, legacyStatErr := os.Stat(legacyAbsPath)
			if legacyStatErr == nil && !legacyFileInfo.IsDir() {
				targetAbsPath = legacyAbsPath
			} else if legacyStatErr != nil && !errors.Is(legacyStatErr, os.ErrNotExist) {
				setRequestErrmsg(c, legacyStatErr, "读取历史图片文件信息失败")
				response.InternalError(c)
				return
			} else {
				response.ImageHostingFileNotFound(c)
				return
			}
		} else {
			response.ImageHostingFileNotFound(c)
			return
		}
	} else if fileInfo.IsDir() {
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

func readUploadedFileContent(fileHeader *multipart.FileHeader, maxSize int64) ([]byte, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()

	limitedReader := io.LimitReader(file, maxSize+1)
	content, readErr := io.ReadAll(limitedReader)
	if readErr != nil {
		return nil, readErr
	}
	if int64(len(content)) > maxSize {
		return nil, errUploadedImageTooLarge
	}
	return content, nil
}

func firstBytes(input []byte, limit int) []byte {
	if len(input) <= limit {
		return input
	}
	return input[:limit]
}

func readIssueImageObjectKeyRequest(c *gin.Context) issueImageObjectKeyRequest {
	if c == nil {
		return issueImageObjectKeyRequest{}
	}
	requestPayload := issueImageObjectKeyRequest{
		Provider:    strings.TrimSpace(c.Query("provider")),
		SpaceID:     strings.TrimSpace(c.Query("spaceId")),
		DocumentID:  strings.TrimSpace(c.Query("docId")),
		FileName:    strings.TrimSpace(c.Query("fileName")),
		ContentType: strings.TrimSpace(c.Query("contentType")),
	}

	if requestPayload.Provider == "" {
		requestPayload.Provider = strings.TrimSpace(c.PostForm("provider"))
	}
	if requestPayload.SpaceID == "" {
		requestPayload.SpaceID = strings.TrimSpace(c.PostForm("spaceId"))
	}
	if requestPayload.DocumentID == "" {
		requestPayload.DocumentID = strings.TrimSpace(c.PostForm("docId"))
	}
	if requestPayload.FileName == "" {
		requestPayload.FileName = strings.TrimSpace(c.PostForm("fileName"))
	}
	if requestPayload.ContentType == "" {
		requestPayload.ContentType = strings.TrimSpace(c.PostForm("contentType"))
	}

	var bodyPayload issueImageObjectKeyRequest
	if bindErr := c.ShouldBindJSON(&bodyPayload); bindErr == nil {
		if requestPayload.Provider == "" {
			requestPayload.Provider = strings.TrimSpace(bodyPayload.Provider)
		}
		if requestPayload.SpaceID == "" {
			requestPayload.SpaceID = strings.TrimSpace(bodyPayload.SpaceID)
		}
		if requestPayload.DocumentID == "" {
			requestPayload.DocumentID = strings.TrimSpace(bodyPayload.DocumentID)
		}
		if requestPayload.FileName == "" {
			requestPayload.FileName = strings.TrimSpace(bodyPayload.FileName)
		}
		if requestPayload.ContentType == "" {
			requestPayload.ContentType = strings.TrimSpace(bodyPayload.ContentType)
		}
	}

	return requestPayload
}

func normalizeImageObjectKeyProvider(rawProvider string) service.ImageHostingProvider {
	switch strings.ToLower(strings.TrimSpace(rawProvider)) {
	case string(service.ImageHostingProviderLocal):
		return service.ImageHostingProviderLocal
	case string(service.ImageHostingProviderCloudflareR2):
		return service.ImageHostingProviderCloudflareR2
	case string(service.ImageHostingProviderAliyunOSS):
		return service.ImageHostingProviderAliyunOSS
	default:
		return ""
	}
}

func buildImageObjectKey(
	fileName string,
	contentType string,
	spaceID string,
	documentID string,
	uploaderUserID string,
	now time.Time,
	uploadPathTemplate string,
) (string, error) {
	extension := sanitizePathSegment(resolveFileExtension(fileName, contentType), "png")
	assetID := strings.ToLower(ulid.Make().String())
	replaced, err := service.RenderImageHostingUploadPathTemplate(uploadPathTemplate, map[string]string{
		"spaceId":    sanitizePathSegment(spaceID, "space"),
		"docId":      sanitizePathSegment(documentID, "doc"),
		"yyyy":       fmt.Sprintf("%04d", now.Year()),
		"mm":         fmt.Sprintf("%02d", int(now.Month())),
		"dd":         fmt.Sprintf("%02d", now.Day()),
		"hh":         fmt.Sprintf("%02d", now.Hour()),
		"assetId":    sanitizePathSegment(assetID, "asset"),
		"origName":   sanitizePathSegment(resolveOriginName(fileName), "file"),
		"ext":        extension,
		"uploaderId": sanitizePathSegment(uploaderUserID, "uploader"),
	})
	if err != nil {
		return "", err
	}

	normalizedObjectKey := strings.TrimSpace(strings.TrimPrefix(replaced, "/"))
	if normalizedObjectKey == "" {
		return "", errors.New("image object key is empty")
	}
	cleanObjectKey := path.Clean(normalizedObjectKey)
	if cleanObjectKey == "." || cleanObjectKey == "/" || strings.HasPrefix(cleanObjectKey, "../") {
		return "", errors.New("image object key is invalid")
	}
	if !strings.HasPrefix(cleanObjectKey, "images/") {
		return "", errors.New("image object key must start with images/")
	}
	if len(cleanObjectKey) > 512 {
		return "", errors.New("image object key is too long")
	}
	return cleanObjectKey, nil
}

func resolveFileExtension(fileName string, contentType string) string {
	// 优先通过 MIME 类型推导扩展名，确保扩展名与实际内容一致。
	extensions, err := mime.ExtensionsByType(contentType)
	if err == nil {
		for _, item := range extensions {
			candidate := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(item)), ".")
			if isSafeFileExtension(candidate) {
				return candidate
			}
		}
	}

	// 再回退到上传文件名中的扩展名，但必须通过安全字符校验。
	extension := strings.TrimSpace(strings.ToLower(path.Ext(fileName)))
	if extension != "" {
		extension = strings.TrimPrefix(extension, ".")
		if isSafeFileExtension(extension) {
			return extension
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

func resolveOriginName(fileName string) string {
	trimmed := strings.TrimSpace(fileName)
	if trimmed == "" {
		return "file"
	}
	baseName := strings.TrimSpace(path.Base(trimmed))
	if baseName == "" || baseName == "." || baseName == "/" {
		return "file"
	}
	extension := path.Ext(baseName)
	if extension != "" {
		baseName = strings.TrimSuffix(baseName, extension)
	}
	return baseName
}

func sanitizePathSegment(rawValue string, fallback string) string {
	trimmed := strings.TrimSpace(rawValue)
	if trimmed == "" {
		return fallback
	}

	builder := strings.Builder{}
	builder.Grow(len(trimmed))
	for _, char := range trimmed {
		switch {
		case (char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '-' || char == '_' || char == '.':
			builder.WriteRune(char)
		default:
			builder.WriteByte('-')
		}
	}

	normalized := strings.Trim(builder.String(), "-._")
	if normalized == "" {
		return fallback
	}
	return normalized
}

func uploadImageToCloudflareR2(
	ctx context.Context,
	content []byte,
	contentType string,
	objectKey string,
	config service.ImageHostingConfig,
) (string, error) {
	if ctx == nil {
		return "", errors.New("request context is nil")
	}
	accountID := strings.TrimSpace(config.CloudflareR2.AccountID)
	bucket := strings.TrimSpace(config.CloudflareR2.Bucket)
	accessKeyID := strings.TrimSpace(config.CloudflareR2.AccessKeyID)
	secretAccessKey := strings.TrimSpace(config.CloudflareR2.SecretAccessKey)
	if accountID == "" || bucket == "" || accessKeyID == "" || secretAccessKey == "" {
		return "", errors.New("cloudflare r2 config is incomplete")
	}
	endpoint := resolveCloudflareR2Endpoint(accountID)
	if endpoint == "" {
		return "", errors.New("cloudflare r2 endpoint is empty")
	}

	awsConfig := aws.Config{
		Region: "auto",
		Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(
			accessKeyID,
			secretAccessKey,
			"",
		)),
		EndpointResolverWithOptions: aws.EndpointResolverWithOptionsFunc(
			func(service string, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL:               endpoint,
					SigningRegion:     "auto",
					HostnameImmutable: true,
				}, nil
			},
		),
	}
	client := s3.NewFromConfig(awsConfig, func(options *s3.Options) {
		options.UsePathStyle = true
	})
	_, putErr := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(objectKey),
		Body:        bytes.NewReader(content),
		ContentType: aws.String(contentType),
	})
	if putErr != nil {
		return "", putErr
	}

	resolvedURL := resolveObjectStoragePublicURL(config.CloudflareR2.PublicBaseURL, objectKey)
	if resolvedURL == "" {
		resolvedURL = strings.TrimRight(endpoint, "/") + "/" + strings.TrimLeft(bucket+"/"+objectKey, "/")
	}
	return resolvedURL, nil
}

func deleteImageFromCloudflareR2(
	ctx context.Context,
	objectKey string,
	config service.ImageHostingConfig,
) error {
	if ctx == nil {
		return errors.New("request context is nil")
	}
	normalizedObjectKey := strings.TrimSpace(objectKey)
	if normalizedObjectKey == "" {
		return errors.New("cloudflare r2 object key is empty")
	}
	accountID := strings.TrimSpace(config.CloudflareR2.AccountID)
	bucket := strings.TrimSpace(config.CloudflareR2.Bucket)
	accessKeyID := strings.TrimSpace(config.CloudflareR2.AccessKeyID)
	secretAccessKey := strings.TrimSpace(config.CloudflareR2.SecretAccessKey)
	if accountID == "" || bucket == "" || accessKeyID == "" || secretAccessKey == "" {
		return errors.New("cloudflare r2 config is incomplete")
	}
	endpoint := resolveCloudflareR2Endpoint(accountID)
	if endpoint == "" {
		return errors.New("cloudflare r2 endpoint is empty")
	}

	awsConfig := aws.Config{
		Region: "auto",
		Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(
			accessKeyID,
			secretAccessKey,
			"",
		)),
		EndpointResolverWithOptions: aws.EndpointResolverWithOptionsFunc(
			func(service string, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL:               endpoint,
					SigningRegion:     "auto",
					HostnameImmutable: true,
				}, nil
			},
		),
	}
	client := s3.NewFromConfig(awsConfig, func(options *s3.Options) {
		options.UsePathStyle = true
	})

	_, deleteErr := client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(normalizedObjectKey),
	})
	return deleteErr
}

func uploadImageToAliyunOSS(
	content []byte,
	contentType string,
	objectKey string,
	config service.ImageHostingConfig,
) (string, error) {
	bucket := strings.TrimSpace(config.AliyunOSS.Bucket)
	accessKeyID := strings.TrimSpace(config.AliyunOSS.AccessKeyID)
	accessKeySecret := strings.TrimSpace(config.AliyunOSS.AccessKeySecret)
	if bucket == "" || accessKeyID == "" || accessKeySecret == "" {
		return "", errors.New("aliyun oss config is incomplete")
	}

	endpoint := resolveAliyunOSSEndpoint(config.AliyunOSS.Endpoint, config.AliyunOSS.Region)
	if endpoint == "" {
		return "", errors.New("aliyun oss endpoint is empty")
	}

	client, err := oss.New(endpoint, accessKeyID, accessKeySecret)
	if err != nil {
		return "", err
	}
	bucketClient, err := client.Bucket(bucket)
	if err != nil {
		return "", err
	}

	if putErr := bucketClient.PutObject(objectKey, bytes.NewReader(content), oss.ContentType(contentType)); putErr != nil {
		return "", putErr
	}

	resolvedURL := resolveObjectStoragePublicURL(config.AliyunOSS.PublicBaseURL, objectKey)
	if resolvedURL == "" {
		baseURL := buildAliyunOSSFallbackPublicBaseURL(endpoint, bucket)
		resolvedURL = resolveObjectStoragePublicURL(baseURL, objectKey)
	}
	return resolvedURL, nil
}

func deleteImageFromAliyunOSS(
	objectKey string,
	config service.ImageHostingConfig,
) error {
	normalizedObjectKey := strings.TrimSpace(objectKey)
	if normalizedObjectKey == "" {
		return errors.New("aliyun oss object key is empty")
	}

	bucket := strings.TrimSpace(config.AliyunOSS.Bucket)
	accessKeyID := strings.TrimSpace(config.AliyunOSS.AccessKeyID)
	accessKeySecret := strings.TrimSpace(config.AliyunOSS.AccessKeySecret)
	if bucket == "" || accessKeyID == "" || accessKeySecret == "" {
		return errors.New("aliyun oss config is incomplete")
	}

	endpoint := resolveAliyunOSSEndpoint(config.AliyunOSS.Endpoint, config.AliyunOSS.Region)
	if endpoint == "" {
		return errors.New("aliyun oss endpoint is empty")
	}

	client, err := oss.New(endpoint, accessKeyID, accessKeySecret)
	if err != nil {
		return err
	}
	bucketClient, err := client.Bucket(bucket)
	if err != nil {
		return err
	}
	return bucketClient.DeleteObject(normalizedObjectKey)
}

func resolveCloudflareR2Endpoint(accountID string) string {
	normalizedAccountID := strings.TrimSpace(accountID)
	if normalizedAccountID == "" {
		return ""
	}
	if strings.HasPrefix(normalizedAccountID, "https://") || strings.HasPrefix(normalizedAccountID, "http://") {
		return strings.TrimRight(normalizedAccountID, "/")
	}
	return "https://" + normalizedAccountID + ".r2.cloudflarestorage.com"
}

func resolveAliyunOSSEndpoint(endpoint string, region string) string {
	normalizedEndpoint := strings.TrimSpace(endpoint)
	if normalizedEndpoint != "" {
		if strings.HasPrefix(normalizedEndpoint, "https://") || strings.HasPrefix(normalizedEndpoint, "http://") {
			return strings.TrimRight(normalizedEndpoint, "/")
		}
		return "https://" + strings.TrimRight(normalizedEndpoint, "/")
	}
	normalizedRegion := strings.TrimSpace(region)
	if normalizedRegion == "" {
		return ""
	}
	return "https://oss-" + normalizedRegion + ".aliyuncs.com"
}

func resolveObjectStoragePublicURL(baseURL string, objectKey string) string {
	normalizedBaseURL := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	normalizedObjectKey := strings.TrimLeft(strings.TrimSpace(objectKey), "/")
	if normalizedBaseURL == "" || normalizedObjectKey == "" {
		return ""
	}
	return normalizedBaseURL + "/" + normalizedObjectKey
}

func buildAliyunOSSFallbackPublicBaseURL(endpoint string, bucket string) string {
	normalizedEndpoint := strings.TrimSpace(endpoint)
	normalizedBucket := strings.TrimSpace(bucket)
	if normalizedEndpoint == "" || normalizedBucket == "" {
		return ""
	}
	parsedURL, err := url.Parse(normalizedEndpoint)
	if err != nil || parsedURL.Host == "" {
		return ""
	}
	host := parsedURL.Hostname()
	if host == "" {
		return ""
	}
	port := parsedURL.Port()
	scheme := parsedURL.Scheme
	if scheme == "" {
		scheme = "https"
	}
	bucketHost := host
	if !strings.HasPrefix(host, normalizedBucket+".") {
		bucketHost = normalizedBucket + "." + host
	}
	if port != "" {
		return scheme + "://" + bucketHost + ":" + port
	}
	return scheme + "://" + bucketHost
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

func resolveLocalImageAbsolutePath(rootDir string, cleanPath string) (string, error) {
	targetPath := filepath.Join(rootDir, filepath.FromSlash(cleanPath))
	targetAbsPath, err := filepath.Abs(targetPath)
	if err != nil {
		return "", err
	}
	rootAbsPath, err := filepath.Abs(rootDir)
	if err != nil {
		return "", err
	}
	if !isPathWithinRoot(rootAbsPath, targetAbsPath) {
		return "", errLocalImagePathOutOfRootDir
	}
	return targetAbsPath, nil
}
