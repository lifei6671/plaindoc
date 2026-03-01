package service

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"
)

const (
	documentImageAssetStatusActive         = "active"
	documentImageAssetStatusPendingCleanup = "pending_cleanup"
	documentImageAssetStatusDeleted        = "deleted"

	defaultDocumentImageAssetCleanupGracePeriod = 24 * time.Hour
	defaultDocumentImageAssetCleanupBatchSize   = 500
	maxDocumentImageAssetCleanupBatchSize       = 5000
	defaultDocumentImageAssetLocalRootDir       = "uploads"
)

var (
	markdownImageURLPattern = regexp.MustCompile(`!\[[^\]]*\]\((\S+?)(?:\s+"[^"]*")?\)`)
	htmlImageSrcPattern     = regexp.MustCompile(`(?i)<img[^>]*\bsrc=['"]([^'"]+)['"][^>]*>`)
)

// SyncDocumentImageAssetsInput 描述文档图片引用同步输入。
type SyncDocumentImageAssetsInput struct {
	DocumentID   string
	SpaceID      string
	ContentMD    string
	ReferencedAt time.Time
}

type documentImageReference struct {
	StorageProvider ImageHostingProvider
	ObjectKey       string
	ObjectURL       string
}

// DocumentImageAssetService 负责文档图片引用追踪与幽灵图片清理。
type DocumentImageAssetService struct {
	db                  *gorm.DB
	imageHostingService *ImageHostingService
	localRootDir        string
	cleanupGracePeriod  time.Duration
}

// NewDocumentImageAssetService 创建文档图片追踪服务。
func NewDocumentImageAssetService(
	db *gorm.DB,
	imageHostingService *ImageHostingService,
) *DocumentImageAssetService {
	return &DocumentImageAssetService{
		db:                  db,
		imageHostingService: imageHostingService,
		localRootDir:        defaultDocumentImageAssetLocalRootDir,
		cleanupGracePeriod:  defaultDocumentImageAssetCleanupGracePeriod,
	}
}

