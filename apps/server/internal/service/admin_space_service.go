package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

const (
	defaultAdminSpacePage     = 1
	defaultAdminSpacePageSize = 20
	maxAdminSpacePageSize     = 100
)

var (
	ErrAdminSpaceInvalidStatusFilter     = errors.New("invalid admin space status filter")
	ErrAdminSpaceInvalidVisibilityFilter = errors.New("invalid admin space visibility filter")
	ErrAdminSpaceInvalidSpaceID          = errors.New("admin space id is invalid")
	ErrAdminSpaceInvalidName             = errors.New("admin space name is invalid")
	ErrAdminSpaceInvalidVisibility       = errors.New("admin space visibility is invalid")
	ErrAdminSpaceInvalidStatus           = errors.New("admin space status is invalid")
	ErrAdminSpaceBanReasonRequired       = errors.New("admin space ban reason is required")
	ErrAdminSpaceNoMetadataChange        = errors.New("admin space metadata change is required")
	ErrAdminSpaceNotFound                = errors.New("admin space target not found")
	ErrAdminSpaceAlreadyDeleted          = errors.New("admin space target already deleted")
)

// AdminSpaceStatusFilter 管理后台空间状态过滤条件。
type AdminSpaceStatusFilter string

const (
	AdminSpaceStatusFilterAll     AdminSpaceStatusFilter = "all"
	AdminSpaceStatusFilterActive  AdminSpaceStatusFilter = "active"
	AdminSpaceStatusFilterBanned  AdminSpaceStatusFilter = "banned"
	AdminSpaceStatusFilterDeleted AdminSpaceStatusFilter = "deleted"
)

// AdminSpaceVisibilityFilter 管理后台空间可见性过滤条件。
type AdminSpaceVisibilityFilter string

const (
	AdminSpaceVisibilityFilterAll           AdminSpaceVisibilityFilter = "all"
	AdminSpaceVisibilityFilterPublic        AdminSpaceVisibilityFilter = "public"
	AdminSpaceVisibilityFilterAuthenticated AdminSpaceVisibilityFilter = "authenticated"
	AdminSpaceVisibilityFilterMember        AdminSpaceVisibilityFilter = "member"
)

