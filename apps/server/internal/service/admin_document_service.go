package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/errcode"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

const (
	defaultAdminDocumentPage     = 1
	defaultAdminDocumentPageSize = 20
	maxAdminDocumentPageSize     = 100
)

// AdminDocumentStatusFilter 管理后台文档状态过滤条件。
type AdminDocumentStatusFilter string

const (
	AdminDocumentStatusFilterAll     AdminDocumentStatusFilter = "all"
	AdminDocumentStatusFilterActive  AdminDocumentStatusFilter = "active"
	AdminDocumentStatusFilterBanned  AdminDocumentStatusFilter = "banned"
	AdminDocumentStatusFilterDeleted AdminDocumentStatusFilter = "deleted"
)

// AdminDocumentVisibilityFilter 管理后台文档可见性过滤条件。
type AdminDocumentVisibilityFilter string

const (
	AdminDocumentVisibilityFilterAll           AdminDocumentVisibilityFilter = "all"
	AdminDocumentVisibilityFilterPublic        AdminDocumentVisibilityFilter = "public"
	AdminDocumentVisibilityFilterAuthenticated AdminDocumentVisibilityFilter = "authenticated"
	AdminDocumentVisibilityFilterMember        AdminDocumentVisibilityFilter = "member"
)

// AdminDocumentRecord 后台文档列表项。
type AdminDocumentRecord struct {
	DocumentID       string
	NodeID           string
	Title            string
	SpaceID          string
	SpaceName        string
	SpaceOwnerUserID string
	SpaceOwnerName   string
	SpaceOwnerEmail  string
	Visibility       models.Visibility
	Status           models.EntityStatus
	BannedReason     string
	BannedAt         *time.Time
	DeletedAt        *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// ListAdminDocumentsInput 后台文档列表查询参数。
type ListAdminDocumentsInput struct {
	ActorUserID      string
	Keyword          string
	SpaceID          string
	StatusFilter     AdminDocumentStatusFilter
	VisibilityFilter AdminDocumentVisibilityFilter
	Page             int
	PageSize         int
}

// ListAdminDocumentsResult 后台文档列表返回结果。
type ListAdminDocumentsResult struct {
	Items    []AdminDocumentRecord
	Page     int
	PageSize int
	Total    int64
}

// UpdateAdminDocumentStatusInput 后台文档状态更新参数。
type UpdateAdminDocumentStatusInput struct {
	ActorUserID string
	RequestID   string
	DocumentID  string
	Status      models.EntityStatus
	Reason      string
}

// AdminDocumentService 封装文档管理业务。
type AdminDocumentService struct {
	documentRepo                     repository.DocumentRepository
	userRepo                         repository.UserRepository
	adminAccessService               *AdminAccessService
	adminAuditService                *AdminAuditService
	documentAttachmentCleanupService *DocumentAttachmentCleanupService
}

// NewAdminDocumentService 创建后台文档管理服务。
func NewAdminDocumentService(
	documentRepo repository.DocumentRepository,
	userRepo repository.UserRepository,
	adminAccessService *AdminAccessService,
	adminAuditService *AdminAuditService,
	documentAttachmentCleanupService *DocumentAttachmentCleanupService,
) *AdminDocumentService {
	return &AdminDocumentService{
		documentRepo:                     documentRepo,
		userRepo:                         userRepo,
		adminAccessService:               adminAccessService,
		adminAuditService:                adminAuditService,
		documentAttachmentCleanupService: documentAttachmentCleanupService,
	}
}

// ListDocuments 查询后台文档列表。
func (s *AdminDocumentService) ListDocuments(
	ctx context.Context,
	input ListAdminDocumentsInput,
) (result ListAdminDocumentsResult, err error) {
	defer func() {
		err = errcode.MapAdminDocumentError(err)
	}()

	if s == nil || s.documentRepo == nil || s.adminAccessService == nil {
		return ListAdminDocumentsResult{}, errors.New("admin document service dependencies are nil")
	}

	actorUserID := strings.TrimSpace(input.ActorUserID)
	if actorUserID == "" {
		return ListAdminDocumentsResult{}, errcode.ErrAdminForbidden
	}

	restrictToScopes, err := s.resolveScopeRestriction(ctx, actorUserID)
	if err != nil {
		return ListAdminDocumentsResult{}, err
	}

	statuses, err := resolveAdminDocumentStatuses(input.StatusFilter)
	if err != nil {
		return ListAdminDocumentsResult{}, err
	}
	visibilities, err := resolveAdminDocumentVisibilities(input.VisibilityFilter)
	if err != nil {
		return ListAdminDocumentsResult{}, err
	}

	page, pageSize := normalizeAdminDocumentPagination(input.Page, input.PageSize)
	records, total, err := s.documentRepo.ListForAdmin(ctx, repository.ListAdminDocumentsParams{
		ActorUserID:      actorUserID,
		RestrictToScopes: restrictToScopes,
		Keyword:          strings.TrimSpace(input.Keyword),
		SpaceID:          strings.TrimSpace(input.SpaceID),
		Statuses:         statuses,
		Visibilities:     visibilities,
		Limit:            pageSize,
		Offset:           (page - 1) * pageSize,
	})
	if err != nil {
		return ListAdminDocumentsResult{}, err
	}

	items := make([]AdminDocumentRecord, 0, len(records))
	for _, record := range records {
		items = append(items, mapAdminDocumentRecord(record))
	}

	return ListAdminDocumentsResult{
		Items:    items,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

// UpdateStatus 更新文档状态（active/banned）。
func (s *AdminDocumentService) UpdateStatus(
	ctx context.Context,
	input UpdateAdminDocumentStatusInput,
) (result AdminDocumentRecord, err error) {
	defer func() {
		err = errcode.MapAdminDocumentError(err)
	}()

	if s == nil || s.documentRepo == nil || s.adminAccessService == nil {
		return AdminDocumentRecord{}, errors.New("admin document service dependencies are nil")
	}

	actorUserID := strings.TrimSpace(input.ActorUserID)
	documentID := strings.TrimSpace(input.DocumentID)
	if documentID == "" {
		return AdminDocumentRecord{}, errcode.ErrAdminDocumentInvalidDocumentID
	}
	if input.Status != models.EntityStatusActive && input.Status != models.EntityStatusBanned {
		return AdminDocumentRecord{}, errcode.ErrAdminDocumentInvalidStatus
	}
	if input.Status == models.EntityStatusBanned && strings.TrimSpace(input.Reason) == "" {
		return AdminDocumentRecord{}, errcode.ErrAdminDocumentBanReasonRequired
	}

	accessInfo, err := s.documentRepo.GetAccessByDocumentID(ctx, documentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminDocumentRecord{}, errcode.ErrAdminDocumentNotFound
		}
		return AdminDocumentRecord{}, err
	}
	if err := s.ensureCanManageSpace(ctx, actorUserID, accessInfo.SpaceID); err != nil {
		return AdminDocumentRecord{}, err
	}
	if normalizeEntityStatus(accessInfo.Document.Status) == models.EntityStatusDeleted || accessInfo.Document.DeletedAt != nil {
		return AdminDocumentRecord{}, errcode.ErrAdminDocumentAlreadyDeleted
	}

	now := time.Now().UTC()
	params := repository.UpdateDocumentStatusParams{
		DocumentID: documentID,
		Status:     input.Status,
		UpdatedAt:  now,
	}
	if input.Status == models.EntityStatusBanned {
		params.BannedReason = strings.TrimSpace(input.Reason)
		params.BannedAt = &now
	}

	updated, err := s.documentRepo.UpdateStatus(ctx, params)
	if err != nil {
		return AdminDocumentRecord{}, err
	}
	if !updated {
		return AdminDocumentRecord{}, errcode.ErrAdminDocumentNotFound
	}

	latestAccessInfo, err := s.documentRepo.GetAccessByDocumentID(ctx, documentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminDocumentRecord{}, errcode.ErrAdminDocumentNotFound
		}
		return AdminDocumentRecord{}, err
	}
	record := s.buildRecordFromAccess(ctx, latestAccessInfo)
	if err := s.recordDocumentAudit(ctx, RecordAdminAuditInput{
		Module:     AdminAuditModuleDocument,
		Action:     AdminAuditActionUpdate,
		TargetType: "document",
		TargetID:   record.DocumentID,
		Summary:    "document status updated: " + record.DocumentID,
		Detail: map[string]any{
			"spaceId":      record.SpaceID,
			"statusBefore": accessInfo.Document.Status,
			"statusAfter":  record.Status,
			"reason":       strings.TrimSpace(input.Reason),
		},
	}); err != nil {
		return AdminDocumentRecord{}, err
	}

	return record, nil
}

// DeleteDocument 软删除后台目标文档。
func (s *AdminDocumentService) DeleteDocument(
	ctx context.Context,
	actorUserID string,
	documentID string,
	requestID string,
) (err error) {
	defer func() {
		err = errcode.MapAdminDocumentError(err)
	}()

	_ = requestID

	if s == nil || s.documentRepo == nil || s.adminAccessService == nil {
		return errors.New("admin document service dependencies are nil")
	}

	targetDocumentID := strings.TrimSpace(documentID)
	if targetDocumentID == "" {
		return errcode.ErrAdminDocumentInvalidDocumentID
	}

	accessInfo, err := s.documentRepo.GetAccessByDocumentID(ctx, targetDocumentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.ErrAdminDocumentNotFound
		}
		return err
	}
	if err := s.ensureCanManageSpace(ctx, strings.TrimSpace(actorUserID), accessInfo.SpaceID); err != nil {
		return err
	}

	if normalizeEntityStatus(accessInfo.Document.Status) == models.EntityStatusDeleted || accessInfo.Document.DeletedAt != nil {
		return nil
	}

	deleted, err := s.documentRepo.SoftDelete(ctx, targetDocumentID, time.Now().UTC())
	if err != nil {
		return err
	}
	if !deleted {
		return errcode.ErrAdminDocumentNotFound
	}

	var (
		cleanupDeletedAttachments int64
		cleanupDeletedBlobs       int64
		cleanupErrorText          string
	)
	if s.documentAttachmentCleanupService != nil {
		cleanupResult, cleanupErr := s.documentAttachmentCleanupService.CleanupDeletedDocumentAttachments(ctx, defaultDataRetentionBatchSize)
		if cleanupErr != nil {
			cleanupErrorText = strings.TrimSpace(cleanupErr.Error())
		} else {
			cleanupDeletedAttachments = cleanupResult.DeletedAttachments
			cleanupDeletedBlobs = cleanupResult.DeletedBlobs
		}
	}

	if err := s.recordDocumentAudit(ctx, RecordAdminAuditInput{
		Module:     AdminAuditModuleDocument,
		Action:     AdminAuditActionDelete,
		TargetType: "document",
		TargetID:   targetDocumentID,
		Summary:    "document deleted: " + targetDocumentID,
		Detail: map[string]any{
			"spaceId":                   accessInfo.SpaceID,
			"statusBefore":              accessInfo.Document.Status,
			"statusAfter":               models.EntityStatusDeleted,
			"cleanupDeletedAttachments": cleanupDeletedAttachments,
			"cleanupDeletedBlobs":       cleanupDeletedBlobs,
			"cleanupError":              cleanupErrorText,
		},
	}); err != nil {
		return err
	}

	return nil
}