// SyncDocumentImageAssets 保存文档后同步图片引用：
// - 新引用 => active
// - 已取消引用 => pending_cleanup
func (s *DocumentImageAssetService) SyncDocumentImageAssets(
	ctx context.Context,
	input SyncDocumentImageAssetsInput,
) error {
	if s == nil || s.db == nil {
		return errors.New("document image asset service db is nil")
	}
	documentID := strings.TrimSpace(input.DocumentID)
	spaceID := strings.TrimSpace(input.SpaceID)
	if documentID == "" || spaceID == "" {
		return errors.New("document image asset sync input is invalid")
	}

	referencedAt := input.ReferencedAt.UTC()
	if referencedAt.IsZero() {
		referencedAt = time.Now().UTC()
	}

	references, err := s.extractManagedImageReferences(ctx, input.ContentMD)
	if err != nil {
		return err
	}

	type existingDocumentImageAssetRow struct {
		ID              int64  `gorm:"column:id"`
		StorageProvider string `gorm:"column:storage_provider"`
		ObjectKey       string `gorm:"column:object_key"`
	}

	existingAssets := make([]existingDocumentImageAssetRow, 0, len(references)+8)
	if err := s.db.WithContext(ctx).
		Table("document_image_assets").
		Select("id, storage_provider, object_key").
		Where("document_id = ? AND status IN ?", documentID, []string{
			documentImageAssetStatusActive,
			documentImageAssetStatusPendingCleanup,
		}).
		Find(&existingAssets).Error; err != nil {
		return err
	}

	existingByKey := make(map[string]existingDocumentImageAssetRow, len(existingAssets))
	for _, item := range existingAssets {
		identityKey := buildDocumentImageIdentityKey(item.StorageProvider, item.ObjectKey)
		if identityKey == "" {
			continue
		}
		existingByKey[identityKey] = item
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, reference := range references {
			identityKey := buildDocumentImageIdentityKey(string(reference.StorageProvider), reference.ObjectKey)
			if identityKey == "" {
				continue
			}
			if existing, exists := existingByKey[identityKey]; exists {
				if err := tx.Model(&models.DocumentImageAsset{}).
					Where("id = ?", existing.ID).
					Updates(map[string]any{
						"space_id":           spaceID,
						"object_url":         strings.TrimSpace(reference.ObjectURL),
						"status":             documentImageAssetStatusActive,
						"pending_cleanup_at": nil,
						"deleted_at":         nil,
						"last_referenced_at": referencedAt,
						"updated_at":         referencedAt,
					}).Error; err != nil {
					return err
				}
				delete(existingByKey, identityKey)
				continue
			}

			newAsset := &models.DocumentImageAsset{
				ImageAssetID:     strings.ToLower(ulid.Make().String()),
				DocumentID:       documentID,
				SpaceID:          spaceID,
				StorageProvider:  string(reference.StorageProvider),
				ObjectKey:        strings.TrimSpace(reference.ObjectKey),
				ObjectURL:        strings.TrimSpace(reference.ObjectURL),
				Status:           documentImageAssetStatusActive,
				LastReferencedAt: referencedAt,
				CreatedAt:        referencedAt,
				UpdatedAt:        referencedAt,
				PendingCleanupAt: nil,
				DeletedAt:        nil,
			}
			if err := tx.Create(newAsset).Error; err != nil {
				return err
			}
		}

		staleAssetIDs := make([]int64, 0, len(existingByKey))
		for _, item := range existingByKey {
			staleAssetIDs = append(staleAssetIDs, item.ID)
		}
		if len(staleAssetIDs) == 0 {
			return nil
		}

		return tx.Model(&models.DocumentImageAsset{}).
			Where("id IN ?", staleAssetIDs).
			Updates(map[string]any{
				"status":             documentImageAssetStatusPendingCleanup,
				"pending_cleanup_at": gorm.Expr("COALESCE(pending_cleanup_at, ?)", referencedAt),
				"updated_at":         referencedAt,
			}).Error
	})
}

// CleanupPendingDocumentImageAssets 清理待删除图片引用并在无活跃引用时删除物理对象。
// 返回值为“成功标记为 deleted 的引用行数”。
func (s *DocumentImageAssetService) CleanupPendingDocumentImageAssets(
	ctx context.Context,
	batchSize int,
) (int64, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("document image asset service db is nil")
	}
	if batchSize <= 0 {
		batchSize = defaultDocumentImageAssetCleanupBatchSize
	}
	if batchSize > maxDocumentImageAssetCleanupBatchSize {
		batchSize = maxDocumentImageAssetCleanupBatchSize
	}

	now := time.Now().UTC()
	if err := s.markDeletedDocumentReferencesPending(ctx, now); err != nil {
		return 0, err
	}

	pendingCutoff := now.Add(-s.cleanupGracePeriod)
	type pendingCandidateRow struct {
		ID              int64  `gorm:"column:id"`
		StorageProvider string `gorm:"column:storage_provider"`
		ObjectKey       string `gorm:"column:object_key"`
	}

	candidates := make([]pendingCandidateRow, 0, batchSize)
	if err := s.db.WithContext(ctx).
		Table("document_image_assets").
		Select("id, storage_provider, object_key").
		Where("status = ? AND pending_cleanup_at IS NOT NULL AND pending_cleanup_at <= ?",
			documentImageAssetStatusPendingCleanup,
			pendingCutoff,
		).
		Order("pending_cleanup_at ASC").
		Limit(batchSize).
		Find(&candidates).Error; err != nil {
		return 0, err
	}
	if len(candidates) == 0 {
		return 0, nil
	}

	config := DefaultImageHostingConfig()
	if s.imageHostingService != nil {
		loadedConfig, err := s.imageHostingService.GetConfig(ctx)
		if err != nil {
			return 0, err
		}
		config = loadedConfig
	}

	var totalDeletedRows int64
	processedObjects := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		identityKey := buildDocumentImageIdentityKey(candidate.StorageProvider, candidate.ObjectKey)
		if identityKey == "" {
			rows, err := s.markDocumentImageAssetDeletedByID(ctx, candidate.ID, now)
			if err != nil {
				return totalDeletedRows, err
			}
			totalDeletedRows += rows
			continue
		}

		if _, exists := processedObjects[identityKey]; exists {
			continue
		}

		activeRefCount, err := s.countActiveDocumentImageReferences(ctx, candidate.StorageProvider, candidate.ObjectKey)
		if err != nil {
			return totalDeletedRows, err
		}
		if activeRefCount > 0 {
			rows, err := s.markDocumentImageAssetDeletedByID(ctx, candidate.ID, now)
			if err != nil {
				return totalDeletedRows, err
			}
			totalDeletedRows += rows
			processedObjects[identityKey] = struct{}{}
			continue
		}

		if err := s.deletePhysicalObject(ctx, config, candidate.StorageProvider, candidate.ObjectKey); err != nil {
			continue
		}

		rows, err := s.markDocumentImageAssetsDeletedByObject(ctx, candidate.StorageProvider, candidate.ObjectKey, now)
		if err != nil {
			return totalDeletedRows, err
		}
		totalDeletedRows += rows
		processedObjects[identityKey] = struct{}{}
	}

	return totalDeletedRows, nil
}