// AdminSpaceRecord 后台空间列表项。
type AdminSpaceRecord struct {
	SpaceID      string
	Name         string
	OwnerUserID  string
	OwnerName    string
	OwnerEmail   string
	Visibility   models.Visibility
	Status       models.EntityStatus
	BannedReason string
	BannedAt     *time.Time
	DeletedAt    *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ListAdminSpacesInput 后台空间列表查询参数。
type ListAdminSpacesInput struct {
	ActorUserID      string
	Keyword          string
	StatusFilter     AdminSpaceStatusFilter
	VisibilityFilter AdminSpaceVisibilityFilter
	Page             int
	PageSize         int
}

// ListAdminSpacesResult 后台空间列表返回结果。
type ListAdminSpacesResult struct {
	Items    []AdminSpaceRecord
	Page     int
	PageSize int
	Total    int64
}

// UpdateAdminSpaceMetadataInput 后台空间元数据更新参数。
type UpdateAdminSpaceMetadataInput struct {
	ActorUserID string
	RequestID   string
	SpaceID     string
	Name        *string
	Visibility  *models.Visibility
}

// UpdateAdminSpaceStatusInput 后台空间状态更新参数。
type UpdateAdminSpaceStatusInput struct {
	ActorUserID string
	RequestID   string
	SpaceID     string
	Status      models.EntityStatus
	Reason      string
}

// AdminSpaceService 封装空间管理业务。
type AdminSpaceService struct {
	spaceRepo          repository.SpaceRepository
	userRepo           repository.UserRepository
	adminAccessService *AdminAccessService
	adminAuditService  *AdminAuditService
}

// NewAdminSpaceService 创建后台空间管理服务。
func NewAdminSpaceService(
	spaceRepo repository.SpaceRepository,
	userRepo repository.UserRepository,
	adminAccessService *AdminAccessService,
	adminAuditService *AdminAuditService,
) *AdminSpaceService {
	return &AdminSpaceService{
		spaceRepo:          spaceRepo,
		userRepo:           userRepo,
		adminAccessService: adminAccessService,
		adminAuditService:  adminAuditService,
	}
}

// ListSpaces 查询后台空间列表。
func (s *AdminSpaceService) ListSpaces(
	ctx context.Context,
	input ListAdminSpacesInput,
) (ListAdminSpacesResult, error) {
	if s == nil || s.spaceRepo == nil || s.adminAccessService == nil {
		return ListAdminSpacesResult{}, errors.New("admin space service dependencies are nil")
	}

	actorUserID := strings.TrimSpace(input.ActorUserID)
	if actorUserID == "" {
		return ListAdminSpacesResult{}, ErrAdminForbidden
	}

	restrictToScopes, err := s.resolveScopeRestriction(ctx, actorUserID)
	if err != nil {
		return ListAdminSpacesResult{}, err
	}

	statuses, err := resolveAdminSpaceStatuses(input.StatusFilter)
	if err != nil {
		return ListAdminSpacesResult{}, err
	}
	visibilities, err := resolveAdminSpaceVisibilities(input.VisibilityFilter)
	if err != nil {
		return ListAdminSpacesResult{}, err
	}

	page, pageSize := normalizeAdminSpacePagination(input.Page, input.PageSize)
	records, total, err := s.spaceRepo.ListForAdmin(ctx, repository.ListAdminSpacesParams{
		ActorUserID:      actorUserID,
		RestrictToScopes: restrictToScopes,
		Keyword:          strings.TrimSpace(input.Keyword),
		Statuses:         statuses,
		Visibilities:     visibilities,
		Limit:            pageSize,
		Offset:           (page - 1) * pageSize,
	})
	if err != nil {
		return ListAdminSpacesResult{}, err
	}

	items := make([]AdminSpaceRecord, 0, len(records))
	for _, record := range records {
		items = append(items, mapAdminSpaceRecord(record))
	}

	return ListAdminSpacesResult{
		Items:    items,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

// UpdateMetadata 更新后台空间元数据（名称、可见性）。
func (s *AdminSpaceService) UpdateMetadata(
	ctx context.Context,
	input UpdateAdminSpaceMetadataInput,
) (AdminSpaceRecord, error) {
	if s == nil || s.spaceRepo == nil || s.adminAccessService == nil {
		return AdminSpaceRecord{}, errors.New("admin space service dependencies are nil")
	}

	actorUserID := strings.TrimSpace(input.ActorUserID)
	spaceID := strings.TrimSpace(input.SpaceID)
	if spaceID == "" {
		return AdminSpaceRecord{}, ErrAdminSpaceInvalidSpaceID
	}

	if err := s.ensureCanManageSpace(ctx, actorUserID, spaceID); err != nil {
		return AdminSpaceRecord{}, err
	}

	if input.Name == nil && input.Visibility == nil {
		return AdminSpaceRecord{}, ErrAdminSpaceNoMetadataChange
	}

	var normalizedName *string
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return AdminSpaceRecord{}, ErrAdminSpaceInvalidName
		}
		normalizedName = &name
	}

	var normalizedVisibility *models.Visibility
	if input.Visibility != nil {
		if !models.IsValidVisibility(*input.Visibility) {
			return AdminSpaceRecord{}, ErrAdminSpaceInvalidVisibility
		}
		visibility := *input.Visibility
		normalizedVisibility = &visibility
	}

	snapshot, err := s.spaceRepo.GetBySpaceID(ctx, spaceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminSpaceRecord{}, ErrAdminSpaceNotFound
		}
		return AdminSpaceRecord{}, err
	}
	if normalizeEntityStatus(snapshot.Status) == models.EntityStatusDeleted || snapshot.DeletedAt != nil {
		return AdminSpaceRecord{}, ErrAdminSpaceAlreadyDeleted
	}

	updated, err := s.spaceRepo.UpdateMetadata(ctx, repository.UpdateSpaceMetadataParams{
		SpaceID:    spaceID,
		Name:       normalizedName,
		Visibility: normalizedVisibility,
		UpdatedAt:  time.Now().UTC(),
	})
	if err != nil {
		return AdminSpaceRecord{}, err
	}
	if !updated {
		return AdminSpaceRecord{}, ErrAdminSpaceNotFound
	}

	latest, err := s.spaceRepo.GetBySpaceID(ctx, spaceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminSpaceRecord{}, ErrAdminSpaceNotFound
		}
		return AdminSpaceRecord{}, err
	}

	ownerName := ""
	ownerEmail := ""
	if s.userRepo != nil {
		ownerUser, ownerErr := s.userRepo.GetByUserID(ctx, latest.OwnerUserID)
		if ownerErr == nil && ownerUser != nil {
			ownerName = ownerUser.Name
			ownerEmail = ownerUser.Email
		}
	}

	record := AdminSpaceRecord{
		SpaceID:      latest.SpaceID,
		Name:         latest.Name,
		OwnerUserID:  latest.OwnerUserID,
		OwnerName:    ownerName,
		OwnerEmail:   ownerEmail,
		Visibility:   latest.Visibility,
		Status:       latest.Status,
		BannedReason: latest.BannedReason,
		BannedAt:     latest.BannedAt,
		DeletedAt:    latest.DeletedAt,
		CreatedAt:    latest.CreatedAt,
		UpdatedAt:    latest.UpdatedAt,
	}

	detail := map[string]any{
		"nameBefore":       snapshot.Name,
		"nameAfter":        record.Name,
		"visibilityBefore": snapshot.Visibility,
		"visibilityAfter":  record.Visibility,
	}
	if err := s.recordSpaceAudit(ctx, RecordAdminAuditInput{
		Module:     AdminAuditModuleSpace,
		Action:     AdminAuditActionUpdate,
		TargetType: "space",
		TargetID:   record.SpaceID,
		Summary:    "space metadata updated: " + record.SpaceID,
		Detail:     detail,
	}); err != nil {
		return AdminSpaceRecord{}, err
	}

	return record, nil
}

