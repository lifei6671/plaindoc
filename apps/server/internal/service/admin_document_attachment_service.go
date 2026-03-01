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
	defaultAdminDocumentAttachmentPage     = 1
	defaultAdminDocumentAttachmentPageSize = 20
	maxAdminDocumentAttachmentPageSize     = 100
	defaultAdminLocalAttachmentRootDir     = "uploads/local"
	defaultAdminDeleteRefPreviewLimit      = 10
)

// AdminDocumentAttachmentStatusFilter 管理后台文档附件状态过滤条件。
type AdminDocumentAttachmentStatusFilter string

const (
	AdminDocumentAttachmentStatusFilterAll     AdminDocumentAttachmentStatusFilter = "all"
	AdminDocumentAttachmentStatusFilterActive  AdminDocumentAttachmentStatusFilter = "active"
	AdminDocumentAttachmentStatusFilterDeleted AdminDocumentAttachmentStatusFilter = "deleted"
)

// AdminDocumentAttachmentStorageProviderFilter 管理后台文档附件存储提供商过滤条件。
type AdminDocumentAttachmentStorageProviderFilter string

const (
	AdminDocumentAttachmentStorageProviderFilterAll          AdminDocumentAttachmentStorageProviderFilter = "all"
	AdminDocumentAttachmentStorageProviderFilterLocal        AdminDocumentAttachmentStorageProviderFilter = "local"
	AdminDocumentAttachmentStorageProviderFilterCloudflareR2 AdminDocumentAttachmentStorageProviderFilter = "cloudflare-r2"
	AdminDocumentAttachmentStorageProviderFilterAliyunOSS    AdminDocumentAttachmentStorageProviderFilter = "aliyun-oss"
)