func (s *DocumentImageAssetService) markDeletedDocumentReferencesPending(
	ctx context.Context,
	now time.Time,
) error {
	activeDocumentSubquery := s.db.WithContext(ctx).
		Table("documents").
		Select("document_id").
		Where("status = ? AND deleted_at IS NULL", models.EntityStatusActive)

	return s.db.WithContext(ctx).
		Model(&models.DocumentImageAsset{}).
		Where("status = ?", documentImageAssetStatusActive).
		Where("document_id NOT IN (?)", activeDocumentSubquery).
		Updates(map[string]any{
			"status":             documentImageAssetStatusPendingCleanup,
			"pending_cleanup_at": gorm.Expr("COALESCE(pending_cleanup_at, ?)", now),
			"updated_at":         now,
		}).Error
}

func (s *DocumentImageAssetService) countActiveDocumentImageReferences(
	ctx context.Context,
	storageProvider string,
	objectKey string,
) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Model(&models.DocumentImageAsset{}).
		Where("storage_provider = ? AND object_key = ? AND status = ?",
			strings.ToLower(strings.TrimSpace(storageProvider)),
			strings.TrimSpace(objectKey),
			documentImageAssetStatusActive,
		).
		Count(&count).Error
	return count, err
}

func (s *DocumentImageAssetService) markDocumentImageAssetDeletedByID(
	ctx context.Context,
	id int64,
	now time.Time,
) (int64, error) {
	if id <= 0 {
		return 0, nil
	}
	updateTx := s.db.WithContext(ctx).
		Model(&models.DocumentImageAsset{}).
		Where("id = ? AND status <> ?", id, documentImageAssetStatusDeleted).
		Updates(map[string]any{
			"status":     documentImageAssetStatusDeleted,
			"deleted_at": now,
			"updated_at": now,
		})
	return updateTx.RowsAffected, updateTx.Error
}

func (s *DocumentImageAssetService) markDocumentImageAssetsDeletedByObject(
	ctx context.Context,
	storageProvider string,
	objectKey string,
	now time.Time,
) (int64, error) {
	updateTx := s.db.WithContext(ctx).
		Model(&models.DocumentImageAsset{}).
		Where("storage_provider = ? AND object_key = ? AND status <> ?",
			strings.ToLower(strings.TrimSpace(storageProvider)),
			strings.TrimSpace(objectKey),
			documentImageAssetStatusDeleted,
		).
		Updates(map[string]any{
			"status":     documentImageAssetStatusDeleted,
			"deleted_at": now,
			"updated_at": now,
		})
	return updateTx.RowsAffected, updateTx.Error
}