// UpdateStatus 更新空间状态（active/banned）。
func (s *AdminSpaceService) UpdateStatus(
	ctx context.Context,
	input UpdateAdminSpaceStatusInput,
) (AdminSpaceRecord, error) {
	if s == nil || s.spaceRepo == nil || s.adminAccessService == nil {
		return AdminSpaceRecord{}, errors.New("admin space service dependencies are nil")
	}

	actorUserID := strings.TrimSpace(input.ActorUserID)
	spaceID := strings.TrimSpace(input.SpaceID)
	if spaceID == "" {
		return AdminSpaceRecord{}, ErrAdminSpaceInvalidSpaceID
	}
	if input.Status != models.EntityStatusActive && input.Status != models.EntityStatusBanned {
		return AdminSpaceRecord{}, ErrAdminSpaceInvalidStatus
	}
	if input.Status == models.EntityStatusBanned && strings.TrimSpace(input.Reason) == "" {
		return AdminSpaceRecord{}, ErrAdminSpaceBanReasonRequired
	}

	if err := s.ensureCanManageSpace(ctx, actorUserID, spaceID); err != nil {
		return AdminSpaceRecord{}, err
	}

	snapshot, err := s.spaceRepo.GetBySpaceID(ctx, spaceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminSpaceRecord{}, ErrAdminSpaceNotFound
		}
		return AdminSpaceRecord{}, err
	}
	if normalizeEntityStatus(snapshot.Status) == models.EntityStatusDeleted || snapshot.DeletedAt != nil {
		return AdminSpaceRecord{}, ErrAdminSpaceAlreadyDeleted
	}

	now := time.Now().UTC()
	params := repository.UpdateSpaceStatusParams{
		SpaceID:   spaceID,
		Status:    input.Status,
		UpdatedAt: now,
	}
	if input.Status == models.EntityStatusBanned {
		params.BannedReason = strings.TrimSpace(input.Reason)
		params.BannedAt = &now
	}

	updated, err := s.spaceRepo.UpdateStatus(ctx, params)
	if err != nil {
		return AdminSpaceRecord{}, err
	}
	if !updated {
		return AdminSpaceRecord{}, ErrAdminSpaceNotFound
	}

	latest, err := s.spaceRepo.GetBySpaceID(ctx, spaceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminSpaceRecord{}, ErrAdminSpaceNotFound
		}
		return AdminSpaceRecord{}, err
	}

	ownerName := ""
	ownerEmail := ""
	if s.userRepo != nil {
		ownerUser, ownerErr := s.userRepo.GetByUserID(ctx, latest.OwnerUserID)
		if ownerErr == nil && ownerUser != nil {
			ownerName = ownerUser.Name
			ownerEmail = ownerUser.Email
		}
	}

	record := AdminSpaceRecord{
		SpaceID:      latest.SpaceID,
		Name:         latest.Name,
		OwnerUserID:  latest.OwnerUserID,
		OwnerName:    ownerName,
		OwnerEmail:   ownerEmail,
		Visibility:   latest.Visibility,
		Status:       latest.Status,
		BannedReason: latest.BannedReason,
		BannedAt:     latest.BannedAt,
		DeletedAt:    latest.DeletedAt,
		CreatedAt:    latest.CreatedAt,
		UpdatedAt:    latest.UpdatedAt,
	}

	if err := s.recordSpaceAudit(ctx, RecordAdminAuditInput{
		Module:     AdminAuditModuleSpace,
		Action:     AdminAuditActionUpdate,
		TargetType: "space",
		TargetID:   record.SpaceID,
		Summary:    "space status updated: " + record.SpaceID,
		Detail: map[string]any{
			"statusBefore": snapshot.Status,
			"statusAfter":  record.Status,
			"reason":       strings.TrimSpace(input.Reason),
		},
	}); err != nil {
		return AdminSpaceRecord{}, err
	}

	return record, nil
}