// AdminDocumentAttachmentRecord 后台文档附件列表项。
type AdminDocumentAttachmentRecord struct {
	AttachmentID     string
	DocumentID       string
	DocumentTitle    string
	DocumentStatus   models.EntityStatus
	SpaceID          string
	SpaceName        string
	SpaceOwnerUserID string
	SpaceOwnerName   string
	SpaceOwnerEmail  string
	FileName         string
	MimeType         string
	SizeBytes        int64
	StorageProvider  string
	PreviewKind      string
	Status           models.EntityStatus
	CreatedByUserID  *string
	CreatedByName    string
	CreatedByEmail   string
	DeletedAt        *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// ListAdminDocumentAttachmentsInput 后台文档附件列表查询参数。
type ListAdminDocumentAttachmentsInput struct {
	ActorUserID           string
	Keyword               string
	SpaceID               string
	DocumentID            string
	StatusFilter          AdminDocumentAttachmentStatusFilter
	StorageProviderFilter AdminDocumentAttachmentStorageProviderFilter
	Page                  int
	PageSize              int
}

// ListAdminDocumentAttachmentsResult 后台文档附件列表返回结果。
type ListAdminDocumentAttachmentsResult struct {
	Items    []AdminDocumentAttachmentRecord
	Page     int
	PageSize int
	Total    int64
}

// DeleteAdminDocumentAttachmentInput 后台文档附件删除参数。
type DeleteAdminDocumentAttachmentInput struct {
	ActorUserID                string
	AttachmentID               string
	PhysicalDelete             bool
	ForcePhysicalDeleteOnShare bool
	RequestID                  string
}

// DeleteAdminDocumentAttachmentReference 表示共享文件被哪些附件引用。
type DeleteAdminDocumentAttachmentReference struct {
	AttachmentID  string
	DocumentID    string
	DocumentTitle string
	SpaceID       string
	SpaceName     string
	FileName      string
}

// DeleteAdminDocumentAttachmentResult 描述后台删除附件执行结果。
type DeleteAdminDocumentAttachmentResult struct {
	AttachmentID            string
	DocumentID              string
	SpaceID                 string
	PhysicalDeleteRequested bool
	PhysicalDeleteExecuted  bool
	SoftDeleted             bool
	HardDeleted             bool
	SharedReferenceCount    int64
	SharedReferences        []DeleteAdminDocumentAttachmentReference
	ConfirmationRequired    bool
	ConfirmationReason      string
	PhysicalDeleteError     string
}

// AdminDocumentAttachmentService 封装文档附件后台治理业务。
type AdminDocumentAttachmentService struct {
	documentAttachmentRepo repository.DocumentAttachmentRepository
	adminAccessService     *AdminAccessService
	adminAuditService      *AdminAuditService
	imageHostingService    *ImageHostingService
	localAttachmentRootDir string
}

// NewAdminDocumentAttachmentService 创建后台文档附件管理服务。
func NewAdminDocumentAttachmentService(
	documentAttachmentRepo repository.DocumentAttachmentRepository,
	adminAccessService *AdminAccessService,
	adminAuditService *AdminAuditService,
	imageHostingService *ImageHostingService,
) *AdminDocumentAttachmentService {
	return &AdminDocumentAttachmentService{
		documentAttachmentRepo: documentAttachmentRepo,
		adminAccessService:     adminAccessService,
		adminAuditService:      adminAuditService,
		imageHostingService:    imageHostingService,
		localAttachmentRootDir: defaultAdminLocalAttachmentRootDir,
	}
}

// ListAttachments 查询后台文档附件列表。
func (s *AdminDocumentAttachmentService) ListAttachments(
	ctx context.Context,
	input ListAdminDocumentAttachmentsInput,
) (result ListAdminDocumentAttachmentsResult, err error) {
	defer func() {
		err = errcode.MapAdminDocumentAttachmentError(err)
	}()

	if s == nil || s.documentAttachmentRepo == nil || s.adminAccessService == nil {
		return ListAdminDocumentAttachmentsResult{}, errors.New("admin document attachment service dependencies are nil")
	}

	actorUserID := strings.TrimSpace(input.ActorUserID)
	if actorUserID == "" {
		return ListAdminDocumentAttachmentsResult{}, errcode.ErrAdminForbidden
	}

	restrictToScopes, err := s.resolveScopeRestriction(ctx, actorUserID)
	if err != nil {
		return ListAdminDocumentAttachmentsResult{}, err
	}

	statuses, err := resolveAdminDocumentAttachmentStatuses(input.StatusFilter)
	if err != nil {
		return ListAdminDocumentAttachmentsResult{}, err
	}
	storageProviders, err := resolveAdminDocumentAttachmentStorageProviders(input.StorageProviderFilter)
	if err != nil {
		return ListAdminDocumentAttachmentsResult{}, err
	}

	page, pageSize := normalizeAdminDocumentAttachmentPagination(input.Page, input.PageSize)
	records, total, err := s.documentAttachmentRepo.ListForAdmin(ctx, repository.ListAdminDocumentAttachmentsParams{
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
		return ListAdminDocumentAttachmentsResult{}, err
	}

	items := make([]AdminDocumentAttachmentRecord, 0, len(records))
	for _, record := range records {
		items = append(items, mapAdminDocumentAttachmentRecord(record))
	}

	return ListAdminDocumentAttachmentsResult{
		Items:    items,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

// DeleteAttachment 删除后台目标附件。
// - PhysicalDelete=false: 仅逻辑删除（status=deleted，保留记录与文件）
// - PhysicalDelete=true: 物理删除（删除文件并硬删除记录）
func (s *AdminDocumentAttachmentService) DeleteAttachment(
	ctx context.Context,
	input DeleteAdminDocumentAttachmentInput,
) (result DeleteAdminDocumentAttachmentResult, err error) {
	defer func() {
		err = errcode.MapAdminDocumentAttachmentError(err)
	}()

	_ = input.RequestID

	if s == nil || s.documentAttachmentRepo == nil || s.adminAccessService == nil {
		return DeleteAdminDocumentAttachmentResult{}, errors.New("admin document attachment service dependencies are nil")
	}

	actorUserID := strings.TrimSpace(input.ActorUserID)
	if actorUserID == "" {
		return DeleteAdminDocumentAttachmentResult{}, errcode.ErrAdminForbidden
	}
	attachmentID := strings.TrimSpace(input.AttachmentID)
	if attachmentID == "" {
		return DeleteAdminDocumentAttachmentResult{}, errcode.ErrAdminDocumentAttachmentInvalidAttachmentID
	}

	attachment, err := s.documentAttachmentRepo.GetByAttachmentID(ctx, attachmentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return DeleteAdminDocumentAttachmentResult{}, errcode.ErrAdminDocumentAttachmentNotFound
		}
		return DeleteAdminDocumentAttachmentResult{}, err
	}

	canManage, err := s.adminAccessService.CanManageSpace(ctx, actorUserID, strings.TrimSpace(attachment.SpaceID))
	if err != nil {
		return DeleteAdminDocumentAttachmentResult{}, err
	}
	if !canManage {
		return DeleteAdminDocumentAttachmentResult{}, errcode.ErrAdminForbidden
	}

	result = DeleteAdminDocumentAttachmentResult{
		AttachmentID:            attachmentID,
		DocumentID:              strings.TrimSpace(attachment.DocumentID),
		SpaceID:                 strings.TrimSpace(attachment.SpaceID),
		PhysicalDeleteRequested: input.PhysicalDelete,
	}

	beforeStatus := attachment.Status
	afterStatus := any(attachment.Status)
	if !models.IsValidEntityStatus(beforeStatus) {
		beforeStatus = models.EntityStatusActive
	}

	physicalDeleteExecuted := false
	if input.PhysicalDelete {
		blobID := strings.TrimSpace(attachment.BlobID)
		if blobID == "" {
			return DeleteAdminDocumentAttachmentResult{}, errors.New("attachment blob id is empty")
		}

		activeRefCount, countErr := s.documentAttachmentRepo.CountActiveReferencesByBlobID(ctx, blobID)
		if countErr != nil {
			return DeleteAdminDocumentAttachmentResult{}, countErr
		}
		result.SharedReferenceCount = activeRefCount
		if activeRefCount > 1 {
			references, referenceErr := s.documentAttachmentRepo.ListActiveReferencesByBlobID(ctx, blobID, defaultAdminDeleteRefPreviewLimit)
			if referenceErr != nil {
				return DeleteAdminDocumentAttachmentResult{}, referenceErr
			}
			result.SharedReferences = mapDeleteAdminDocumentAttachmentReferences(references)
			if !input.ForcePhysicalDeleteOnShare {
				result.ConfirmationRequired = true
				result.ConfirmationReason = "当前文件被多个文档引用，确认后仅删除当前附件记录，不会删除物理文件。"
				return result, nil
			}
		}

		hardDeleted, hardDeleteErr := s.documentAttachmentRepo.HardDelete(ctx, attachmentID)
		if hardDeleteErr != nil {
			return DeleteAdminDocumentAttachmentResult{}, hardDeleteErr
		}
		if !hardDeleted {
			return DeleteAdminDocumentAttachmentResult{}, errcode.ErrAdminDocumentAttachmentNotFound
		}
		result.HardDeleted = true

		blob, getBlobErr := s.documentAttachmentRepo.GetBlobByBlobID(ctx, blobID)
		if getBlobErr != nil && !errors.Is(getBlobErr, gorm.ErrRecordNotFound) {
			return DeleteAdminDocumentAttachmentResult{}, getBlobErr
		}

		blobDeleted, deleteBlobErr := s.documentAttachmentRepo.HardDeleteBlobIfUnreferenced(ctx, blobID)
		if deleteBlobErr != nil {
			return DeleteAdminDocumentAttachmentResult{}, deleteBlobErr
		}
		if blobDeleted && blob != nil {
			deletePhysicalErr := s.tryPhysicalDeleteBlob(ctx, blob)
			if deletePhysicalErr != nil {
				result.PhysicalDeleteError = deletePhysicalErr.Error()
			} else {
				physicalDeleteExecuted = true
			}
		} else if blob == nil {
			result.PhysicalDeleteError = "文件实体不存在，跳过物理文件删除"
		} else {
			result.PhysicalDeleteError = "文件仍存在引用，未执行物理文件删除"
		}

		remainingRefCount, remainingCountErr := s.documentAttachmentRepo.CountActiveReferencesByBlobID(ctx, blobID)
		if remainingCountErr != nil {
			return DeleteAdminDocumentAttachmentResult{}, remainingCountErr
		}
		result.SharedReferenceCount = remainingRefCount
		afterStatus = "hard_deleted"
	} else if beforeStatus != models.EntityStatusDeleted {
		deleted, deleteErr := s.documentAttachmentRepo.SoftDelete(ctx, attachmentID, time.Now().UTC())
		if deleteErr != nil {
			return DeleteAdminDocumentAttachmentResult{}, deleteErr
		}
		if !deleted {
			return DeleteAdminDocumentAttachmentResult{}, errcode.ErrAdminDocumentAttachmentNotFound
		}
		result.SoftDeleted = true
		afterStatus = models.EntityStatusDeleted
	}
	result.PhysicalDeleteExecuted = physicalDeleteExecuted

	if err := s.recordAttachmentAudit(ctx, RecordAdminAuditInput{
		Module:     AdminAuditModuleDocument,
		Action:     AdminAuditActionDelete,
		TargetType: "document_attachment",
		TargetID:   attachmentID,
		Summary:    "document attachment deleted: " + attachmentID,
		Detail: map[string]any{
			"documentId":             strings.TrimSpace(attachment.DocumentID),
			"spaceId":                strings.TrimSpace(attachment.SpaceID),
			"storageProvider":        strings.TrimSpace(attachment.StorageProvider),
			"objectKey":              strings.TrimSpace(attachment.ObjectKey),
			"statusBefore":           beforeStatus,
			"statusAfter":            afterStatus,
			"physicalDelete":         input.PhysicalDelete,
			"physicalDeleteExecuted": physicalDeleteExecuted,
			"physicalDeleteError":    strings.TrimSpace(result.PhysicalDeleteError),
			"sharedReferenceCount":   result.SharedReferenceCount,
			"confirmationRequired":   result.ConfirmationRequired,
		},
	}); err != nil {
		return DeleteAdminDocumentAttachmentResult{}, err
	}

	return result, nil
}

func (s *AdminDocumentAttachmentService) resolveScopeRestriction(ctx context.Context, actorUserID string) (bool, error) {
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

func (s *AdminDocumentAttachmentService) tryPhysicalDeleteBlob(
	ctx context.Context,
	blob *models.DocumentAttachmentBlob,
) error {
	if blob == nil {
		return nil
	}
	storageProvider := normalizeImageHostingProvider(blob.StorageProvider)
	if storageProvider == "" {
		storageProvider = ImageHostingProviderLocal
	}
	normalizedObjectKey := strings.TrimSpace(blob.ObjectKey)
	if normalizedObjectKey == "" {
		return errors.New("attachment object key is empty")
	}

	switch storageProvider {
	case ImageHostingProviderLocal:
		targetPath, err := s.resolveLocalAttachmentTargetPath(normalizedObjectKey)
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
		return s.deleteBlobFromCloudflareR2(ctx, normalizedObjectKey, config)
	case ImageHostingProviderAliyunOSS:
		config, configErr := s.loadImageHostingConfig(ctx)
		if configErr != nil {
			return configErr
		}
		return s.deleteBlobFromAliyunOSS(normalizedObjectKey, config)
	default:
		return errors.New("unsupported attachment storage provider")
	}
}

func (s *AdminDocumentAttachmentService) loadImageHostingConfig(ctx context.Context) (ImageHostingConfig, error) {
	if s == nil || s.imageHostingService == nil {
		return ImageHostingConfig{}, errors.New("image hosting service is nil")
	}
	return s.imageHostingService.GetConfig(ctx)
}

func (s *AdminDocumentAttachmentService) deleteBlobFromCloudflareR2(
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

func (s *AdminDocumentAttachmentService) deleteBlobFromAliyunOSS(
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

func resolveImageHostingCloudflareR2Endpoint(accountID string) string {
	normalizedAccountID := strings.TrimSpace(accountID)
	if normalizedAccountID == "" {
		return ""
	}
	if strings.HasPrefix(normalizedAccountID, "https://") || strings.HasPrefix(normalizedAccountID, "http://") {
		return strings.TrimRight(normalizedAccountID, "/")
	}
	return "https://" + normalizedAccountID + ".r2.cloudflarestorage.com"
}

func resolveImageHostingAliyunOSSEndpoint(endpoint string, region string) string {
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

func (s *AdminDocumentAttachmentService) resolveLocalAttachmentTargetPath(objectKey string) (string, error) {
	normalizedObjectKey := strings.TrimSpace(strings.TrimPrefix(objectKey, "/"))
	if normalizedObjectKey == "" {
		return "", errors.New("object key is empty")
	}
	cleanObjectKey := path.Clean(normalizedObjectKey)
	if cleanObjectKey == "." || cleanObjectKey == "/" || strings.HasPrefix(cleanObjectKey, "../") {
		return "", errors.New("object key is invalid")
	}

	localRootDir := strings.TrimSpace(s.localAttachmentRootDir)
	if localRootDir == "" {
		localRootDir = defaultAdminLocalAttachmentRootDir
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

func isPathWithinRoot(rootPath string, targetPath string) bool {
	relativePath, err := filepath.Rel(rootPath, targetPath)
	if err != nil {
		return false
	}
	cleanRelativePath := filepath.Clean(relativePath)
	if cleanRelativePath == "." {
		return true
	}
	parentPrefix := ".." + string(filepath.Separator)
	return cleanRelativePath != ".." && !strings.HasPrefix(cleanRelativePath, parentPrefix)
}

func (s *AdminDocumentAttachmentService) recordAttachmentAudit(ctx context.Context, input RecordAdminAuditInput) error {
	if s == nil || s.adminAuditService == nil {
		return nil
	}
	return s.adminAuditService.Record(ctx, input)
}

func mapDeleteAdminDocumentAttachmentReferences(
	records []repository.DocumentAttachmentReferenceRecord,
) []DeleteAdminDocumentAttachmentReference {
	if len(records) == 0 {
		return []DeleteAdminDocumentAttachmentReference{}
	}

	items := make([]DeleteAdminDocumentAttachmentReference, 0, len(records))
	for _, record := range records {
		items = append(items, DeleteAdminDocumentAttachmentReference{
			AttachmentID:  strings.TrimSpace(record.AttachmentID),
			DocumentID:    strings.TrimSpace(record.DocumentID),
			DocumentTitle: strings.TrimSpace(record.DocumentTitle),
			SpaceID:       strings.TrimSpace(record.SpaceID),
			SpaceName:     strings.TrimSpace(record.SpaceName),
			FileName:      strings.TrimSpace(record.FileName),
		})
	}
	return items
}

func resolveAdminDocumentAttachmentStatuses(
	filter AdminDocumentAttachmentStatusFilter,
) ([]models.EntityStatus, error) {
	switch normalizeAdminDocumentAttachmentStatusFilter(filter) {
	case "":
		return []models.EntityStatus{
			models.EntityStatusActive,
			models.EntityStatusBanned,
			models.EntityStatusDeleted,
		}, nil
	case AdminDocumentAttachmentStatusFilterAll:
		return []models.EntityStatus{
			models.EntityStatusActive,
			models.EntityStatusBanned,
			models.EntityStatusDeleted,
		}, nil
	case AdminDocumentAttachmentStatusFilterActive:
		return []models.EntityStatus{models.EntityStatusActive}, nil
	case AdminDocumentAttachmentStatusFilterDeleted:
		return []models.EntityStatus{models.EntityStatusDeleted}, nil
	default:
		return nil, errcode.ErrAdminDocumentAttachmentInvalidStatusFilter
	}
}

func resolveAdminDocumentAttachmentStorageProviders(
	filter AdminDocumentAttachmentStorageProviderFilter,
) ([]string, error) {
	switch normalizeAdminDocumentAttachmentStorageProviderFilter(filter) {
	case "", AdminDocumentAttachmentStorageProviderFilterAll:
		return nil, nil
	case AdminDocumentAttachmentStorageProviderFilterLocal:
		return []string{string(ImageHostingProviderLocal)}, nil
	case AdminDocumentAttachmentStorageProviderFilterCloudflareR2:
		return []string{string(ImageHostingProviderCloudflareR2)}, nil
	case AdminDocumentAttachmentStorageProviderFilterAliyunOSS:
		return []string{string(ImageHostingProviderAliyunOSS)}, nil
	default:
		return nil, errcode.ErrAdminDocumentAttachmentInvalidStorageProviderFilter
	}
}

func normalizeAdminDocumentAttachmentStatusFilter(
	filter AdminDocumentAttachmentStatusFilter,
) AdminDocumentAttachmentStatusFilter {
	value := strings.ToLower(strings.TrimSpace(string(filter)))
	if value == "" {
		return ""
	}
	return AdminDocumentAttachmentStatusFilter(value)
}

func normalizeAdminDocumentAttachmentStorageProviderFilter(
	filter AdminDocumentAttachmentStorageProviderFilter,
) AdminDocumentAttachmentStorageProviderFilter {
	value := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(string(filter))), "_", "-")
	if value == "" {
		return ""
	}
	return AdminDocumentAttachmentStorageProviderFilter(value)
}

func normalizeAdminDocumentAttachmentPagination(page int, pageSize int) (int, int) {
	normalizedPage := page
	if normalizedPage <= 0 {
		normalizedPage = defaultAdminDocumentAttachmentPage
	}

	normalizedPageSize := pageSize
	if normalizedPageSize <= 0 {
		normalizedPageSize = defaultAdminDocumentAttachmentPageSize
	}
	if normalizedPageSize > maxAdminDocumentAttachmentPageSize {
		normalizedPageSize = maxAdminDocumentAttachmentPageSize
	}

	return normalizedPage, normalizedPageSize
}

func mapAdminDocumentAttachmentRecord(
	record repository.AdminDocumentAttachmentListRecord,
) AdminDocumentAttachmentRecord {
	attachment := record.Attachment
	status := attachment.Status
	if !models.IsValidEntityStatus(status) {
		status = models.EntityStatusActive
	}
	documentStatus := record.DocumentStatus
	if !models.IsValidEntityStatus(documentStatus) {
		documentStatus = models.EntityStatusActive
	}

	return AdminDocumentAttachmentRecord{
		AttachmentID:     strings.TrimSpace(attachment.AttachmentID),
		DocumentID:       strings.TrimSpace(attachment.DocumentID),
		DocumentTitle:    strings.TrimSpace(record.DocumentTitle),
		DocumentStatus:   documentStatus,
		SpaceID:          strings.TrimSpace(attachment.SpaceID),
		SpaceName:        strings.TrimSpace(record.SpaceName),
		SpaceOwnerUserID: strings.TrimSpace(record.SpaceOwnerID),
		SpaceOwnerName:   strings.TrimSpace(record.SpaceOwnerName),
		SpaceOwnerEmail:  strings.TrimSpace(record.SpaceOwnerEmail),
		FileName:         strings.TrimSpace(attachment.FileName),
		MimeType:         strings.TrimSpace(attachment.MimeType),
		SizeBytes:        attachment.SizeBytes,
		StorageProvider:  strings.TrimSpace(attachment.StorageProvider),
		PreviewKind:      strings.TrimSpace(attachment.PreviewKind),
		Status:           status,
		CreatedByUserID:  attachment.CreatedByUserID,
		CreatedByName:    strings.TrimSpace(record.CreatedByName),
		CreatedByEmail:   strings.TrimSpace(record.CreatedByEmail),
		DeletedAt:        attachment.DeletedAt,
		CreatedAt:        attachment.CreatedAt,
		UpdatedAt:        attachment.UpdatedAt,
	}
}