func (s *DocumentImageAssetService) deletePhysicalObject(
	ctx context.Context,
	config ImageHostingConfig,
	storageProvider string,
	objectKey string,
) error {
	normalizedProvider := normalizeImageHostingProvider(storageProvider)
	if normalizedProvider == "" {
		normalizedProvider = ImageHostingProviderLocal
	}
	normalizedObjectKey := strings.TrimSpace(objectKey)
	if normalizedObjectKey == "" {
		return errors.New("document image object key is empty")
	}

	switch normalizedProvider {
	case ImageHostingProviderLocal:
		targetPath, err := s.resolveLocalTargetPath(normalizedObjectKey)
		if err != nil {
			return err
		}
		if removeErr := os.Remove(targetPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return removeErr
		}
		return nil
	case ImageHostingProviderCloudflareR2:
		return s.deleteCloudflareR2Object(ctx, normalizedObjectKey, config)
	case ImageHostingProviderAliyunOSS:
		return s.deleteAliyunOSSObject(normalizedObjectKey, config)
	default:
		return errors.New("unsupported image storage provider")
	}
}

func (s *DocumentImageAssetService) resolveLocalTargetPath(objectKey string) (string, error) {
	normalizedObjectKey := strings.TrimSpace(strings.TrimPrefix(objectKey, "/"))
	if normalizedObjectKey == "" {
		return "", errors.New("object key is empty")
	}
	cleanObjectKey := path.Clean(normalizedObjectKey)
	if cleanObjectKey == "." || cleanObjectKey == "/" || strings.HasPrefix(cleanObjectKey, "../") {
		return "", errors.New("object key is invalid")
	}
	localRootDir := strings.TrimSpace(s.localRootDir)
	if localRootDir == "" {
		localRootDir = defaultDocumentImageAssetLocalRootDir
	}
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

func (s *DocumentImageAssetService) deleteCloudflareR2Object(
	ctx context.Context,
	objectKey string,
	config ImageHostingConfig,
) error {
	if ctx == nil {
		return errors.New("request context is nil")
	}
	accountID := strings.TrimSpace(config.CloudflareR2.AccountID)
	bucket := strings.TrimSpace(config.CloudflareR2.Bucket)
	accessKeyID := strings.TrimSpace(config.CloudflareR2.AccessKeyID)
	secretAccessKey := strings.TrimSpace(config.CloudflareR2.SecretAccessKey)
	if accountID == "" || bucket == "" || accessKeyID == "" || secretAccessKey == "" {
		return errors.New("cloudflare r2 config is incomplete")
	}

	endpoint := resolveImageHostingCloudflareR2Endpoint(accountID)
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
		Key:    aws.String(strings.TrimSpace(objectKey)),
	})
	return deleteErr
}

func (s *DocumentImageAssetService) deleteAliyunOSSObject(
	objectKey string,
	config ImageHostingConfig,
) error {
	bucket := strings.TrimSpace(config.AliyunOSS.Bucket)
	accessKeyID := strings.TrimSpace(config.AliyunOSS.AccessKeyID)
	accessKeySecret := strings.TrimSpace(config.AliyunOSS.AccessKeySecret)
	if bucket == "" || accessKeyID == "" || accessKeySecret == "" {
		return errors.New("aliyun oss config is incomplete")
	}

	endpoint := resolveImageHostingAliyunOSSEndpoint(config.AliyunOSS.Endpoint, config.AliyunOSS.Region)
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
	return bucketClient.DeleteObject(strings.TrimSpace(objectKey))
}