// DeleteSpace 软删除后台目标空间。
func (s *AdminSpaceService) DeleteSpace(
	ctx context.Context,
	actorUserID string,
	spaceID string,
	requestID string,
) error {
	_ = requestID

	if s == nil || s.spaceRepo == nil || s.adminAccessService == nil {
		return errors.New("admin space service dependencies are nil")
	}

	actor := strings.TrimSpace(actorUserID)
	targetSpaceID := strings.TrimSpace(spaceID)
	if targetSpaceID == "" {
		return ErrAdminSpaceInvalidSpaceID
	}

	if err := s.ensureCanManageSpace(ctx, actor, targetSpaceID); err != nil {
		return err
	}

	snapshot, err := s.spaceRepo.GetBySpaceID(ctx, targetSpaceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrAdminSpaceNotFound
		}
		return err
	}
	if normalizeEntityStatus(snapshot.Status) == models.EntityStatusDeleted || snapshot.DeletedAt != nil {
		return nil
	}

	deleted, err := s.spaceRepo.SoftDelete(ctx, targetSpaceID, time.Now().UTC())
	if err != nil {
		return err
	}
	if !deleted {
		return ErrAdminSpaceNotFound
	}

	if err := s.recordSpaceAudit(ctx, RecordAdminAuditInput{
		Module:     AdminAuditModuleSpace,
		Action:     AdminAuditActionDelete,
		TargetType: "space",
		TargetID:   targetSpaceID,
		Summary:    "space deleted: " + targetSpaceID,
		Detail: map[string]any{
			"statusBefore": snapshot.Status,
			"statusAfter":  models.EntityStatusDeleted,
		},
	}); err != nil {
		return err
	}

	return nil
}

func (s *AdminSpaceService) ensureCanManageSpace(ctx context.Context, actorUserID string, spaceID string) error {
	if strings.TrimSpace(actorUserID) == "" {
		return ErrAdminForbidden
	}
	canManage, err := s.adminAccessService.CanManageSpace(ctx, actorUserID, spaceID)
	if err != nil {
		return err
	}
	if !canManage {
		return ErrAdminForbidden
	}
	return nil
}

func (s *AdminSpaceService) resolveScopeRestriction(ctx context.Context, actorUserID string) (bool, error) {
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
		return false, ErrAdminForbidden
	}
	return true, nil
}

