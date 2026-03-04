package service

import (
	"context"
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/errcode"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

const (
	defaultAdminDocumentImageAssetPage     = 1
	defaultAdminDocumentImageAssetPageSize = 20
	maxAdminDocumentImageAssetPageSize     = 100
	defaultAdminLocalImageRootDir          = "uploads"
	defaultAdminImageDeleteRefPreviewLimit = 10
)

// AdminDocumentImageAssetStatusFilter 管理后台文档图片资源状态过滤条件。
type AdminDocumentImageAssetStatusFilter string

const (
	AdminDocumentImageAssetStatusFilterAll            AdminDocumentImageAssetStatusFilter = "all"
	AdminDocumentImageAssetStatusFilterActive         AdminDocumentImageAssetStatusFilter = "active"
	AdminDocumentImageAssetStatusFilterPendingCleanup AdminDocumentImageAssetStatusFilter = "pending_cleanup"
	AdminDocumentImageAssetStatusFilterDeleted        AdminDocumentImageAssetStatusFilter = "deleted"
)

// AdminDocumentImageAssetStorageProviderFilter 管理后台文档图片资源存储提供商过滤条件。
type AdminDocumentImageAssetStorageProviderFilter string

const (
	AdminDocumentImageAssetStorageProviderFilterAll          AdminDocumentImageAssetStorageProviderFilter = "all"
	AdminDocumentImageAssetStorageProviderFilterLocal        AdminDocumentImageAssetStorageProviderFilter = "local"
	AdminDocumentImageAssetStorageProviderFilterCloudflareR2 AdminDocumentImageAssetStorageProviderFilter = "cloudflare-r2"
	AdminDocumentImageAssetStorageProviderFilterAliyunOSS    AdminDocumentImageAssetStorageProviderFilter = "aliyun-oss"
)

// AdminDocumentImageAssetRecord 后台文档图片资源列表项。
type AdminDocumentImageAssetRecord struct {
	ImageAssetID     string
	DocumentID       string
	DocumentRouteKey string
	DocumentTitle    string
	DocumentStatus   models.EntityStatus
	SpaceID          string
	SpaceName        string
	SpaceOwnerUserID string
	SpaceOwnerName   string
	SpaceOwnerEmail  string
	StorageProvider  string
	ObjectKey        string
	ObjectURL        string
	Status           string
	LastReferencedAt time.Time
	PendingCleanupAt *time.Time
	DeletedAt        *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// ListAdminDocumentImageAssetsInput 后台文档图片资源列表查询参数。
type ListAdminDocumentImageAssetsInput struct {
	ActorUserID           string
	Keyword               string
	SpaceID               string
	DocumentID            string
	StatusFilter          AdminDocumentImageAssetStatusFilter
	StorageProviderFilter AdminDocumentImageAssetStorageProviderFilter
	Page                  int
	PageSize              int
}

// ListAdminDocumentImageAssetsResult 后台文档图片资源列表返回结果。
type ListAdminDocumentImageAssetsResult struct {
	Items    []AdminDocumentImageAssetRecord
	Page     int
	PageSize int
	Total    int64
}

// DeleteAdminDocumentImageAssetInput 后台文档图片删除参数。
type DeleteAdminDocumentImageAssetInput struct {
	ActorUserID                string
	ImageAssetID               string
	PhysicalDelete             bool
	ForcePhysicalDeleteOnShare bool
	RequestID                  string
}

// DeleteAdminDocumentImageAssetReference 表示共享图片对象被哪些文档引用。
type DeleteAdminDocumentImageAssetReference struct {
	ImageAssetID  string
	DocumentID    string
	DocumentTitle string
	SpaceID       string
	SpaceName     string
}

// DeleteAdminDocumentImageAssetResult 描述后台删除图片执行结果。
type DeleteAdminDocumentImageAssetResult struct {
	ImageAssetID            string
	DocumentID              string
	SpaceID                 string
	PhysicalDeleteRequested bool
	PhysicalDeleteExecuted  bool
	SoftDeleted             bool
	HardDeleted             bool
	SharedReferenceCount    int64
	SharedReferences        []DeleteAdminDocumentImageAssetReference
	ConfirmationRequired    bool
	ConfirmationReason      string
	PhysicalDeleteError     string
}

// AdminDocumentImageAssetService 封装文档图片后台治理业务。
type AdminDocumentImageAssetService struct {
	documentImageAssetRepo repository.DocumentImageAssetRepository
	adminAccessService     *AdminAccessService
	adminAuditService      *AdminAuditService
	imageHostingService    *ImageHostingService
	localImageRootDir      string
}

// NewAdminDocumentImageAssetService 创建后台文档图片资源管理服务。
func NewAdminDocumentImageAssetService(
	documentImageAssetRepo repository.DocumentImageAssetRepository,
	adminAccessService *AdminAccessService,
	adminAuditService *AdminAuditService,
	imageHostingService *ImageHostingService,
) *AdminDocumentImageAssetService {
	return &AdminDocumentImageAssetService{
		documentImageAssetRepo: documentImageAssetRepo,
		adminAccessService:     adminAccessService,
		adminAuditService:      adminAuditService,
		imageHostingService:    imageHostingService,
		localImageRootDir:      defaultAdminLocalImageRootDir,
	}
}

// ListImageAssets 查询后台文档图片资源列表。
func (s *AdminDocumentImageAssetService) ListImageAssets(
	ctx context.Context,
	input ListAdminDocumentImageAssetsInput,
) (result ListAdminDocumentImageAssetsResult, err error) {
	defer func() {
		err = errcode.MapAdminDocumentImageAssetError(err)
	}()

	if s == nil || s.documentImageAssetRepo == nil || s.adminAccessService == nil {
		return ListAdminDocumentImageAssetsResult{}, errors.New("admin document image asset service dependencies are nil")
	}

	actorUserID := strings.TrimSpace(input.ActorUserID)
	if actorUserID == "" {
		return ListAdminDocumentImageAssetsResult{}, errcode.ErrAdminForbidden
	}

	restrictToScopes, err := s.resolveScopeRestriction(ctx, actorUserID)
	if err != nil {
		return ListAdminDocumentImageAssetsResult{}, err
	}

	statuses, err := resolveAdminDocumentImageAssetStatuses(input.StatusFilter)
	if err != nil {
		return ListAdminDocumentImageAssetsResult{}, err
	}
	storageProviders, err := resolveAdminDocumentImageAssetStorageProviders(input.StorageProviderFilter)
	if err != nil {
		return ListAdminDocumentImageAssetsResult{}, err
	}

	page, pageSize := normalizeAdminDocumentImageAssetPagination(input.Page, input.PageSize)
	records, total, err := s.documentImageAssetRepo.ListForAdmin(ctx, repository.ListAdminDocumentImageAssetsParams{
		ActorUserID:      actorUserID,
		RestrictToScopes: restrictToScopes,
		Keyword:          strings.TrimSpace(input.Keyword),
		SpaceID:          strings.TrimSpace(input.SpaceID),
		DocumentID:       strings.TrimSpace(input.DocumentID),
		Statuses:         statuses,
		StorageProviders: storageProviders,
		Limit:            pageSize,
		Offset:           (page - 1) * pageSize,
	})
	if err != nil {
		return ListAdminDocumentImageAssetsResult{}, err
	}

	items := make([]AdminDocumentImageAssetRecord, 0, len(records))
	for _, record := range records {
		items = append(items, mapAdminDocumentImageAssetRecord(record))
	}

	return ListAdminDocumentImageAssetsResult{
		Items:    items,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

// DeleteImageAsset 删除后台目标图片资源。
// - PhysicalDelete=false: 仅逻辑删除（status=deleted）
// - PhysicalDelete=true: 物理删除（删除记录；仅无活跃引用时删除物理文件）
func (s *AdminDocumentImageAssetService) DeleteImageAsset(
	ctx context.Context,
	input DeleteAdminDocumentImageAssetInput,
) (result DeleteAdminDocumentImageAssetResult, err error) {
	defer func() {
		err = errcode.MapAdminDocumentImageAssetError(err)
	}()

	_ = input.RequestID

	if s == nil || s.documentImageAssetRepo == nil || s.adminAccessService == nil {
		return DeleteAdminDocumentImageAssetResult{}, errors.New("admin document image asset service dependencies are nil")
	}

	actorUserID := strings.TrimSpace(input.ActorUserID)
	if actorUserID == "" {
		return DeleteAdminDocumentImageAssetResult{}, errcode.ErrAdminForbidden
	}
	imageAssetID := strings.TrimSpace(input.ImageAssetID)
	if imageAssetID == "" {
		return DeleteAdminDocumentImageAssetResult{}, errcode.ErrAdminDocumentImageAssetInvalidImageAssetID
	}

	imageAsset, err := s.documentImageAssetRepo.GetByImageAssetID(ctx, imageAssetID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return DeleteAdminDocumentImageAssetResult{}, errcode.ErrAdminDocumentImageAssetNotFound
		}
		return DeleteAdminDocumentImageAssetResult{}, err
	}

	canManage, err := s.adminAccessService.CanManageSpace(ctx, actorUserID, strings.TrimSpace(imageAsset.SpaceID))
	if err != nil {
		return DeleteAdminDocumentImageAssetResult{}, err
	}
	if !canManage {
		return DeleteAdminDocumentImageAssetResult{}, errcode.ErrAdminForbidden
	}

	result = DeleteAdminDocumentImageAssetResult{
		ImageAssetID:            imageAssetID,
		DocumentID:              strings.TrimSpace(imageAsset.DocumentID),
		SpaceID:                 strings.TrimSpace(imageAsset.SpaceID),
		PhysicalDeleteRequested: input.PhysicalDelete,
	}

	beforeStatus := normalizeAdminDocumentImageAssetRecordStatus(imageAsset.Status)
	afterStatus := any(beforeStatus)

	physicalDeleteExecuted := false
	if input.PhysicalDelete {
		normalizedProvider := strings.TrimSpace(imageAsset.StorageProvider)
		normalizedObjectKey := strings.TrimSpace(imageAsset.ObjectKey)
		if normalizedProvider == "" || normalizedObjectKey == "" {
			return DeleteAdminDocumentImageAssetResult{}, errors.New("image asset object key is empty")
		}

		activeRefCount, countErr := s.documentImageAssetRepo.CountActiveReferencesByObject(
			ctx,
			normalizedProvider,
			normalizedObjectKey,
		)
		if countErr != nil {
			return DeleteAdminDocumentImageAssetResult{}, countErr
		}

		otherActiveRefCount := activeRefCount
		if beforeStatus == "active" && otherActiveRefCount > 0 {
			otherActiveRefCount--
		}
		result.SharedReferenceCount = otherActiveRefCount

		if otherActiveRefCount > 0 {
			references, referenceErr := s.documentImageAssetRepo.ListActiveReferencesByObject(
				ctx,
				normalizedProvider,
				normalizedObjectKey,
				defaultAdminImageDeleteRefPreviewLimit,
			)
			if referenceErr != nil {
				return DeleteAdminDocumentImageAssetResult{}, referenceErr
			}
			result.SharedReferences = mapDeleteAdminDocumentImageAssetReferences(references, imageAssetID)
			if !input.ForcePhysicalDeleteOnShare {
				result.ConfirmationRequired = true
				result.ConfirmationReason = "当前图片对象被多个文档引用，确认后仅删除当前图片记录，不会删除物理文件。"
				return result, nil
			}
		}

		hardDeleted, hardDeleteErr := s.documentImageAssetRepo.HardDelete(ctx, imageAssetID)
		if hardDeleteErr != nil {
			return DeleteAdminDocumentImageAssetResult{}, hardDeleteErr
		}
		if !hardDeleted {
			return DeleteAdminDocumentImageAssetResult{}, errcode.ErrAdminDocumentImageAssetNotFound
		}
		result.HardDeleted = true

		remainingRefCount, remainingCountErr := s.documentImageAssetRepo.CountActiveReferencesByObject(
			ctx,
			normalizedProvider,
			normalizedObjectKey,
		)
		if remainingCountErr != nil {
			return DeleteAdminDocumentImageAssetResult{}, remainingCountErr
		}
		result.SharedReferenceCount = remainingRefCount

		if remainingRefCount > 0 {
			result.PhysicalDeleteError = "文件仍存在引用，未执行物理文件删除"
		} else {
			deletePhysicalErr := s.tryPhysicalDeleteObject(ctx, normalizedProvider, normalizedObjectKey)
			if deletePhysicalErr != nil {
				result.PhysicalDeleteError = deletePhysicalErr.Error()
			} else {
				physicalDeleteExecuted = true
			}
		}
		afterStatus = "hard_deleted"
	} else if beforeStatus != "deleted" {
		deleted, deleteErr := s.documentImageAssetRepo.SoftDelete(ctx, imageAssetID, time.Now().UTC())
		if deleteErr != nil {
			return DeleteAdminDocumentImageAssetResult{}, deleteErr
		}
		if !deleted {
			return DeleteAdminDocumentImageAssetResult{}, errcode.ErrAdminDocumentImageAssetNotFound
		}
		result.SoftDeleted = true
		afterStatus = "deleted"
	}
	result.PhysicalDeleteExecuted = physicalDeleteExecuted

	if err := s.recordImageAssetAudit(ctx, RecordAdminAuditInput{
		Module:     AdminAuditModuleDocument,
		Action:     AdminAuditActionDelete,
		TargetType: "document_image",
		TargetID:   imageAssetID,
		Summary:    "document image asset deleted: " + imageAssetID,
		Detail: map[string]any{
			"documentId":             strings.TrimSpace(imageAsset.DocumentID),
			"spaceId":                strings.TrimSpace(imageAsset.SpaceID),
			"storageProvider":        strings.TrimSpace(imageAsset.StorageProvider),
			"objectKey":              strings.TrimSpace(imageAsset.ObjectKey),
			"statusBefore":           beforeStatus,
			"statusAfter":            afterStatus,
			"physicalDelete":         input.PhysicalDelete,
			"physicalDeleteExecuted": physicalDeleteExecuted,
			"physicalDeleteError":    strings.TrimSpace(result.PhysicalDeleteError),
			"sharedReferenceCount":   result.SharedReferenceCount,
			"confirmationRequired":   result.ConfirmationRequired,
		},
	}); err != nil {
		return DeleteAdminDocumentImageAssetResult{}, err
	}

	return result, nil
}

func (s *AdminDocumentImageAssetService) resolveScopeRestriction(ctx context.Context, actorUserID string) (bool, error) {
	isPlatformAdmin, err := s.adminAccessService.IsPlatformAdmin(ctx, actorUserID)
	if err != nil {
		return false, err
	}
	if isPlatformAdmin {
		return false, nil
	}

	isSpaceAdmin, err := s.adminAccessService.IsSpaceAdmin(ctx, actorUserID)
	if err != nil {
		return false, err
	}
	if !isSpaceAdmin {
		return false, errcode.ErrAdminForbidden
	}
	return true, nil
}

func (s *AdminDocumentImageAssetService) tryPhysicalDeleteObject(
	ctx context.Context,
	storageProvider string,
	objectKey string,
) error {
	normalizedProvider := normalizeImageHostingProvider(storageProvider)
	if normalizedProvider == "" {
		normalizedProvider = ImageHostingProviderLocal
	}
	normalizedObjectKey := strings.TrimSpace(objectKey)
	if normalizedObjectKey == "" {
		return errors.New("image object key is empty")
	}

	switch normalizedProvider {
	case ImageHostingProviderLocal:
		targetPath, err := s.resolveLocalImageTargetPath(normalizedObjectKey)
		if err != nil {
			return err
		}
		if removeErr := os.Remove(targetPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return removeErr
		}
		return nil
	case ImageHostingProviderCloudflareR2:
		config, configErr := s.loadImageHostingConfig(ctx)
		if configErr != nil {
			return configErr
		}
		return s.deleteImageObjectFromCloudflareR2(ctx, normalizedObjectKey, config)
	case ImageHostingProviderAliyunOSS:
		config, configErr := s.loadImageHostingConfig(ctx)
		if configErr != nil {
			return configErr
		}
		return s.deleteImageObjectFromAliyunOSS(normalizedObjectKey, config)
	default:
		return errors.New("unsupported image storage provider")
	}
}

func (s *AdminDocumentImageAssetService) loadImageHostingConfig(ctx context.Context) (ImageHostingConfig, error) {
	if s == nil || s.imageHostingService == nil {
		return ImageHostingConfig{}, errors.New("image hosting service is nil")
	}
	return s.imageHostingService.GetConfig(ctx)
}

func (s *AdminDocumentImageAssetService) deleteImageObjectFromCloudflareR2(
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

func (s *AdminDocumentImageAssetService) deleteImageObjectFromAliyunOSS(
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

func (s *AdminDocumentImageAssetService) resolveLocalImageTargetPath(objectKey string) (string, error) {
	normalizedObjectKey := strings.TrimSpace(strings.TrimPrefix(objectKey, "/"))
	if normalizedObjectKey == "" {
		return "", errors.New("object key is empty")
	}
	cleanObjectKey := path.Clean(normalizedObjectKey)
	if cleanObjectKey == "." || cleanObjectKey == "/" || strings.HasPrefix(cleanObjectKey, "../") {
		return "", errors.New("object key is invalid")
	}

	localRootDir := strings.TrimSpace(s.localImageRootDir)
	if localRootDir == "" {
		localRootDir = defaultAdminLocalImageRootDir
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

func (s *AdminDocumentImageAssetService) recordImageAssetAudit(ctx context.Context, input RecordAdminAuditInput) error {
	if s == nil || s.adminAuditService == nil {
		return nil
	}
	return s.adminAuditService.Record(ctx, input)
}

func mapDeleteAdminDocumentImageAssetReferences(
	records []repository.DocumentImageAssetReferenceRecord,
	currentImageAssetID string,
) []DeleteAdminDocumentImageAssetReference {
	if len(records) == 0 {
		return []DeleteAdminDocumentImageAssetReference{}
	}

	items := make([]DeleteAdminDocumentImageAssetReference, 0, len(records))
	for _, record := range records {
		if strings.TrimSpace(record.ImageAssetID) == strings.TrimSpace(currentImageAssetID) {
			continue
		}
		items = append(items, DeleteAdminDocumentImageAssetReference{
			ImageAssetID:  strings.TrimSpace(record.ImageAssetID),
			DocumentID:    strings.TrimSpace(record.DocumentID),
			DocumentTitle: strings.TrimSpace(record.DocumentTitle),
			SpaceID:       strings.TrimSpace(record.SpaceID),
			SpaceName:     strings.TrimSpace(record.SpaceName),
		})
	}
	return items
}

func resolveAdminDocumentImageAssetStatuses(
	filter AdminDocumentImageAssetStatusFilter,
) ([]string, error) {
	switch normalizeAdminDocumentImageAssetStatusFilter(filter) {
	case "":
		return []string{"active", "pending_cleanup", "deleted"}, nil
	case AdminDocumentImageAssetStatusFilterAll:
		return []string{"active", "pending_cleanup", "deleted"}, nil
	case AdminDocumentImageAssetStatusFilterActive:
		return []string{"active"}, nil
	case AdminDocumentImageAssetStatusFilterPendingCleanup:
		return []string{"pending_cleanup"}, nil
	case AdminDocumentImageAssetStatusFilterDeleted:
		return []string{"deleted"}, nil
	default:
		return nil, errcode.ErrAdminDocumentImageAssetInvalidStatusFilter
	}
}

func resolveAdminDocumentImageAssetStorageProviders(
	filter AdminDocumentImageAssetStorageProviderFilter,
) ([]string, error) {
	switch normalizeAdminDocumentImageAssetStorageProviderFilter(filter) {
	case "", AdminDocumentImageAssetStorageProviderFilterAll:
		return []string{"local", "cloudflare-r2", "aliyun-oss"}, nil
	case AdminDocumentImageAssetStorageProviderFilterLocal:
		return []string{"local"}, nil
	case AdminDocumentImageAssetStorageProviderFilterCloudflareR2:
		return []string{"cloudflare-r2"}, nil
	case AdminDocumentImageAssetStorageProviderFilterAliyunOSS:
		return []string{"aliyun-oss"}, nil
	default:
		return nil, errcode.ErrAdminDocumentImageAssetInvalidStorageProviderFilter
	}
}

func normalizeAdminDocumentImageAssetStatusFilter(
	filter AdminDocumentImageAssetStatusFilter,
) AdminDocumentImageAssetStatusFilter {
	value := strings.ToLower(strings.TrimSpace(string(filter)))
	return AdminDocumentImageAssetStatusFilter(value)
}

func normalizeAdminDocumentImageAssetStorageProviderFilter(
	filter AdminDocumentImageAssetStorageProviderFilter,
) AdminDocumentImageAssetStorageProviderFilter {
	value := strings.ToLower(strings.TrimSpace(string(filter)))
	return AdminDocumentImageAssetStorageProviderFilter(value)
}

func normalizeAdminDocumentImageAssetPagination(page int, pageSize int) (int, int) {
	normalizedPage := page
	if normalizedPage <= 0 {
		normalizedPage = defaultAdminDocumentImageAssetPage
	}

	normalizedPageSize := pageSize
	if normalizedPageSize <= 0 {
		normalizedPageSize = defaultAdminDocumentImageAssetPageSize
	}
	if normalizedPageSize > maxAdminDocumentImageAssetPageSize {
		normalizedPageSize = maxAdminDocumentImageAssetPageSize
	}
	return normalizedPage, normalizedPageSize
}

func normalizeAdminDocumentImageAssetRecordStatus(rawStatus string) string {
	status := strings.ToLower(strings.TrimSpace(rawStatus))
	switch status {
	case "active", "pending_cleanup", "deleted":
		return status
	default:
		return "active"
	}
}

func mapAdminDocumentImageAssetRecord(
	record repository.AdminDocumentImageAssetListRecord,
) AdminDocumentImageAssetRecord {
	imageAsset := record.ImageAsset
	documentID := strings.TrimSpace(imageAsset.DocumentID)
	documentRouteKey := strings.TrimSpace(record.DocumentRouteKey)
	if documentRouteKey == "" {
		documentRouteKey = documentID
	}
	documentStatus := record.DocumentStatus
	if !models.IsValidEntityStatus(documentStatus) {
		documentStatus = models.EntityStatusActive
	}

	return AdminDocumentImageAssetRecord{
		ImageAssetID:     strings.TrimSpace(imageAsset.ImageAssetID),
		DocumentID:       documentID,
		DocumentRouteKey: documentRouteKey,
		DocumentTitle:    strings.TrimSpace(record.DocumentTitle),
		DocumentStatus:   documentStatus,
		SpaceID:          strings.TrimSpace(imageAsset.SpaceID),
		SpaceName:        strings.TrimSpace(record.SpaceName),
		SpaceOwnerUserID: strings.TrimSpace(record.SpaceOwnerID),
		SpaceOwnerName:   strings.TrimSpace(record.SpaceOwnerName),
		SpaceOwnerEmail:  strings.TrimSpace(record.SpaceOwnerEmail),
		StorageProvider:  strings.TrimSpace(imageAsset.StorageProvider),
		ObjectKey:        strings.TrimSpace(imageAsset.ObjectKey),
		ObjectURL:        strings.TrimSpace(imageAsset.ObjectURL),
		Status:           normalizeAdminDocumentImageAssetRecordStatus(imageAsset.Status),
		LastReferencedAt: imageAsset.LastReferencedAt,
		PendingCleanupAt: imageAsset.PendingCleanupAt,
		DeletedAt:        imageAsset.DeletedAt,
		CreatedAt:        imageAsset.CreatedAt,
		UpdatedAt:        imageAsset.UpdatedAt,
	}
}