func (s *DocumentImageAssetService) extractManagedImageReferences(
	ctx context.Context,
	contentMD string,
) ([]documentImageReference, error) {
	content := strings.TrimSpace(contentMD)
	if content == "" {
		return []documentImageReference{}, nil
	}

	config := DefaultImageHostingConfig()
	if s.imageHostingService != nil {
		loadedConfig, err := s.imageHostingService.GetConfig(ctx)
		if err != nil {
			return nil, err
		}
		config = loadedConfig
	}

	rawURLs := extractImageURLsFromMarkdownAndHTML(content)
	if len(rawURLs) == 0 {
		return []documentImageReference{}, nil
	}

	references := make([]documentImageReference, 0, len(rawURLs))
	seen := make(map[string]struct{}, len(rawURLs))
	for _, rawURL := range rawURLs {
		resolvedRef, ok := resolveManagedImageReference(rawURL, config)
		if !ok {
			continue
		}
		identityKey := buildDocumentImageIdentityKey(string(resolvedRef.StorageProvider), resolvedRef.ObjectKey)
		if identityKey == "" {
			continue
		}
		if _, exists := seen[identityKey]; exists {
			continue
		}
		seen[identityKey] = struct{}{}
		references = append(references, resolvedRef)
	}
	return references, nil
}

func extractImageURLsFromMarkdownAndHTML(content string) []string {
	imageURLs := make([]string, 0)
	seen := make(map[string]struct{})

	markdownMatches := markdownImageURLPattern.FindAllStringSubmatch(content, -1)
	for _, match := range markdownMatches {
		if len(match) < 2 {
			continue
		}
		rawURL := strings.TrimSpace(match[1])
		if rawURL == "" {
			continue
		}
		if _, exists := seen[rawURL]; exists {
			continue
		}
		seen[rawURL] = struct{}{}
		imageURLs = append(imageURLs, rawURL)
	}

	htmlMatches := htmlImageSrcPattern.FindAllStringSubmatch(content, -1)
	for _, match := range htmlMatches {
		if len(match) < 2 {
			continue
		}
		rawURL := strings.TrimSpace(match[1])
		if rawURL == "" {
			continue
		}
		if _, exists := seen[rawURL]; exists {
			continue
		}
		seen[rawURL] = struct{}{}
		imageURLs = append(imageURLs, rawURL)
	}

	return imageURLs
}

func resolveManagedImageReference(
	rawURL string,
	config ImageHostingConfig,
) (documentImageReference, bool) {
	normalizedURL := strings.TrimSpace(rawURL)
	if normalizedURL == "" {
		return documentImageReference{}, false
	}

	localBases := []string{
		config.Local.PublicBaseURL,
		"/uploads",
		"/api/uploads",
		"/uploads/local",
		"/api/uploads/local",
	}
	for _, base := range localBases {
		objectKey := extractObjectKeyByBaseURL(normalizedURL, base)
		if objectKey == "" {
			continue
		}
		objectKey = strings.TrimPrefix(objectKey, "local/")
		objectKey = strings.TrimSpace(strings.TrimPrefix(objectKey, "/"))
		if objectKey == "" {
			continue
		}
		return documentImageReference{
			StorageProvider: ImageHostingProviderLocal,
			ObjectKey:       objectKey,
			ObjectURL:       resolveDocumentImageLocalPublicURL(config.Local.PublicBaseURL, objectKey),
		}, true
	}

	cloudflareBase := strings.TrimSpace(config.CloudflareR2.PublicBaseURL)
	if cloudflareBase != "" {
		if objectKey := extractObjectKeyByBaseURL(normalizedURL, cloudflareBase); objectKey != "" {
			return documentImageReference{
				StorageProvider: ImageHostingProviderCloudflareR2,
				ObjectKey:       objectKey,
				ObjectURL:       resolveDocumentImageObjectStoragePublicURL(cloudflareBase, objectKey),
			}, true
		}
	}

	aliyunBase := strings.TrimSpace(config.AliyunOSS.PublicBaseURL)
	if aliyunBase != "" {
		if objectKey := extractObjectKeyByBaseURL(normalizedURL, aliyunBase); objectKey != "" {
			return documentImageReference{
				StorageProvider: ImageHostingProviderAliyunOSS,
				ObjectKey:       objectKey,
				ObjectURL:       resolveDocumentImageObjectStoragePublicURL(aliyunBase, objectKey),
			}, true
		}
	}

	return documentImageReference{}, false
}