func (s *AdminSpaceService) recordSpaceAudit(ctx context.Context, input RecordAdminAuditInput) error {
	if s == nil || s.adminAuditService == nil {
		return nil
	}
	return s.adminAuditService.Record(ctx, input)
}

func resolveAdminSpaceStatuses(filter AdminSpaceStatusFilter) ([]models.EntityStatus, error) {
	switch normalizeAdminSpaceStatusFilter(filter) {
	case "":
		return []models.EntityStatus{models.EntityStatusActive, models.EntityStatusBanned}, nil
	case AdminSpaceStatusFilterAll:
		return []models.EntityStatus{
			models.EntityStatusActive,
			models.EntityStatusBanned,
			models.EntityStatusDeleted,
		}, nil
	case AdminSpaceStatusFilterActive:
		return []models.EntityStatus{models.EntityStatusActive}, nil
	case AdminSpaceStatusFilterBanned:
		return []models.EntityStatus{models.EntityStatusBanned}, nil
	case AdminSpaceStatusFilterDeleted:
		return []models.EntityStatus{models.EntityStatusDeleted}, nil
	default:
		return nil, ErrAdminSpaceInvalidStatusFilter
	}
}

func resolveAdminSpaceVisibilities(filter AdminSpaceVisibilityFilter) ([]models.Visibility, error) {
	switch normalizeAdminSpaceVisibilityFilter(filter) {
	case "", AdminSpaceVisibilityFilterAll:
		return []models.Visibility{
			models.VisibilityPublic,
			models.VisibilityAuthenticated,
			models.VisibilityMember,
		}, nil
	case AdminSpaceVisibilityFilterPublic:
		return []models.Visibility{models.VisibilityPublic}, nil
	case AdminSpaceVisibilityFilterAuthenticated:
		return []models.Visibility{models.VisibilityAuthenticated}, nil
	case AdminSpaceVisibilityFilterMember:
		return []models.Visibility{models.VisibilityMember}, nil
	default:
		return nil, ErrAdminSpaceInvalidVisibilityFilter
	}
}

func normalizeAdminSpaceStatusFilter(filter AdminSpaceStatusFilter) AdminSpaceStatusFilter {
	value := strings.ToLower(strings.TrimSpace(string(filter)))
	if value == "" {
		return ""
	}
	return AdminSpaceStatusFilter(value)
}

func normalizeAdminSpaceVisibilityFilter(filter AdminSpaceVisibilityFilter) AdminSpaceVisibilityFilter {
	value := strings.ToLower(strings.TrimSpace(string(filter)))
	if value == "" {
		return ""
	}
	return AdminSpaceVisibilityFilter(value)
}

func normalizeAdminSpacePagination(page int, pageSize int) (int, int) {
	normalizedPage := page
	if normalizedPage <= 0 {
		normalizedPage = defaultAdminSpacePage
	}

	normalizedPageSize := pageSize
	if normalizedPageSize <= 0 {
		normalizedPageSize = defaultAdminSpacePageSize
	}
	if normalizedPageSize > maxAdminSpacePageSize {
		normalizedPageSize = maxAdminSpacePageSize
	}

	return normalizedPage, normalizedPageSize
}

func mapAdminSpaceRecord(record repository.AdminSpaceListRecord) AdminSpaceRecord {
	space := record.Space
	visibility := space.Visibility
	if !models.IsValidVisibility(visibility) {
		visibility = models.VisibilityMember
	}
	status := space.Status
	if !models.IsValidEntityStatus(status) {
		status = models.EntityStatusActive
	}

	return AdminSpaceRecord{
		SpaceID:      space.SpaceID,
		Name:         space.Name,
		OwnerUserID:  space.OwnerUserID,
		OwnerName:    record.OwnerName,
		OwnerEmail:   record.OwnerEmail,
		Visibility:   visibility,
		Status:       status,
		BannedReason: space.BannedReason,
		BannedAt:     space.BannedAt,
		DeletedAt:    space.DeletedAt,
		CreatedAt:    space.CreatedAt,
		UpdatedAt:    space.UpdatedAt,
	}
}