func (s *AdminDocumentService) ensureCanManageSpace(ctx context.Context, actorUserID string, spaceID string) error {
	if strings.TrimSpace(actorUserID) == "" {
		return errcode.ErrAdminForbidden
	}
	canManage, err := s.adminAccessService.CanManageSpace(ctx, actorUserID, spaceID)
	if err != nil {
		return err
	}
	if !canManage {
		return errcode.ErrAdminForbidden
	}
	return nil
}

func (s *AdminDocumentService) resolveScopeRestriction(ctx context.Context, actorUserID string) (bool, error) {
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

func (s *AdminDocumentService) recordDocumentAudit(ctx context.Context, input RecordAdminAuditInput) error {
	if s == nil || s.adminAuditService == nil {
		return nil
	}
	return s.adminAuditService.Record(ctx, input)
}

func (s *AdminDocumentService) buildRecordFromAccess(
	ctx context.Context,
	accessInfo *repository.DocumentAccessInfo,
) AdminDocumentRecord {
	if accessInfo == nil {
		return AdminDocumentRecord{}
	}

	spaceOwnerName := ""
	spaceOwnerEmail := ""
	if s.userRepo != nil {
		owner, err := s.userRepo.GetByUserID(ctx, accessInfo.SpaceOwnerUserID)
		if err == nil && owner != nil {
			spaceOwnerName = owner.Name
			spaceOwnerEmail = owner.Email
		}
	}

	visibility := accessInfo.Document.Visibility
	if !models.IsValidVisibility(visibility) {
		visibility = models.VisibilityMember
	}
	status := accessInfo.Document.Status
	if !models.IsValidEntityStatus(status) {
		status = models.EntityStatusActive
	}

	return AdminDocumentRecord{
		DocumentID:       accessInfo.Document.DocumentID,
		NodeID:           accessInfo.Document.NodeID,
		Title:            accessInfo.Document.Title,
		SpaceID:          accessInfo.SpaceID,
		SpaceName:        accessInfo.SpaceName,
		SpaceOwnerUserID: accessInfo.SpaceOwnerUserID,
		SpaceOwnerName:   spaceOwnerName,
		SpaceOwnerEmail:  spaceOwnerEmail,
		Visibility:       visibility,
		Status:           status,
		BannedReason:     accessInfo.Document.BannedReason,
		BannedAt:         accessInfo.Document.BannedAt,
		DeletedAt:        accessInfo.Document.DeletedAt,
		CreatedAt:        accessInfo.Document.CreatedAt,
		UpdatedAt:        accessInfo.Document.UpdatedAt,
	}
}

func resolveAdminDocumentStatuses(filter AdminDocumentStatusFilter) ([]models.EntityStatus, error) {
	switch normalizeAdminDocumentStatusFilter(filter) {
	case "":
		return []models.EntityStatus{models.EntityStatusActive, models.EntityStatusBanned}, nil
	case AdminDocumentStatusFilterAll:
		return []models.EntityStatus{
			models.EntityStatusActive,
			models.EntityStatusBanned,
			models.EntityStatusDeleted,
		}, nil
	case AdminDocumentStatusFilterActive:
		return []models.EntityStatus{models.EntityStatusActive}, nil
	case AdminDocumentStatusFilterBanned:
		return []models.EntityStatus{models.EntityStatusBanned}, nil
	case AdminDocumentStatusFilterDeleted:
		return []models.EntityStatus{models.EntityStatusDeleted}, nil
	default:
		return nil, errcode.ErrAdminDocumentInvalidStatusFilter
	}
}

func resolveAdminDocumentVisibilities(filter AdminDocumentVisibilityFilter) ([]models.Visibility, error) {
	switch normalizeAdminDocumentVisibilityFilter(filter) {
	case "", AdminDocumentVisibilityFilterAll:
		return []models.Visibility{
			models.VisibilityPublic,
			models.VisibilityAuthenticated,
			models.VisibilityMember,
		}, nil
	case AdminDocumentVisibilityFilterPublic:
		return []models.Visibility{models.VisibilityPublic}, nil
	case AdminDocumentVisibilityFilterAuthenticated:
		return []models.Visibility{models.VisibilityAuthenticated}, nil
	case AdminDocumentVisibilityFilterMember:
		return []models.Visibility{models.VisibilityMember}, nil
	default:
		return nil, errcode.ErrAdminDocumentInvalidVisibilityFilter
	}
}

func normalizeAdminDocumentStatusFilter(filter AdminDocumentStatusFilter) AdminDocumentStatusFilter {
	value := strings.ToLower(strings.TrimSpace(string(filter)))
	if value == "" {
		return ""
	}
	return AdminDocumentStatusFilter(value)
}

func normalizeAdminDocumentVisibilityFilter(filter AdminDocumentVisibilityFilter) AdminDocumentVisibilityFilter {
	value := strings.ToLower(strings.TrimSpace(string(filter)))
	if value == "" {
		return ""
	}
	return AdminDocumentVisibilityFilter(value)
}

func normalizeAdminDocumentPagination(page int, pageSize int) (int, int) {
	normalizedPage := page
	if normalizedPage <= 0 {
		normalizedPage = defaultAdminDocumentPage
	}

	normalizedPageSize := pageSize
	if normalizedPageSize <= 0 {
		normalizedPageSize = defaultAdminDocumentPageSize
	}
	if normalizedPageSize > maxAdminDocumentPageSize {
		normalizedPageSize = maxAdminDocumentPageSize
	}

	return normalizedPage, normalizedPageSize
}

func mapAdminDocumentRecord(record repository.AdminDocumentListRecord) AdminDocumentRecord {
	visibility := record.Document.Visibility
	if !models.IsValidVisibility(visibility) {
		visibility = models.VisibilityMember
	}
	status := record.Document.Status
	if !models.IsValidEntityStatus(status) {
		status = models.EntityStatusActive
	}

	return AdminDocumentRecord{
		DocumentID:       record.Document.DocumentID,
		NodeID:           record.Document.NodeID,
		Title:            record.Document.Title,
		SpaceID:          record.SpaceID,
		SpaceName:        record.SpaceName,
		SpaceOwnerUserID: record.SpaceOwnerID,
		SpaceOwnerName:   record.SpaceOwnerName,
		SpaceOwnerEmail:  record.SpaceOwnerEmail,
		Visibility:       visibility,
		Status:           status,
		BannedReason:     record.Document.BannedReason,
		BannedAt:         record.Document.BannedAt,
		DeletedAt:        record.Document.DeletedAt,
		CreatedAt:        record.Document.CreatedAt,
		UpdatedAt:        record.Document.UpdatedAt,
	}
}
