package service

import (
	"context"
	"errors"
	"strings"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

const (
	defaultDocumentAttachmentCleanupBatchSize = 500
	maxDocumentAttachmentCleanupBatchSize     = 5000
)

// DocumentAttachmentCleanupResult 描述一次附件清理执行结果。
type DocumentAttachmentCleanupResult struct {
	DeletedAttachments int64
	DeletedBlobs       int64
}

// DocumentAttachmentCleanupService 负责清理“已删除文档”的附件引用与无引用文件实体。
// 补偿策略：当物理文件删除失败时，保留 file_blobs 记录，由后续批次继续重试。
type DocumentAttachmentCleanupService struct {
	db                     *gorm.DB
	documentAttachmentRepo repository.DocumentAttachmentRepository
	imageHostingService    *ImageHostingService
	objectCleanupService   *DocumentImageAssetService
}

// NewDocumentAttachmentCleanupService 创建附件清理服务。
func NewDocumentAttachmentCleanupService(
	db *gorm.DB,
	documentAttachmentRepo repository.DocumentAttachmentRepository,
	imageHostingService *ImageHostingService,
) *DocumentAttachmentCleanupService {
	return &DocumentAttachmentCleanupService{
		db:                     db,
		documentAttachmentRepo: documentAttachmentRepo,
		imageHostingService:    imageHostingService,
		objectCleanupService:   NewDocumentImageAssetService(db, imageHostingService),
	}
}

// CleanupDeletedDocumentAttachments 批量执行附件清理：
// 1) 删除“已删除文档”仍残留的附件记录；
// 2) 回收无引用 file_blobs；
// 3) 物理文件删除失败时保留 blob，等待后续补偿批次重试。
func (s *DocumentAttachmentCleanupService) CleanupDeletedDocumentAttachments(
	ctx context.Context,
	batchSize int,
) (DocumentAttachmentCleanupResult, error) {
	if s == nil || s.db == nil || s.documentAttachmentRepo == nil {
		return DocumentAttachmentCleanupResult{}, errors.New("document attachment cleanup service dependencies are nil")
	}
	normalizedBatchSize := normalizeDocumentAttachmentCleanupBatchSize(batchSize)
	result := DocumentAttachmentCleanupResult{}

	attachments, err := s.listDeletedDocumentAttachmentCandidates(ctx, normalizedBatchSize)
	if err != nil {
		return result, err
	}

	blobIDsToCleanup := make(map[string]struct{}, len(attachments))
	for _, attachment := range attachments {
		attachmentID := strings.TrimSpace(attachment.AttachmentID)
		if attachmentID == "" {
			continue
		}
		deleted, hardDeleteErr := s.documentAttachmentRepo.HardDelete(ctx, attachmentID)
		if hardDeleteErr != nil {
			return result, hardDeleteErr
		}
		if !deleted {
			continue
		}
		result.DeletedAttachments++

		blobID := strings.TrimSpace(attachment.BlobID)
		if blobID != "" {
			blobIDsToCleanup[blobID] = struct{}{}
		}
	}

	for blobID := range blobIDsToCleanup {
		deletedBlob, cleanupErr := s.cleanupBlobIfUnreferenced(ctx, blobID)
		if cleanupErr != nil {
			return result, cleanupErr
		}
		if deletedBlob {
			result.DeletedBlobs++
		}
	}

	orphanBlobs, err := s.documentAttachmentRepo.ListOrphanBlobs(ctx, normalizedBatchSize)
	if err != nil {
		return result, err
	}
	for _, blob := range orphanBlobs {
		blobID := strings.TrimSpace(blob.BlobID)
		if blobID == "" {
			continue
		}
		if _, exists := blobIDsToCleanup[blobID]; exists {
			continue
		}
		deletedBlob, cleanupErr := s.cleanupBlobIfUnreferenced(ctx, blobID)
		if cleanupErr != nil {
			return result, cleanupErr
		}
		if deletedBlob {
			result.DeletedBlobs++
		}
	}

	return result, nil
}

type deletedDocumentAttachmentCleanupCandidate struct {
	AttachmentID string `gorm:"column:attachment_id"`
	BlobID       string `gorm:"column:blob_id"`
}

func (s *DocumentAttachmentCleanupService) listDeletedDocumentAttachmentCandidates(
	ctx context.Context,
	batchSize int,
) ([]deletedDocumentAttachmentCleanupCandidate, error) {
	candidates := make([]deletedDocumentAttachmentCleanupCandidate, 0, batchSize)
	if err := s.db.WithContext(ctx).
		Table("document_attachments AS da").
		Select("da.attachment_id, da.blob_id").
		Joins("JOIN documents AS d ON d.document_id = da.document_id").
		Where("d.status = ? OR d.deleted_at IS NOT NULL", models.EntityStatusDeleted).
		Order("da.id ASC").
		Limit(batchSize).
		Find(&candidates).Error; err != nil {
		return nil, err
	}
	return candidates, nil
}

func (s *DocumentAttachmentCleanupService) cleanupBlobIfUnreferenced(
	ctx context.Context,
	blobID string,
) (bool, error) {
	normalizedBlobID := strings.TrimSpace(blobID)
	if normalizedBlobID == "" {
		return false, nil
	}

	activeRefs, countErr := s.documentAttachmentRepo.CountActiveReferencesByBlobID(ctx, normalizedBlobID)
	if countErr != nil {
		return false, countErr
	}
	if activeRefs > 0 {
		return false, nil
	}

	blob, blobErr := s.documentAttachmentRepo.GetBlobByBlobID(ctx, normalizedBlobID)
	if blobErr != nil {
		if errors.Is(blobErr, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, blobErr
	}

	if deleteErr := s.tryDeleteBlobPhysicalObject(ctx, blob); deleteErr != nil {
		// 保留 blob 记录，交给后续批次补偿重试。
		return false, nil
	}

	return s.documentAttachmentRepo.HardDeleteBlobIfUnreferenced(ctx, normalizedBlobID)
}

func (s *DocumentAttachmentCleanupService) tryDeleteBlobPhysicalObject(
	ctx context.Context,
	blob *models.DocumentAttachmentBlob,
) error {
	if s == nil || s.objectCleanupService == nil {
		return errors.New("document attachment cleanup object service is nil")
	}
	if blob == nil {
		return nil
	}

	config := DefaultImageHostingConfig()
	if s.imageHostingService != nil {
		loadedConfig, err := s.imageHostingService.GetConfig(ctx)
		if err != nil {
			return err
		}
		config = loadedConfig
	}
	return s.objectCleanupService.deletePhysicalObject(
		ctx,
		config,
		blob.StorageProvider,
		blob.ObjectKey,
	)
}

func normalizeDocumentAttachmentCleanupBatchSize(batchSize int) int {
	if batchSize <= 0 {
		return defaultDocumentAttachmentCleanupBatchSize
	}
	if batchSize > maxDocumentAttachmentCleanupBatchSize {
		return maxDocumentAttachmentCleanupBatchSize
	}
	return batchSize
}