func extractObjectKeyByBaseURL(rawURL string, baseURL string) string {
	normalizedBaseURL := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	normalizedRawURL := strings.TrimSpace(rawURL)
	if normalizedBaseURL == "" || normalizedRawURL == "" {
		return ""
	}

	rawParsedURL, rawParseErr := url.Parse(normalizedRawURL)
	if rawParseErr != nil {
		return ""
	}

	// 绝对地址匹配：scheme + host + path 前缀。
	if strings.HasPrefix(strings.ToLower(normalizedBaseURL), "http://") ||
		strings.HasPrefix(strings.ToLower(normalizedBaseURL), "https://") {
		baseParsedURL, baseParseErr := url.Parse(normalizedBaseURL)
		if baseParseErr != nil || baseParsedURL.Host == "" {
			return ""
		}
		if !strings.EqualFold(rawParsedURL.Scheme, baseParsedURL.Scheme) ||
			!strings.EqualFold(rawParsedURL.Host, baseParsedURL.Host) {
			return ""
		}
		basePath := strings.TrimRight(strings.TrimSpace(baseParsedURL.Path), "/")
		rawPath := strings.TrimSpace(rawParsedURL.Path)
		if basePath != "" {
			if !strings.HasPrefix(rawPath, basePath+"/") {
				return ""
			}
			rawPath = strings.TrimPrefix(rawPath, basePath)
		}
		return normalizeObjectKeyFromPath(rawPath)
	}

	// 相对路径匹配：仅对 path 做前缀判断。
	basePath := normalizedBaseURL
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	rawPath := strings.TrimSpace(rawParsedURL.Path)
	if rawPath == "" {
		rawPath = normalizedRawURL
	}
	if !strings.HasPrefix(rawPath, "/") {
		rawPath = "/" + rawPath
	}
	if !strings.HasPrefix(rawPath, basePath+"/") {
		return ""
	}
	rawPath = strings.TrimPrefix(rawPath, basePath)
	return normalizeObjectKeyFromPath(rawPath)
}

func normalizeObjectKeyFromPath(rawPath string) string {
	normalizedPath := strings.TrimSpace(rawPath)
	normalizedPath = strings.TrimLeft(normalizedPath, "/")
	if normalizedPath == "" {
		return ""
	}
	if decodedPath, err := url.PathUnescape(normalizedPath); err == nil {
		normalizedPath = decodedPath
	}
	cleanPath := path.Clean(normalizedPath)
	if cleanPath == "." || cleanPath == "/" || strings.HasPrefix(cleanPath, "../") {
		return ""
	}
	return strings.TrimLeft(cleanPath, "/")
}

func buildDocumentImageIdentityKey(storageProvider string, objectKey string) string {
	normalizedProvider := strings.ToLower(strings.TrimSpace(storageProvider))
	normalizedObjectKey := strings.TrimSpace(objectKey)
	if normalizedProvider == "" || normalizedObjectKey == "" {
		return ""
	}
	return normalizedProvider + "::" + normalizedObjectKey
}

func resolveDocumentImageLocalPublicURL(baseURL string, objectKey string) string {
	base := strings.TrimSpace(baseURL)
	if base == "" {
		base = "/uploads"
	}
	if strings.HasPrefix(base, "/api/uploads/") {
		base = strings.TrimPrefix(base, "/api")
	}
	if strings.EqualFold(strings.TrimRight(base, "/"), "/api/uploads/local") {
		base = "/uploads"
	}
	if strings.EqualFold(strings.TrimRight(base, "/"), "/uploads/local") {
		base = "/uploads"
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(strings.TrimSpace(objectKey), "/")
}

func resolveDocumentImageObjectStoragePublicURL(baseURL string, objectKey string) string {
	normalizedBaseURL := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	normalizedObjectKey := strings.TrimLeft(strings.TrimSpace(objectKey), "/")
	if normalizedBaseURL == "" || normalizedObjectKey == "" {
		return ""
	}
	return normalizedBaseURL + "/" + normalizedObjectKey
}
