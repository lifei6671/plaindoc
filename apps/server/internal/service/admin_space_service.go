package service

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/errcode"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"
)

const (
	defaultAdminSpacePage     = 1
	defaultAdminSpacePageSize = 20
	maxAdminSpacePageSize     = 100
	maxAdminSpaceIDLength     = 26
	maxAdminSpaceNameLength   = 120
	maxAdminSpaceDescLength   = 280
	maxAdminSpaceCategoryLen  = 40
)

var adminSpaceIDPattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

// AdminSpaceCoverSource 定义空间封面来源。
type AdminSpaceCoverSource string

const (
	AdminSpaceCoverSourceUserUpload     AdminSpaceCoverSource = "user_upload"
	AdminSpaceCoverSourceSystemGenerate AdminSpaceCoverSource = "system_generated"
)

// AdminSpaceCoverAsset 后台封面资产。
type AdminSpaceCoverAsset struct {
	AssetID     string
	Key         string
	URL         string
	Width       int
	Height      int
	MimeType    string
	SizeBytes   int64
	Normalized  bool
	Source      AdminSpaceCoverSource
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CreatedByID string
}

// CreateAdminSpaceInput 后台新建空间参数。
type CreateAdminSpaceInput struct {
	ActorUserID  string
	RequestID    string
	SpaceID      string
	Name         string
	Description  string
	CategoryID   string
	Visibility   models.Visibility
	CoverAssetID string
}

// CreateAdminSpaceCoverAssetInput 后台创建空间封面资产参数。
type CreateAdminSpaceCoverAssetInput struct {
	ActorUserID      string
	Source           AdminSpaceCoverSource
	FileName         string
	FileContentType  string
	FileBytes        []byte
	SpaceName        string
	ClientWidth      int
	ClientHeight     int
	ClientMimeType   string
	ClientProcessed  bool
	PreferredQuality float64
}

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
	SpaceID           string
	Name              string
	Description       string
	CategoryID        string
	Category          string
	CategoryIsDefault bool
	OwnerUserID       string
	OwnerName         string
	OwnerEmail        string
	Visibility        models.Visibility
	Cover             *AdminSpaceCoverAsset
	Status            models.EntityStatus
	BannedReason      string
	BannedAt          *time.Time
	DeletedAt         *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// AdminSpaceMemberRecord 后台空间成员列表项。
type AdminSpaceMemberRecord struct {
	UserID    string
	Email     string
	Name      string
	Role      models.Role
	IsOwner   bool
	CreatedAt time.Time
	UpdatedAt time.Time
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

// AdminSpaceCategoryRecord 后台空间分类记录。
type AdminSpaceCategoryRecord struct {
	CategoryID string
	Name       string
	IsDefault  bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ListAdminSpaceMembersInput 后台空间成员列表查询参数。
type ListAdminSpaceMembersInput struct {
	ActorUserID string
	SpaceID     string
}

// UpsertAdminSpaceMemberInput 后台空间成员新增参数。
type UpsertAdminSpaceMemberInput struct {
	ActorUserID  string
	RequestID    string
	SpaceID      string
	TargetUserID string
	TargetEmail  string
	Role         models.Role
}

// UpdateAdminSpaceMemberRoleInput 后台空间成员角色更新参数。
type UpdateAdminSpaceMemberRoleInput struct {
	ActorUserID string
	RequestID   string
	SpaceID     string
	UserID      string
	Role        models.Role
}

// DeleteAdminSpaceMemberInput 后台空间成员删除参数。
type DeleteAdminSpaceMemberInput struct {
	ActorUserID string
	RequestID   string
	SpaceID     string
	UserID      string
}

// UpdateAdminSpaceMetadataInput 后台空间元数据更新参数。
type UpdateAdminSpaceMetadataInput struct {
	ActorUserID  string
	RequestID    string
	SpaceID      string
	Name         *string
	Description  *string
	CategoryID   *string
	Visibility   *models.Visibility
	CoverAssetID *string
}

// CreateAdminSpaceCategoryInput 后台创建空间分类参数。
type CreateAdminSpaceCategoryInput struct {
	ActorUserID string
	RequestID   string
	Name        string
}

// RenameAdminSpaceCategoryInput 后台重命名空间分类参数。
type RenameAdminSpaceCategoryInput struct {
	ActorUserID string
	RequestID   string
	CategoryID  string
	Name        string
}

// DeleteAdminSpaceCategoryInput 后台删除空间分类参数。
type DeleteAdminSpaceCategoryInput struct {
	ActorUserID string
	RequestID   string
	CategoryID  string
}

// DeleteAdminSpaceCategoryResult 后台删除空间分类结果。
type DeleteAdminSpaceCategoryResult struct {
	CategoryID             string
	ReassignedToCategoryID string
	ReassignedToName       string
	MovedSpaceCount        int64
}

// TransferAdminSpaceOwnershipInput 后台空间转让参数。
type TransferAdminSpaceOwnershipInput struct {
	ActorUserID  string
	RequestID    string
	SpaceID      string
	TargetUserID string
	TargetEmail  string
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
	spaceCategoryRepo  repository.SpaceCategoryRepository
	userRepo           repository.UserRepository
	adminRoleRepo      repository.AdminRoleRepository
	spaceScopeRepo     repository.SpaceAdminScopeRepository
	systemConfigRepo   repository.SystemConfigRepository
	adminAccessService *AdminAccessService
	adminAuditService  *AdminAuditService
}

// NewAdminSpaceService 创建后台空间管理服务。
func NewAdminSpaceService(
	spaceRepo repository.SpaceRepository,
	spaceCategoryRepo repository.SpaceCategoryRepository,
	userRepo repository.UserRepository,
	adminRoleRepo repository.AdminRoleRepository,
	spaceScopeRepo repository.SpaceAdminScopeRepository,
	systemConfigRepo repository.SystemConfigRepository,
	adminAccessService *AdminAccessService,
	adminAuditService *AdminAuditService,
) *AdminSpaceService {
	return &AdminSpaceService{
		spaceRepo:          spaceRepo,
		spaceCategoryRepo:  spaceCategoryRepo,
		userRepo:           userRepo,
		adminRoleRepo:      adminRoleRepo,
		spaceScopeRepo:     spaceScopeRepo,
		systemConfigRepo:   systemConfigRepo,
		adminAccessService: adminAccessService,
		adminAuditService:  adminAuditService,
	}
}

// ListSpaces 查询后台空间列表。
func (s *AdminSpaceService) ListSpaces(
	ctx context.Context,
	input ListAdminSpacesInput,
) (result ListAdminSpacesResult, err error) {
	defer func() {
		err = errcode.MapAdminSpaceError(err)
	}()

	if s == nil || s.spaceRepo == nil || s.adminAccessService == nil {
		return ListAdminSpacesResult{}, errors.New("admin space service dependencies are nil")
	}

	actorUserID := strings.TrimSpace(input.ActorUserID)
	if actorUserID == "" {
		return ListAdminSpacesResult{}, errcode.ErrAdminForbidden
	}

	isPlatformAdmin, err := s.adminAccessService.IsPlatformAdmin(ctx, actorUserID)
	if err != nil {
		return ListAdminSpacesResult{}, err
	}
	isAdmin := isPlatformAdmin
	if !isPlatformAdmin {
		isAdmin, err = s.adminAccessService.IsSpaceAdmin(ctx, actorUserID)
		if err != nil {
			return ListAdminSpacesResult{}, err
		}
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
	// 管理员继续走原有空间治理视图，普通登录用户则切换到“我参与的空间”视图。
	params := repository.ListAdminSpacesParams{
		ActorUserID:       actorUserID,
		Keyword:           strings.TrimSpace(input.Keyword),
		Statuses:          statuses,
		Visibilities:      visibilities,
		Limit:             pageSize,
		Offset:            (page - 1) * pageSize,
		RestrictToMembers: !isAdmin,
	}
	if isAdmin && !isPlatformAdmin {
		params.RestrictToScopes = true
	}

	records, total, err := s.spaceRepo.ListForAdmin(ctx, params)
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

// ListCategories 查询后台空间分类列表。
func (s *AdminSpaceService) ListCategories(
	ctx context.Context,
	actorUserID string,
) (result []AdminSpaceCategoryRecord, err error) {
	defer func() {
		err = errcode.MapAdminSpaceError(err)
	}()

	if s == nil || s.spaceCategoryRepo == nil || s.adminAccessService == nil {
		return nil, errors.New("admin space service dependencies are nil")
	}
	actor := strings.TrimSpace(actorUserID)
	if actor == "" {
		return nil, errcode.ErrAdminForbidden
	}
	isAdmin, err := s.adminAccessService.IsAdmin(ctx, actor)
	if err != nil {
		return nil, err
	}
	if !isAdmin {
		return nil, errcode.ErrAdminForbidden
	}

	items, err := s.spaceCategoryRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	result = make([]AdminSpaceCategoryRecord, 0, len(items))
	for _, item := range items {
		result = append(result, AdminSpaceCategoryRecord{
			CategoryID: strings.TrimSpace(item.CategoryID),
			Name:       strings.TrimSpace(item.Name),
			IsDefault:  item.IsDefault,
			CreatedAt:  item.CreatedAt,
			UpdatedAt:  item.UpdatedAt,
		})
	}
	return result, nil
}

// CreateCategory 新增后台空间分类。
func (s *AdminSpaceService) CreateCategory(
	ctx context.Context,
	input CreateAdminSpaceCategoryInput,
) (result AdminSpaceCategoryRecord, err error) {
	defer func() {
		err = errcode.MapAdminSpaceError(err)
	}()

	if s == nil || s.spaceCategoryRepo == nil || s.adminAccessService == nil {
		return AdminSpaceCategoryRecord{}, errors.New("admin space service dependencies are nil")
	}

	actorUserID := strings.TrimSpace(input.ActorUserID)
	if actorUserID == "" {
		return AdminSpaceCategoryRecord{}, errcode.ErrAdminForbidden
	}
	isAdmin, err := s.adminAccessService.IsAdmin(ctx, actorUserID)
	if err != nil {
		return AdminSpaceCategoryRecord{}, err
	}
	if !isAdmin {
		return AdminSpaceCategoryRecord{}, errcode.ErrAdminForbidden
	}

	name, err := normalizeAdminSpaceCategoryName(input.Name)
	if err != nil {
		return AdminSpaceCategoryRecord{}, err
	}
	existing, err := s.spaceCategoryRepo.GetByName(ctx, name)
	switch {
	case err == nil && existing != nil:
		return AdminSpaceCategoryRecord{}, errcode.ErrAdminSpaceCategoryNameConflict
	case err != nil && !errors.Is(err, gorm.ErrRecordNotFound):
		return AdminSpaceCategoryRecord{}, err
	}

	now := time.Now().UTC()
	category := &models.SpaceCategory{
		CategoryID: strings.ToLower(ulid.Make().String()),
		Name:       name,
		IsDefault:  false,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.spaceCategoryRepo.Create(ctx, category); err != nil {
		return AdminSpaceCategoryRecord{}, err
	}

	record := AdminSpaceCategoryRecord{
		CategoryID: category.CategoryID,
		Name:       category.Name,
		IsDefault:  category.IsDefault,
		CreatedAt:  category.CreatedAt,
		UpdatedAt:  category.UpdatedAt,
	}

	if err := s.recordSpaceAudit(ctx, RecordAdminAuditInput{
		Module:     AdminAuditModuleSpace,
		Action:     AdminAuditActionCreate,
		TargetType: "space_category",
		TargetID:   record.CategoryID,
		Summary:    "space category created: " + record.CategoryID,
		Detail: map[string]any{
			"categoryId": record.CategoryID,
			"name":       record.Name,
		},
	}); err != nil {
		return AdminSpaceCategoryRecord{}, err
	}

	return record, nil
}

// RenameCategory 重命名后台空间分类。
func (s *AdminSpaceService) RenameCategory(
	ctx context.Context,
	input RenameAdminSpaceCategoryInput,
) (result AdminSpaceCategoryRecord, err error) {
	defer func() {
		err = errcode.MapAdminSpaceError(err)
	}()

	if s == nil || s.spaceCategoryRepo == nil || s.adminAccessService == nil {
		return AdminSpaceCategoryRecord{}, errors.New("admin space service dependencies are nil")
	}

	actorUserID := strings.TrimSpace(input.ActorUserID)
	if actorUserID == "" {
		return AdminSpaceCategoryRecord{}, errcode.ErrAdminForbidden
	}
	isAdmin, err := s.adminAccessService.IsAdmin(ctx, actorUserID)
	if err != nil {
		return AdminSpaceCategoryRecord{}, err
	}
	if !isAdmin {
		return AdminSpaceCategoryRecord{}, errcode.ErrAdminForbidden
	}

	categoryID := strings.TrimSpace(input.CategoryID)
	if categoryID == "" {
		return AdminSpaceCategoryRecord{}, errcode.ErrAdminSpaceInvalidCategory
	}
	target, err := s.spaceCategoryRepo.GetByCategoryID(ctx, categoryID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminSpaceCategoryRecord{}, errcode.ErrAdminSpaceCategoryNotFound
		}
		return AdminSpaceCategoryRecord{}, err
	}
	if target.IsDefault {
		return AdminSpaceCategoryRecord{}, errcode.ErrAdminSpaceCategoryDefaultImmutable
	}

	name, err := normalizeAdminSpaceCategoryName(input.Name)
	if err != nil {
		return AdminSpaceCategoryRecord{}, err
	}
	if strings.EqualFold(strings.TrimSpace(target.Name), name) {
		return AdminSpaceCategoryRecord{
			CategoryID: target.CategoryID,
			Name:       target.Name,
			IsDefault:  target.IsDefault,
			CreatedAt:  target.CreatedAt,
			UpdatedAt:  target.UpdatedAt,
		}, nil
	}

	existing, err := s.spaceCategoryRepo.GetByName(ctx, name)
	switch {
	case err == nil && existing != nil && strings.TrimSpace(existing.CategoryID) != categoryID:
		return AdminSpaceCategoryRecord{}, errcode.ErrAdminSpaceCategoryNameConflict
	case err != nil && !errors.Is(err, gorm.ErrRecordNotFound):
		return AdminSpaceCategoryRecord{}, err
	}

	updated, err := s.spaceCategoryRepo.RenameAndSyncSpaces(ctx, categoryID, name, time.Now().UTC())
	if err != nil {
		return AdminSpaceCategoryRecord{}, err
	}
	if !updated {
		return AdminSpaceCategoryRecord{}, errcode.ErrAdminSpaceCategoryNotFound
	}
	latest, err := s.spaceCategoryRepo.GetByCategoryID(ctx, categoryID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminSpaceCategoryRecord{}, errcode.ErrAdminSpaceCategoryNotFound
		}
		return AdminSpaceCategoryRecord{}, err
	}

	record := AdminSpaceCategoryRecord{
		CategoryID: latest.CategoryID,
		Name:       latest.Name,
		IsDefault:  latest.IsDefault,
		CreatedAt:  latest.CreatedAt,
		UpdatedAt:  latest.UpdatedAt,
	}
	if err := s.recordSpaceAudit(ctx, RecordAdminAuditInput{
		Module:     AdminAuditModuleSpace,
		Action:     AdminAuditActionUpdate,
		TargetType: "space_category",
		TargetID:   record.CategoryID,
		Summary:    "space category renamed: " + record.CategoryID,
		Detail: gin.H{
			"categoryId": record.CategoryID,
			"nameBefore": target.Name,
			"nameAfter":  record.Name,
		},
	}); err != nil {
		return AdminSpaceCategoryRecord{}, err
	}

	return record, nil
}

// DeleteCategory 删除后台空间分类，并将关联空间移动到“未分类”。
func (s *AdminSpaceService) DeleteCategory(
	ctx context.Context,
	input DeleteAdminSpaceCategoryInput,
) (result DeleteAdminSpaceCategoryResult, err error) {
	defer func() {
		err = errcode.MapAdminSpaceError(err)
	}()

	if s == nil || s.spaceCategoryRepo == nil || s.adminAccessService == nil {
		return DeleteAdminSpaceCategoryResult{}, errors.New("admin space service dependencies are nil")
	}

	actorUserID := strings.TrimSpace(input.ActorUserID)
	if actorUserID == "" {
		return DeleteAdminSpaceCategoryResult{}, errcode.ErrAdminForbidden
	}
	isAdmin, err := s.adminAccessService.IsAdmin(ctx, actorUserID)
	if err != nil {
		return DeleteAdminSpaceCategoryResult{}, err
	}
	if !isAdmin {
		return DeleteAdminSpaceCategoryResult{}, errcode.ErrAdminForbidden
	}

	categoryID := strings.TrimSpace(input.CategoryID)
	if categoryID == "" {
		return DeleteAdminSpaceCategoryResult{}, errcode.ErrAdminSpaceInvalidCategory
	}
	target, err := s.spaceCategoryRepo.GetByCategoryID(ctx, categoryID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return DeleteAdminSpaceCategoryResult{}, errcode.ErrAdminSpaceCategoryNotFound
		}
		return DeleteAdminSpaceCategoryResult{}, err
	}
	if target.IsDefault {
		return DeleteAdminSpaceCategoryResult{}, errcode.ErrAdminSpaceCategoryDefaultImmutable
	}

	defaultCategory, err := s.getDefaultSpaceCategory(ctx)
	if err != nil {
		return DeleteAdminSpaceCategoryResult{}, err
	}
	if strings.TrimSpace(defaultCategory.CategoryID) == categoryID {
		return DeleteAdminSpaceCategoryResult{}, errcode.ErrAdminSpaceCategoryDefaultImmutable
	}

	movedCount, deleted, err := s.spaceCategoryRepo.DeleteAndReassignSpaces(
		ctx,
		categoryID,
		defaultCategory.CategoryID,
		defaultCategory.Name,
		time.Now().UTC(),
	)
	if err != nil {
		return DeleteAdminSpaceCategoryResult{}, err
	}
	if !deleted {
		return DeleteAdminSpaceCategoryResult{}, errcode.ErrAdminSpaceCategoryNotFound
	}

	result = DeleteAdminSpaceCategoryResult{
		CategoryID:             categoryID,
		ReassignedToCategoryID: defaultCategory.CategoryID,
		ReassignedToName:       defaultCategory.Name,
		MovedSpaceCount:        movedCount,
	}
	if err := s.recordSpaceAudit(ctx, RecordAdminAuditInput{
		Module:     AdminAuditModuleSpace,
		Action:     AdminAuditActionDelete,
		TargetType: "space_category",
		TargetID:   categoryID,
		Summary:    "space category deleted: " + categoryID,
		Detail: gin.H{
			"categoryId":             categoryID,
			"name":                   target.Name,
			"movedSpaceCount":        movedCount,
			"reassignedToCategoryId": defaultCategory.CategoryID,
			"reassignedToName":       defaultCategory.Name,
		},
	}); err != nil {
		return DeleteAdminSpaceCategoryResult{}, err
	}

	return result, nil
}

// ListMembers 查询后台空间成员列表。
func (s *AdminSpaceService) ListMembers(
	ctx context.Context,
	input ListAdminSpaceMembersInput,
) (result []AdminSpaceMemberRecord, err error) {
	defer func() {
		err = errcode.MapAdminSpaceError(err)
	}()

	if s == nil || s.spaceRepo == nil || s.adminAccessService == nil {
		return nil, errors.New("admin space service dependencies are nil")
	}

	actorUserID := strings.TrimSpace(input.ActorUserID)
	spaceID := strings.TrimSpace(input.SpaceID)
	if spaceID == "" {
		return nil, errcode.ErrAdminSpaceInvalidSpaceID
	}

	if err := s.ensureCanManageSpace(ctx, actorUserID, spaceID); err != nil {
		return nil, err
	}

	spaceSnapshot, err := s.spaceRepo.GetBySpaceID(ctx, spaceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.ErrAdminSpaceNotFound
		}
		return nil, err
	}
	if normalizeEntityStatus(spaceSnapshot.Status) == models.EntityStatusDeleted || spaceSnapshot.DeletedAt != nil {
		return nil, errcode.ErrAdminSpaceAlreadyDeleted
	}

	members, err := s.spaceRepo.ListMembers(ctx, spaceID)
	if err != nil {
		return nil, err
	}

	return s.hydrateSpaceMembers(ctx, spaceSnapshot, members), nil
}

// UpsertMember 新增空间成员（已存在则更新角色）。
func (s *AdminSpaceService) UpsertMember(
	ctx context.Context,
	input UpsertAdminSpaceMemberInput,
) (result AdminSpaceMemberRecord, err error) {
	defer func() {
		err = errcode.MapAdminSpaceError(err)
	}()

	if s == nil || s.spaceRepo == nil || s.userRepo == nil || s.adminAccessService == nil {
		return AdminSpaceMemberRecord{}, errors.New("admin space service dependencies are nil")
	}

	actorUserID := strings.TrimSpace(input.ActorUserID)
	spaceID := strings.TrimSpace(input.SpaceID)
	if spaceID == "" {
		return AdminSpaceMemberRecord{}, errcode.ErrAdminSpaceInvalidSpaceID
	}
	if err := s.ensureCanManageSpace(ctx, actorUserID, spaceID); err != nil {
		return AdminSpaceMemberRecord{}, err
	}

	memberRole := normalizeEditableAdminSpaceMemberRole(input.Role)
	if memberRole == "" {
		return AdminSpaceMemberRecord{}, errcode.ErrAdminSpaceMemberInvalidRole
	}

	spaceSnapshot, err := s.spaceRepo.GetBySpaceID(ctx, spaceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminSpaceMemberRecord{}, errcode.ErrAdminSpaceNotFound
		}
		return AdminSpaceMemberRecord{}, err
	}
	if normalizeEntityStatus(spaceSnapshot.Status) == models.EntityStatusDeleted || spaceSnapshot.DeletedAt != nil {
		return AdminSpaceMemberRecord{}, errcode.ErrAdminSpaceAlreadyDeleted
	}

	targetUser, err := s.resolveTargetUser(ctx, input.TargetUserID, input.TargetEmail)
	if err != nil {
		return AdminSpaceMemberRecord{}, err
	}
	targetUserID := strings.TrimSpace(targetUser.UserID)
	if targetUserID == strings.TrimSpace(spaceSnapshot.OwnerUserID) {
		return AdminSpaceMemberRecord{}, errcode.ErrAdminSpaceMemberOwnerImmutable
	}

	now := time.Now().UTC()
	if err := s.spaceRepo.UpsertMember(ctx, repository.UpsertSpaceMemberParams{
		SpaceID:   spaceID,
		UserID:    targetUserID,
		Role:      memberRole,
		UpdatedAt: now,
	}); err != nil {
		return AdminSpaceMemberRecord{}, err
	}

	members, err := s.spaceRepo.ListMembers(ctx, spaceID)
	if err != nil {
		return AdminSpaceMemberRecord{}, err
	}
	memberRecord, found := findAdminSpaceMemberRecord(s.hydrateSpaceMembers(ctx, spaceSnapshot, members), targetUserID)
	if !found {
		return AdminSpaceMemberRecord{}, errcode.ErrAdminSpaceMemberNotFound
	}

	if err := s.recordSpaceAudit(ctx, RecordAdminAuditInput{
		Module:     AdminAuditModuleSpace,
		Action:     AdminAuditActionUpdate,
		TargetType: "space_member",
		TargetID:   spaceID + ":" + targetUserID,
		Summary:    "space member upserted: " + spaceID + ":" + targetUserID,
		Detail: gin.H{
			"spaceId":   spaceID,
			"userId":    targetUserID,
			"userEmail": strings.TrimSpace(targetUser.Email),
			"role":      memberRole,
		},
	}); err != nil {
		return AdminSpaceMemberRecord{}, err
	}

	return memberRecord, nil
}

// UpdateMemberRole 更新空间成员角色。
func (s *AdminSpaceService) UpdateMemberRole(
	ctx context.Context,
	input UpdateAdminSpaceMemberRoleInput,
) (result AdminSpaceMemberRecord, err error) {
	defer func() {
		err = errcode.MapAdminSpaceError(err)
	}()

	if s == nil || s.spaceRepo == nil || s.userRepo == nil || s.adminAccessService == nil {
		return AdminSpaceMemberRecord{}, errors.New("admin space service dependencies are nil")
	}

	actorUserID := strings.TrimSpace(input.ActorUserID)
	spaceID := strings.TrimSpace(input.SpaceID)
	memberUserID := strings.TrimSpace(input.UserID)
	if spaceID == "" {
		return AdminSpaceMemberRecord{}, errcode.ErrAdminSpaceInvalidSpaceID
	}
	if memberUserID == "" {
		return AdminSpaceMemberRecord{}, errcode.ErrAdminSpaceMemberTargetRequired
	}
	if err := s.ensureCanManageSpace(ctx, actorUserID, spaceID); err != nil {
		return AdminSpaceMemberRecord{}, err
	}

	memberRole := normalizeEditableAdminSpaceMemberRole(input.Role)
	if memberRole == "" {
		return AdminSpaceMemberRecord{}, errcode.ErrAdminSpaceMemberInvalidRole
	}

	spaceSnapshot, err := s.spaceRepo.GetBySpaceID(ctx, spaceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminSpaceMemberRecord{}, errcode.ErrAdminSpaceNotFound
		}
		return AdminSpaceMemberRecord{}, err
	}
	if normalizeEntityStatus(spaceSnapshot.Status) == models.EntityStatusDeleted || spaceSnapshot.DeletedAt != nil {
		return AdminSpaceMemberRecord{}, errcode.ErrAdminSpaceAlreadyDeleted
	}
	if memberUserID == strings.TrimSpace(spaceSnapshot.OwnerUserID) {
		return AdminSpaceMemberRecord{}, errcode.ErrAdminSpaceMemberOwnerImmutable
	}

	updated, err := s.spaceRepo.UpdateMemberRole(ctx, repository.UpdateSpaceMemberRoleParams{
		SpaceID:   spaceID,
		UserID:    memberUserID,
		Role:      memberRole,
		UpdatedAt: time.Now().UTC(),
	})
	if err != nil {
		return AdminSpaceMemberRecord{}, err
	}
	if !updated {
		return AdminSpaceMemberRecord{}, errcode.ErrAdminSpaceMemberNotFound
	}

	members, err := s.spaceRepo.ListMembers(ctx, spaceID)
	if err != nil {
		return AdminSpaceMemberRecord{}, err
	}
	memberRecord, found := findAdminSpaceMemberRecord(s.hydrateSpaceMembers(ctx, spaceSnapshot, members), memberUserID)
	if !found {
		return AdminSpaceMemberRecord{}, errcode.ErrAdminSpaceMemberNotFound
	}

	if err := s.recordSpaceAudit(ctx, RecordAdminAuditInput{
		Module:     AdminAuditModuleSpace,
		Action:     AdminAuditActionUpdate,
		TargetType: "space_member",
		TargetID:   spaceID + ":" + memberUserID,
		Summary:    "space member role updated: " + spaceID + ":" + memberUserID,
		Detail: map[string]any{
			"spaceId": spaceID,
			"userId":  memberUserID,
			"role":    memberRole,
		},
	}); err != nil {
		return AdminSpaceMemberRecord{}, err
	}

	return memberRecord, nil
}

// DeleteMember 删除空间成员。
func (s *AdminSpaceService) DeleteMember(
	ctx context.Context,
	input DeleteAdminSpaceMemberInput,
) (err error) {
	defer func() {
		err = errcode.MapAdminSpaceError(err)
	}()

	if s == nil || s.spaceRepo == nil || s.adminAccessService == nil {
		return errors.New("admin space service dependencies are nil")
	}

	actorUserID := strings.TrimSpace(input.ActorUserID)
	spaceID := strings.TrimSpace(input.SpaceID)
	memberUserID := strings.TrimSpace(input.UserID)
	if spaceID == "" {
		return errcode.ErrAdminSpaceInvalidSpaceID
	}
	if memberUserID == "" {
		return errcode.ErrAdminSpaceMemberTargetRequired
	}
	if err := s.ensureCanManageSpace(ctx, actorUserID, spaceID); err != nil {
		return err
	}

	spaceSnapshot, err := s.spaceRepo.GetBySpaceID(ctx, spaceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.ErrAdminSpaceNotFound
		}
		return err
	}
	if normalizeEntityStatus(spaceSnapshot.Status) == models.EntityStatusDeleted || spaceSnapshot.DeletedAt != nil {
		return errcode.ErrAdminSpaceAlreadyDeleted
	}
	if memberUserID == strings.TrimSpace(spaceSnapshot.OwnerUserID) {
		return errcode.ErrAdminSpaceMemberOwnerImmutable
	}

	deleted, err := s.spaceRepo.DeleteMember(ctx, spaceID, memberUserID)
	if err != nil {
		return err
	}
	if !deleted {
		return errcode.ErrAdminSpaceMemberNotFound
	}

	if err := s.recordSpaceAudit(ctx, RecordAdminAuditInput{
		Module:     AdminAuditModuleSpace,
		Action:     AdminAuditActionDelete,
		TargetType: "space_member",
		TargetID:   spaceID + ":" + memberUserID,
		Summary:    "space member removed: " + spaceID + ":" + memberUserID,
		Detail: map[string]any{
			"spaceId": spaceID,
			"userId":  memberUserID,
		},
	}); err != nil {
		return err
	}
	return nil
}

// CreateSpace 后台创建空间。
func (s *AdminSpaceService) CreateSpace(
	ctx context.Context,
	input CreateAdminSpaceInput,
) (result AdminSpaceRecord, err error) {
	defer func() {
		err = errcode.MapAdminSpaceError(err)
	}()

	if s == nil || s.spaceRepo == nil || s.adminAccessService == nil {
		return AdminSpaceRecord{}, errors.New("admin space service dependencies are nil")
	}

	actorUserID := strings.TrimSpace(input.ActorUserID)
	if actorUserID == "" {
		return AdminSpaceRecord{}, errcode.ErrAdminForbidden
	}
	isAdmin, err := s.adminAccessService.IsAdmin(ctx, actorUserID)
	if err != nil {
		return AdminSpaceRecord{}, err
	}
	if !isAdmin {
		return AdminSpaceRecord{}, errcode.ErrAdminForbidden
	}

	spaceID, hasCustomSpaceID, err := normalizeAdminSpaceID(input.SpaceID)
	if err != nil {
		return AdminSpaceRecord{}, err
	}
	if hasCustomSpaceID {
		existingSpace, err := s.spaceRepo.GetBySpaceID(ctx, spaceID)
		if err == nil && existingSpace != nil {
			return AdminSpaceRecord{}, errcode.ErrAdminSpaceAlreadyExists
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminSpaceRecord{}, err
		}
	} else {
		spaceID = strings.ToLower(ulid.Make().String())
	}

	name := strings.TrimSpace(input.Name)
	if name == "" || len([]rune(name)) > maxAdminSpaceNameLength {
		return AdminSpaceRecord{}, errcode.ErrAdminSpaceInvalidName
	}

	description := strings.TrimSpace(input.Description)
	if len([]rune(description)) > maxAdminSpaceDescLength {
		return AdminSpaceRecord{}, errcode.ErrAdminSpaceInvalidDescription
	}
	resolvedCategory, err := s.resolveSpaceCategoryByID(ctx, input.CategoryID)
	if err != nil {
		return AdminSpaceRecord{}, err
	}

	visibility := input.Visibility
	if !models.IsValidVisibility(visibility) {
		if strings.TrimSpace(string(visibility)) != "" {
			return AdminSpaceRecord{}, errcode.ErrAdminSpaceInvalidVisibility
		}
		visibility, err = s.resolveDefaultSpaceVisibility(ctx)
		if err != nil {
			return AdminSpaceRecord{}, err
		}
	}

	var (
		coverAssetID *string
		coverKey     string
		coverURL     string
		coverWidth   int
		coverHeight  int
		coverSource  string
	)
	if trimmedCoverAssetID := strings.TrimSpace(input.CoverAssetID); trimmedCoverAssetID != "" {
		coverAsset, err := s.spaceRepo.GetCoverAssetByAssetID(ctx, trimmedCoverAssetID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return AdminSpaceRecord{}, errcode.ErrAdminSpaceCoverAssetNotFound
			}
			return AdminSpaceRecord{}, err
		}

		assetID := strings.TrimSpace(coverAsset.AssetID)
		coverAssetID = &assetID
		coverKey = coverAsset.ObjectKey
		coverURL = coverAsset.ObjectURL
		coverWidth = coverAsset.Width
		coverHeight = coverAsset.Height
		coverSource = coverAsset.Source
	}

	now := time.Now().UTC()
	space := &models.Space{
		SpaceID:      spaceID,
		Name:         name,
		Description:  description,
		CategoryID:   resolvedCategory.CategoryID,
		Category:     resolvedCategory.Name,
		OwnerUserID:  actorUserID,
		Visibility:   visibility,
		CoverAssetID: coverAssetID,
		CoverKey:     coverKey,
		CoverURL:     coverURL,
		CoverWidth:   coverWidth,
		CoverHeight:  coverHeight,
		CoverSource:  coverSource,
		Status:       models.EntityStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.spaceRepo.Create(ctx, space); err != nil {
		return AdminSpaceRecord{}, err
	}

	ownerName := ""
	ownerEmail := ""
	if s.userRepo != nil {
		owner, ownerErr := s.userRepo.GetByUserID(ctx, actorUserID)
		if ownerErr == nil && owner != nil {
			ownerName = owner.Name
			ownerEmail = owner.Email
		}
	}

	record := mapAdminSpaceRecord(repository.AdminSpaceListRecord{
		Space: models.Space{
			ID:           space.ID,
			SpaceID:      space.SpaceID,
			Name:         space.Name,
			Description:  space.Description,
			CategoryID:   space.CategoryID,
			Category:     space.Category,
			OwnerUserID:  space.OwnerUserID,
			Visibility:   space.Visibility,
			CoverAssetID: space.CoverAssetID,
			CoverKey:     space.CoverKey,
			CoverURL:     space.CoverURL,
			CoverWidth:   space.CoverWidth,
			CoverHeight:  space.CoverHeight,
			CoverSource:  space.CoverSource,
			Status:       space.Status,
			BannedReason: space.BannedReason,
			BannedAt:     space.BannedAt,
			DeletedAt:    space.DeletedAt,
			CreatedAt:    space.CreatedAt,
			UpdatedAt:    space.UpdatedAt,
		},
		CategoryID:    space.CategoryID,
		CategoryName:  space.Category,
		CategoryIsDef: resolvedCategory.IsDefault,
		OwnerName:     ownerName,
		OwnerEmail:    ownerEmail,
	})

	if err := s.recordSpaceAudit(ctx, RecordAdminAuditInput{
		Module:     AdminAuditModuleSpace,
		Action:     AdminAuditActionCreate,
		TargetType: "space",
		TargetID:   record.SpaceID,
		Summary:    "space created: " + record.SpaceID,
		Detail: map[string]any{
			"name":         record.Name,
			"description":  record.Description,
			"categoryId":   record.CategoryID,
			"category":     record.Category,
			"visibility":   record.Visibility,
			"coverAssetId": strings.TrimSpace(input.CoverAssetID),
		},
	}); err != nil {
		return AdminSpaceRecord{}, err
	}

	return record, nil
}

type adminSpaceSiteConfigPolicy struct {
	DefaultSpaceVisibility string `json:"defaultSpaceVisibility"`
}

func (s *AdminSpaceService) resolveDefaultSpaceVisibility(ctx context.Context) (models.Visibility, error) {
	defaultVisibility := models.VisibilityMember
	if s == nil || s.systemConfigRepo == nil {
		return defaultVisibility, nil
	}

	config, err := s.systemConfigRepo.GetByConfigKey(ctx, "site")
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return defaultVisibility, nil
		}
		return defaultVisibility, err
	}
	if config == nil || strings.TrimSpace(config.ConfigValueJSON) == "" {
		return defaultVisibility, nil
	}

	var policy adminSpaceSiteConfigPolicy
	if err := json.Unmarshal([]byte(config.ConfigValueJSON), &policy); err != nil {
		return defaultVisibility, nil
	}

	visibility := models.Visibility(strings.ToLower(strings.TrimSpace(policy.DefaultSpaceVisibility)))
	if !models.IsValidVisibility(visibility) {
		return defaultVisibility, nil
	}
	return visibility, nil
}

func normalizeAdminSpaceID(rawSpaceID string) (spaceID string, hasCustom bool, err error) {
	normalized := strings.ToLower(strings.TrimSpace(rawSpaceID))
	if normalized == "" {
		return "", false, nil
	}
	if len(normalized) > maxAdminSpaceIDLength {
		return "", true, errcode.ErrAdminSpaceInvalidSpaceID
	}
	if !adminSpaceIDPattern.MatchString(normalized) {
		return "", true, errcode.ErrAdminSpaceInvalidSpaceID
	}
	return normalized, true, nil
}

// CreateCoverAsset 创建空间封面资产（用户上传或系统生成）。
func (s *AdminSpaceService) CreateCoverAsset(
	ctx context.Context,
	input CreateAdminSpaceCoverAssetInput,
) (result AdminSpaceCoverAsset, err error) {
	defer func() {
		err = errcode.MapAdminSpaceError(err)
	}()

	if s == nil || s.spaceRepo == nil || s.adminAccessService == nil {
		return AdminSpaceCoverAsset{}, errors.New("admin space service dependencies are nil")
	}

	actorUserID := strings.TrimSpace(input.ActorUserID)
	if actorUserID == "" {
		return AdminSpaceCoverAsset{}, errcode.ErrAdminForbidden
	}
	source := normalizeAdminSpaceCoverSource(input.Source)
	if source == "" {
		return AdminSpaceCoverAsset{}, errcode.ErrAdminSpaceInvalidCoverSource
	}

	var (
		encodedBytes []byte
		width        int
		height       int
		normalized   bool
	)

	switch source {
	case AdminSpaceCoverSourceUserUpload:
		if len(input.FileBytes) == 0 {
			return AdminSpaceCoverAsset{}, errcode.ErrAdminSpaceCoverFileRequired
		}
		processed, err := processAdminSpaceUserUploadCover(processAdminSpaceUserUploadCoverInput{
			FileName:        input.FileName,
			FileContentType: input.FileContentType,
			FileBytes:       input.FileBytes,
			Quality:         input.PreferredQuality,
		})
		if err != nil {
			return AdminSpaceCoverAsset{}, err
		}
		encodedBytes = processed.WebPBytes
		width = processed.Width
		height = processed.Height
		normalized = processed.Normalized
	case AdminSpaceCoverSourceSystemGenerate:
		spaceName := strings.TrimSpace(input.SpaceName)
		if spaceName == "" {
			return AdminSpaceCoverAsset{}, errcode.ErrAdminSpaceCoverSpaceNameRequired
		}
		processed, err := renderAdminSpaceSystemCover(renderAdminSpaceSystemCoverInput{
			SpaceName: spaceName,
			Quality:   input.PreferredQuality,
		})
		if err != nil {
			return AdminSpaceCoverAsset{}, err
		}
		encodedBytes = processed.WebPBytes
		width = processed.Width
		height = processed.Height
		normalized = true
	default:
		return AdminSpaceCoverAsset{}, errcode.ErrAdminSpaceInvalidCoverSource
	}

	objectKey, err := buildAdminSpaceCoverObjectKey(time.Now().UTC())
	if err != nil {
		return AdminSpaceCoverAsset{}, err
	}
	if err := saveAdminSpaceCoverObject(objectKey, encodedBytes); err != nil {
		return AdminSpaceCoverAsset{}, err
	}
	coverObjectSaved := true
	defer func() {
		if err != nil && coverObjectSaved {
			_ = removeAdminSpaceCoverObject(objectKey)
		}
	}()

	asset := &models.SpaceCoverAsset{
		AssetID:         strings.ToLower(ulid.Make().String()),
		Source:          string(source),
		ObjectKey:       objectKey,
		ObjectURL:       resolveAdminSpaceCoverPublicURL(objectKey),
		MimeType:        "image/webp",
		Width:           width,
		Height:          height,
		SizeBytes:       int64(len(encodedBytes)),
		Normalized:      normalized,
		CreatedByUserID: actorUserID,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}

	if err := s.spaceRepo.CreateCoverAsset(ctx, asset); err != nil {
		return AdminSpaceCoverAsset{}, err
	}
	coverObjectSaved = false

	payload := mapAdminSpaceCoverAsset(*asset)

	if err := s.recordSpaceAudit(ctx, RecordAdminAuditInput{
		Module:     AdminAuditModuleSpace,
		Action:     AdminAuditActionCreate,
		TargetType: "space_cover_asset",
		TargetID:   payload.AssetID,
		Summary:    "space cover asset created: " + payload.AssetID,
		Detail: map[string]any{
			"source":       payload.Source,
			"width":        payload.Width,
			"height":       payload.Height,
			"sizeBytes":    payload.SizeBytes,
			"normalized":   payload.Normalized,
			"clientWidth":  input.ClientWidth,
			"clientHeight": input.ClientHeight,
			"clientMime":   strings.TrimSpace(input.ClientMimeType),
			"clientDone":   input.ClientProcessed,
		},
	}); err != nil {
		return AdminSpaceCoverAsset{}, err
	}

	return payload, nil
}

// UpdateMetadata 更新后台空间元数据（名称、简介、可见性与封面）。
func (s *AdminSpaceService) UpdateMetadata(
	ctx context.Context,
	input UpdateAdminSpaceMetadataInput,
) (result AdminSpaceRecord, err error) {
	defer func() {
		err = errcode.MapAdminSpaceError(err)
	}()

	if s == nil || s.spaceRepo == nil || s.adminAccessService == nil {
		return AdminSpaceRecord{}, errors.New("admin space service dependencies are nil")
	}

	actorUserID := strings.TrimSpace(input.ActorUserID)
	spaceID := strings.TrimSpace(input.SpaceID)
	if spaceID == "" {
		return AdminSpaceRecord{}, errcode.ErrAdminSpaceInvalidSpaceID
	}

	coverOnlyMetadataUpdate := isCoverOnlyAdminSpaceMetadataUpdate(input)
	coverOnlyCanManageSpace := false
	if coverOnlyMetadataUpdate {
		var err error
		coverOnlyCanManageSpace, err = s.ensureCanManageOrOwnSpace(ctx, actorUserID, spaceID)
		if err != nil {
			return AdminSpaceRecord{}, err
		}
	} else {
		if err := s.ensureCanManageSpace(ctx, actorUserID, spaceID); err != nil {
			return AdminSpaceRecord{}, err
		}
	}

	if input.Name == nil && input.Visibility == nil && input.Description == nil && input.CategoryID == nil && input.CoverAssetID == nil {
		return AdminSpaceRecord{}, errcode.ErrAdminSpaceNoMetadataChange
	}

	var normalizedName *string
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return AdminSpaceRecord{}, errcode.ErrAdminSpaceInvalidName
		}
		normalizedName = &name
	}

	var normalizedDescription *string
	if input.Description != nil {
		description := strings.TrimSpace(*input.Description)
		if len([]rune(description)) > maxAdminSpaceDescLength {
			return AdminSpaceRecord{}, errcode.ErrAdminSpaceInvalidDescription
		}
		normalizedDescription = &description
	}

	var (
		normalizedCategoryID *string
		normalizedCategory   *string
	)
	if input.CategoryID != nil {
		resolvedCategory, err := s.resolveSpaceCategoryByID(ctx, *input.CategoryID)
		if err != nil {
			return AdminSpaceRecord{}, err
		}
		categoryID := strings.TrimSpace(resolvedCategory.CategoryID)
		categoryName := strings.TrimSpace(resolvedCategory.Name)
		normalizedCategoryID = &categoryID
		normalizedCategory = &categoryName
	}

	var normalizedVisibility *models.Visibility
	if input.Visibility != nil {
		if !models.IsValidVisibility(*input.Visibility) {
			return AdminSpaceRecord{}, errcode.ErrAdminSpaceInvalidVisibility
		}
		visibility := *input.Visibility
		normalizedVisibility = &visibility
	}

	var (
		normalizedCoverAssetID *string
		normalizedCoverKey     *string
		normalizedCoverURL     *string
		normalizedCoverSource  *string
		normalizedCoverWidth   *int
		normalizedCoverHeight  *int
	)
	if input.CoverAssetID != nil {
		trimmedCoverAssetID := strings.TrimSpace(*input.CoverAssetID)
		if trimmedCoverAssetID == "" {
			// 关键分支：显式清空封面，需把封面字段重置为默认值。
			empty := ""
			zero := 0
			normalizedCoverAssetID = &empty
			normalizedCoverKey = &empty
			normalizedCoverURL = &empty
			normalizedCoverSource = &empty
			normalizedCoverWidth = &zero
			normalizedCoverHeight = &zero
		} else {
			coverAsset, err := s.spaceRepo.GetCoverAssetByAssetID(ctx, trimmedCoverAssetID)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return AdminSpaceRecord{}, errcode.ErrAdminSpaceCoverAssetNotFound
				}
				return AdminSpaceRecord{}, err
			}
			if coverOnlyMetadataUpdate &&
				!coverOnlyCanManageSpace &&
				strings.TrimSpace(coverAsset.CreatedByUserID) != actorUserID {
				return AdminSpaceRecord{}, errcode.ErrAdminForbidden
			}

			assetID := strings.TrimSpace(coverAsset.AssetID)
			coverKey := strings.TrimSpace(coverAsset.ObjectKey)
			coverURL := strings.TrimSpace(coverAsset.ObjectURL)
			coverSource := strings.TrimSpace(coverAsset.Source)
			width := coverAsset.Width
			height := coverAsset.Height
			normalizedCoverAssetID = &assetID
			normalizedCoverKey = &coverKey
			normalizedCoverURL = &coverURL
			normalizedCoverSource = &coverSource
			normalizedCoverWidth = &width
			normalizedCoverHeight = &height
		}
	}

	snapshot, err := s.spaceRepo.GetBySpaceID(ctx, spaceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminSpaceRecord{}, errcode.ErrAdminSpaceNotFound
		}
		return AdminSpaceRecord{}, err
	}
	if normalizeEntityStatus(snapshot.Status) == models.EntityStatusDeleted || snapshot.DeletedAt != nil {
		return AdminSpaceRecord{}, errcode.ErrAdminSpaceAlreadyDeleted
	}

	updated, err := s.spaceRepo.UpdateMetadata(ctx, repository.UpdateSpaceMetadataParams{
		SpaceID:      spaceID,
		Name:         normalizedName,
		Description:  normalizedDescription,
		CategoryID:   normalizedCategoryID,
		Category:     normalizedCategory,
		Visibility:   normalizedVisibility,
		CoverAssetID: normalizedCoverAssetID,
		CoverKey:     normalizedCoverKey,
		CoverURL:     normalizedCoverURL,
		CoverWidth:   normalizedCoverWidth,
		CoverHeight:  normalizedCoverHeight,
		CoverSource:  normalizedCoverSource,
		UpdatedAt:    time.Now().UTC(),
	})
	if err != nil {
		return AdminSpaceRecord{}, err
	}
	if !updated {
		return AdminSpaceRecord{}, errcode.ErrAdminSpaceNotFound
	}

	latest, err := s.spaceRepo.GetBySpaceID(ctx, spaceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminSpaceRecord{}, errcode.ErrAdminSpaceNotFound
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

	record := mapAdminSpaceRecord(repository.AdminSpaceListRecord{
		Space:      *latest,
		OwnerName:  ownerName,
		OwnerEmail: ownerEmail,
	})

	detail := map[string]any{
		"nameBefore":       snapshot.Name,
		"nameAfter":        record.Name,
		"visibilityBefore": snapshot.Visibility,
		"visibilityAfter":  record.Visibility,
	}
	if normalizedDescription != nil {
		detail["descriptionBefore"] = snapshot.Description
		detail["descriptionAfter"] = record.Description
	}
	if normalizedCategory != nil {
		detail["categoryIdBefore"] = snapshot.CategoryID
		detail["categoryIdAfter"] = record.CategoryID
		detail["categoryBefore"] = snapshot.Category
		detail["categoryAfter"] = record.Category
	}
	if input.CoverAssetID != nil {
		beforeCoverURL := strings.TrimSpace(snapshot.CoverURL)
		afterCoverURL := ""
		afterCoverSource := ""
		if record.Cover != nil {
			afterCoverURL = record.Cover.URL
			afterCoverSource = string(record.Cover.Source)
		}
		detail["coverURLBefore"] = beforeCoverURL
		detail["coverURLAfter"] = afterCoverURL
		detail["coverSourceAfter"] = afterCoverSource
	}
	auditTargetType := "space"
	auditSummary := "space metadata updated: " + record.SpaceID
	if coverOnlyMetadataUpdate {
		auditTargetType = "space_cover_binding"
		auditSummary = "space cover binding updated: " + record.SpaceID
	}
	if err := s.recordSpaceAudit(ctx, RecordAdminAuditInput{
		Module:     AdminAuditModuleSpace,
		Action:     AdminAuditActionUpdate,
		TargetType: auditTargetType,
		TargetID:   record.SpaceID,
		Summary:    auditSummary,
		Detail:     detail,
	}); err != nil {
		return AdminSpaceRecord{}, err
	}

	return record, nil
}

// TransferOwnership 转让空间归属（当前 owner -> 目标成员）。
func (s *AdminSpaceService) TransferOwnership(
	ctx context.Context,
	input TransferAdminSpaceOwnershipInput,
) (result AdminSpaceRecord, err error) {
	defer func() {
		err = errcode.MapAdminSpaceError(err)
	}()

	if s == nil || s.spaceRepo == nil || s.userRepo == nil || s.adminAccessService == nil || s.adminRoleRepo == nil || s.spaceScopeRepo == nil {
		return AdminSpaceRecord{}, errors.New("admin space service dependencies are nil")
	}

	actorUserID := strings.TrimSpace(input.ActorUserID)
	spaceID := strings.TrimSpace(input.SpaceID)
	if spaceID == "" {
		return AdminSpaceRecord{}, errcode.ErrAdminSpaceInvalidSpaceID
	}
	if err := s.ensureCanManageSpace(ctx, actorUserID, spaceID); err != nil {
		return AdminSpaceRecord{}, err
	}

	targetUserID := strings.TrimSpace(input.TargetUserID)
	targetEmail := strings.TrimSpace(input.TargetEmail)
	if targetUserID == "" && targetEmail == "" {
		return AdminSpaceRecord{}, errcode.ErrAdminSpaceTransferTargetRequired
	}

	var targetUser *models.User
	if targetUserID != "" {
		targetUser, err = s.userRepo.GetByUserID(ctx, targetUserID)
	} else {
		targetUser, err = s.userRepo.GetByEmail(ctx, targetEmail)
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminSpaceRecord{}, errcode.ErrAdminSpaceTransferTargetNotFound
		}
		return AdminSpaceRecord{}, err
	}
	if targetUser == nil {
		return AdminSpaceRecord{}, errcode.ErrAdminSpaceTransferTargetNotFound
	}
	targetUserID = strings.TrimSpace(targetUser.UserID)
	if targetUserID == "" {
		return AdminSpaceRecord{}, errcode.ErrAdminSpaceTransferTargetNotFound
	}

	snapshot, err := s.spaceRepo.GetBySpaceID(ctx, spaceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminSpaceRecord{}, errcode.ErrAdminSpaceNotFound
		}
		return AdminSpaceRecord{}, err
	}
	if normalizeEntityStatus(snapshot.Status) == models.EntityStatusDeleted || snapshot.DeletedAt != nil {
		return AdminSpaceRecord{}, errcode.ErrAdminSpaceAlreadyDeleted
	}

	if strings.TrimSpace(snapshot.OwnerUserID) == targetUserID {
		return AdminSpaceRecord{}, errcode.ErrAdminSpaceTransferToSelf
	}

	isMember, err := s.spaceRepo.IsMember(ctx, spaceID, targetUserID)
	if err != nil {
		return AdminSpaceRecord{}, err
	}
	if !isMember {
		return AdminSpaceRecord{}, errcode.ErrAdminSpaceTransferTargetNotMember
	}

	updated, err := s.spaceRepo.TransferOwnership(ctx, spaceID, snapshot.OwnerUserID, targetUserID, time.Now().UTC())
	if err != nil {
		return AdminSpaceRecord{}, err
	}
	if !updated {
		return AdminSpaceRecord{}, errcode.ErrAdminSpaceNotFound
	}

	// 目标用户升级为空间管理员，并绑定空间管理范围。
	if err := s.spaceScopeRepo.UpsertScope(ctx, targetUserID, spaceID); err != nil {
		return AdminSpaceRecord{}, err
	}
	if err := s.ensureSpaceAdminRole(ctx, targetUserID); err != nil {
		return AdminSpaceRecord{}, err
	}

	// 当前 owner 降级为协作者，移除当前空间的管理范围。
	if err := s.spaceScopeRepo.DeleteScope(ctx, snapshot.OwnerUserID, spaceID); err != nil {
		return AdminSpaceRecord{}, err
	}
	if err := s.dropSpaceAdminRoleWhenNoScopes(ctx, snapshot.OwnerUserID); err != nil {
		return AdminSpaceRecord{}, err
	}

	latest, err := s.spaceRepo.GetBySpaceID(ctx, spaceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminSpaceRecord{}, errcode.ErrAdminSpaceNotFound
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

	record := mapAdminSpaceRecord(repository.AdminSpaceListRecord{
		Space:      *latest,
		OwnerName:  ownerName,
		OwnerEmail: ownerEmail,
	})

	detail := map[string]any{
		"ownerBefore": snapshot.OwnerUserID,
		"ownerAfter":  targetUserID,
		"targetEmail": targetUser.Email,
	}
	if err := s.recordSpaceAudit(ctx, RecordAdminAuditInput{
		Module:     AdminAuditModuleSpace,
		Action:     AdminAuditActionUpdate,
		TargetType: "space",
		TargetID:   record.SpaceID,
		Summary:    "space ownership transferred: " + record.SpaceID,
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
) (result AdminSpaceRecord, err error) {
	defer func() {
		err = errcode.MapAdminSpaceError(err)
	}()

	if s == nil || s.spaceRepo == nil || s.adminAccessService == nil {
		return AdminSpaceRecord{}, errors.New("admin space service dependencies are nil")
	}

	actorUserID := strings.TrimSpace(input.ActorUserID)
	spaceID := strings.TrimSpace(input.SpaceID)
	if spaceID == "" {
		return AdminSpaceRecord{}, errcode.ErrAdminSpaceInvalidSpaceID
	}
	if input.Status != models.EntityStatusActive && input.Status != models.EntityStatusBanned {
		return AdminSpaceRecord{}, errcode.ErrAdminSpaceInvalidStatus
	}
	if input.Status == models.EntityStatusBanned && strings.TrimSpace(input.Reason) == "" {
		return AdminSpaceRecord{}, errcode.ErrAdminSpaceBanReasonRequired
	}

	if err := s.ensureCanManageSpace(ctx, actorUserID, spaceID); err != nil {
		return AdminSpaceRecord{}, err
	}

	snapshot, err := s.spaceRepo.GetBySpaceID(ctx, spaceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminSpaceRecord{}, errcode.ErrAdminSpaceNotFound
		}
		return AdminSpaceRecord{}, err
	}
	if normalizeEntityStatus(snapshot.Status) == models.EntityStatusDeleted || snapshot.DeletedAt != nil {
		return AdminSpaceRecord{}, errcode.ErrAdminSpaceAlreadyDeleted
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
		return AdminSpaceRecord{}, errcode.ErrAdminSpaceNotFound
	}

	latest, err := s.spaceRepo.GetBySpaceID(ctx, spaceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminSpaceRecord{}, errcode.ErrAdminSpaceNotFound
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

	record := mapAdminSpaceRecord(repository.AdminSpaceListRecord{
		Space:      *latest,
		OwnerName:  ownerName,
		OwnerEmail: ownerEmail,
	})

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

// DeleteSpace 事务硬删除后台目标空间及其关联资源记录。
func (s *AdminSpaceService) DeleteSpace(
	ctx context.Context,
	actorUserID string,
	spaceID string,
	requestID string,
) (err error) {
	defer func() {
		err = errcode.MapAdminSpaceError(err)
	}()

	_ = requestID

	if s == nil || s.spaceRepo == nil || s.adminAccessService == nil {
		return errors.New("admin space service dependencies are nil")
	}

	actor := strings.TrimSpace(actorUserID)
	targetSpaceID := strings.TrimSpace(spaceID)
	if targetSpaceID == "" {
		return errcode.ErrAdminSpaceInvalidSpaceID
	}

	if err := s.ensureCanManageSpace(ctx, actor, targetSpaceID); err != nil {
		return err
	}

	snapshot, err := s.spaceRepo.GetBySpaceID(ctx, targetSpaceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.ErrAdminSpaceNotFound
		}
		return err
	}

	deleted, err := s.spaceRepo.HardDelete(ctx, targetSpaceID)
	if err != nil {
		return err
	}
	if !deleted {
		return errcode.ErrAdminSpaceNotFound
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

func (s *AdminSpaceService) ensureCanManageOrOwnSpace(ctx context.Context, actorUserID string, spaceID string) (bool, error) {
	if strings.TrimSpace(actorUserID) == "" || strings.TrimSpace(spaceID) == "" || s == nil || s.spaceRepo == nil {
		return false, errcode.ErrAdminForbidden
	}
	canManage, err := s.adminAccessService.CanManageSpace(ctx, actorUserID, spaceID)
	if err != nil {
		return false, err
	}
	if canManage {
		return true, nil
	}
	space, err := s.spaceRepo.GetBySpaceID(ctx, strings.TrimSpace(spaceID))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, errcode.ErrAdminForbidden
		}
		return false, err
	}
	if space == nil || strings.TrimSpace(space.OwnerUserID) != strings.TrimSpace(actorUserID) {
		return false, errcode.ErrAdminForbidden
	}
	return false, nil
}

func isCoverOnlyAdminSpaceMetadataUpdate(input UpdateAdminSpaceMetadataInput) bool {
	return input.CoverAssetID != nil &&
		input.Name == nil &&
		input.Visibility == nil &&
		input.Description == nil &&
		input.CategoryID == nil
}

func (s *AdminSpaceService) ensureSpaceAdminRole(ctx context.Context, userID string) error {
	// 关键函数：保证目标用户具备 space_admin 管理角色。
	if s == nil || s.adminRoleRepo == nil {
		return errors.New("admin space service adminRoleRepo is nil")
	}
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return nil
	}

	roles, err := s.adminRoleRepo.ListByUserID(ctx, normalizedUserID)
	if err != nil {
		return err
	}
	for _, role := range roles {
		if role == models.AdminRoleSpaceAdmin {
			return nil
		}
	}

	roles = append(roles, models.AdminRoleSpaceAdmin)
	return s.adminRoleRepo.ReplaceByUserID(ctx, normalizedUserID, roles)
}

func (s *AdminSpaceService) dropSpaceAdminRoleWhenNoScopes(ctx context.Context, userID string) error {
	// 关键函数：无任何管理范围时移除 space_admin 角色，避免保留后台入口。
	if s == nil || s.adminRoleRepo == nil || s.spaceScopeRepo == nil {
		return errors.New("admin space service dependencies are nil")
	}
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return nil
	}

	roles, err := s.adminRoleRepo.ListByUserID(ctx, normalizedUserID)
	if err != nil {
		return err
	}
	for _, role := range roles {
		if role == models.AdminRolePlatformAdmin {
			return nil
		}
	}

	scopes, err := s.spaceScopeRepo.ListByUserID(ctx, normalizedUserID)
	if err != nil {
		return err
	}
	if len(scopes) > 0 {
		return nil
	}

	filtered := make([]models.AdminRole, 0, len(roles))
	removed := false
	for _, role := range roles {
		if role == models.AdminRoleSpaceAdmin {
			removed = true
			continue
		}
		filtered = append(filtered, role)
	}
	if !removed {
		return nil
	}
	return s.adminRoleRepo.ReplaceByUserID(ctx, normalizedUserID, filtered)
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
		return false, errcode.ErrAdminForbidden
	}
	return true, nil
}

func (s *AdminSpaceService) recordSpaceAudit(ctx context.Context, input RecordAdminAuditInput) error {
	if s == nil || s.adminAuditService == nil {
		return nil
	}
	return s.adminAuditService.Record(ctx, input)
}

func (s *AdminSpaceService) resolveTargetUser(ctx context.Context, targetUserID string, targetEmail string) (*models.User, error) {
	if s == nil || s.userRepo == nil {
		return nil, errors.New("admin space service userRepo is nil")
	}

	normalizedUserID := strings.TrimSpace(targetUserID)
	normalizedEmail := strings.TrimSpace(targetEmail)
	if normalizedUserID == "" && normalizedEmail == "" {
		return nil, errcode.ErrAdminSpaceMemberTargetRequired
	}

	var (
		targetUser *models.User
		err        error
	)
	if normalizedUserID != "" {
		targetUser, err = s.userRepo.GetByUserID(ctx, normalizedUserID)
	} else {
		targetUser, err = s.userRepo.GetByEmail(ctx, normalizedEmail)
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.ErrAdminSpaceMemberTargetNotFound
		}
		return nil, err
	}
	if targetUser == nil || strings.TrimSpace(targetUser.UserID) == "" {
		return nil, errcode.ErrAdminSpaceMemberTargetNotFound
	}
	return targetUser, nil
}

func (s *AdminSpaceService) hydrateSpaceMembers(
	ctx context.Context,
	spaceSnapshot *models.Space,
	members []repository.SpaceMemberListRecord,
) []AdminSpaceMemberRecord {
	ownerUserID := ""
	ownerCreatedAt := time.Time{}
	ownerUpdatedAt := time.Time{}
	if spaceSnapshot != nil {
		ownerUserID = strings.TrimSpace(spaceSnapshot.OwnerUserID)
		ownerCreatedAt = spaceSnapshot.CreatedAt
		ownerUpdatedAt = spaceSnapshot.UpdatedAt
	}

	result := make([]AdminSpaceMemberRecord, 0, len(members)+1)
	seen := make(map[string]int, len(members)+1)
	for _, member := range members {
		memberUserID := strings.TrimSpace(member.UserID)
		if memberUserID == "" {
			continue
		}
		record := AdminSpaceMemberRecord{
			UserID:    memberUserID,
			Email:     strings.TrimSpace(member.Email),
			Name:      strings.TrimSpace(member.Name),
			Role:      normalizeAdminSpaceMemberRole(member.Role),
			IsOwner:   memberUserID == ownerUserID,
			CreatedAt: member.CreatedAt,
			UpdatedAt: member.UpdatedAt,
		}
		if record.IsOwner {
			record.Role = models.RoleOwner
		}
		seen[memberUserID] = len(result)
		result = append(result, record)
	}

	if ownerUserID != "" {
		ownerEmail := ""
		ownerName := ""
		if s != nil && s.userRepo != nil {
			ownerUser, err := s.userRepo.GetByUserID(ctx, ownerUserID)
			if err == nil && ownerUser != nil {
				ownerEmail = strings.TrimSpace(ownerUser.Email)
				ownerName = strings.TrimSpace(ownerUser.Name)
			}
		}
		if index, exists := seen[ownerUserID]; exists {
			result[index].IsOwner = true
			result[index].Role = models.RoleOwner
			if ownerEmail != "" {
				result[index].Email = ownerEmail
			}
			if ownerName != "" {
				result[index].Name = ownerName
			}
		} else {
			result = append([]AdminSpaceMemberRecord{
				{
					UserID:    ownerUserID,
					Email:     ownerEmail,
					Name:      ownerName,
					Role:      models.RoleOwner,
					IsOwner:   true,
					CreatedAt: ownerCreatedAt,
					UpdatedAt: ownerUpdatedAt,
				},
			}, result...)
		}
	}

	return result
}

func findAdminSpaceMemberRecord(members []AdminSpaceMemberRecord, userID string) (AdminSpaceMemberRecord, bool) {
	targetUserID := strings.TrimSpace(userID)
	if targetUserID == "" {
		return AdminSpaceMemberRecord{}, false
	}
	for _, member := range members {
		if strings.TrimSpace(member.UserID) != targetUserID {
			continue
		}
		return member, true
	}
	return AdminSpaceMemberRecord{}, false
}

func normalizeAdminSpaceMemberRole(value models.Role) models.Role {
	switch models.Role(strings.ToLower(strings.TrimSpace(string(value)))) {
	case models.RoleOwner:
		return models.RoleOwner
	case models.RoleCollaborator:
		return models.RoleCollaborator
	case models.RoleReader:
		return models.RoleReader
	default:
		return models.RoleReader
	}
}

func normalizeEditableAdminSpaceMemberRole(value models.Role) models.Role {
	switch models.Role(strings.ToLower(strings.TrimSpace(string(value)))) {
	case models.RoleCollaborator:
		return models.RoleCollaborator
	case models.RoleReader:
		return models.RoleReader
	default:
		return ""
	}
}

func normalizeAdminSpaceCategoryName(rawName string) (string, error) {
	name := strings.TrimSpace(rawName)
	if name == "" {
		return "", errcode.ErrAdminSpaceInvalidCategory
	}
	if len([]rune(name)) > maxAdminSpaceCategoryLen {
		return "", errcode.ErrAdminSpaceInvalidCategory
	}
	return name, nil
}

func (s *AdminSpaceService) getDefaultSpaceCategory(ctx context.Context) (models.SpaceCategory, error) {
	if s == nil || s.spaceCategoryRepo == nil {
		return models.SpaceCategory{}, errors.New("admin space service dependencies are nil")
	}

	defaultCategory, err := s.spaceCategoryRepo.GetDefault(ctx)
	if err == nil && defaultCategory != nil {
		defaultCategory.CategoryID = strings.TrimSpace(defaultCategory.CategoryID)
		defaultCategory.Name = strings.TrimSpace(defaultCategory.Name)
		if defaultCategory.CategoryID != "" && defaultCategory.Name != "" {
			return *defaultCategory, nil
		}
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.SpaceCategory{}, err
	}

	now := time.Now().UTC()
	seed := &models.SpaceCategory{
		CategoryID: models.DefaultSpaceCategoryID,
		Name:       models.DefaultSpaceCategoryName,
		IsDefault:  true,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if createErr := s.spaceCategoryRepo.Create(ctx, seed); createErr != nil {
		if !errors.Is(createErr, gorm.ErrDuplicatedKey) && !strings.Contains(strings.ToLower(createErr.Error()), "unique") {
			return models.SpaceCategory{}, createErr
		}
	}
	defaultCategory, err = s.spaceCategoryRepo.GetDefault(ctx)
	if err != nil {
		return models.SpaceCategory{}, err
	}
	if defaultCategory == nil {
		return models.SpaceCategory{}, errcode.ErrAdminSpaceCategoryNotFound
	}
	defaultCategory.CategoryID = strings.TrimSpace(defaultCategory.CategoryID)
	defaultCategory.Name = strings.TrimSpace(defaultCategory.Name)
	if defaultCategory.CategoryID == "" || defaultCategory.Name == "" {
		return models.SpaceCategory{}, errcode.ErrAdminSpaceCategoryNotFound
	}
	return *defaultCategory, nil
}

func (s *AdminSpaceService) resolveSpaceCategoryByID(
	ctx context.Context,
	rawCategoryID string,
) (models.SpaceCategory, error) {
	if s == nil || s.spaceCategoryRepo == nil {
		return models.SpaceCategory{}, errors.New("admin space service dependencies are nil")
	}
	categoryID := strings.TrimSpace(rawCategoryID)
	if categoryID == "" {
		return s.getDefaultSpaceCategory(ctx)
	}

	category, err := s.spaceCategoryRepo.GetByCategoryID(ctx, categoryID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.SpaceCategory{}, errcode.ErrAdminSpaceInvalidCategory
		}
		return models.SpaceCategory{}, err
	}
	if category == nil {
		return models.SpaceCategory{}, errcode.ErrAdminSpaceInvalidCategory
	}

	category.CategoryID = strings.TrimSpace(category.CategoryID)
	category.Name = strings.TrimSpace(category.Name)
	if category.CategoryID == "" || category.Name == "" {
		return models.SpaceCategory{}, errcode.ErrAdminSpaceInvalidCategory
	}
	return *category, nil
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
		return nil, errcode.ErrAdminSpaceInvalidStatusFilter
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
		return nil, errcode.ErrAdminSpaceInvalidVisibilityFilter
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
	description := strings.TrimSpace(space.Description)
	categoryID := strings.TrimSpace(record.CategoryID)
	if categoryID == "" {
		categoryID = strings.TrimSpace(space.CategoryID)
	}
	category := strings.TrimSpace(record.CategoryName)
	if category == "" {
		category = strings.TrimSpace(space.Category)
	}
	categoryIsDefault := record.CategoryIsDef
	if !categoryIsDefault && categoryID == models.DefaultSpaceCategoryID {
		categoryIsDefault = true
	}
	coverKey := strings.TrimSpace(space.CoverKey)
	coverURL := strings.TrimSpace(space.CoverURL)
	coverSource := normalizeAdminSpaceCoverSource(AdminSpaceCoverSource(strings.TrimSpace(space.CoverSource)))
	coverWidth := space.CoverWidth
	if coverWidth < 0 {
		coverWidth = 0
	}
	coverHeight := space.CoverHeight
	if coverHeight < 0 {
		coverHeight = 0
	}

	var cover *AdminSpaceCoverAsset
	if coverKey != "" && coverURL != "" && coverWidth > 0 && coverHeight > 0 && coverSource != "" {
		assetID := ""
		if space.CoverAssetID != nil {
			assetID = strings.TrimSpace(*space.CoverAssetID)
		}
		cover = &AdminSpaceCoverAsset{
			AssetID:    assetID,
			Key:        coverKey,
			URL:        coverURL,
			Width:      coverWidth,
			Height:     coverHeight,
			MimeType:   "image/webp",
			SizeBytes:  0,
			Normalized: true,
			Source:     coverSource,
		}
	}

	return AdminSpaceRecord{
		SpaceID:           space.SpaceID,
		Name:              strings.TrimSpace(space.Name),
		Description:       description,
		CategoryID:        categoryID,
		Category:          category,
		CategoryIsDefault: categoryIsDefault,
		OwnerUserID:       space.OwnerUserID,
		OwnerName:         record.OwnerName,
		OwnerEmail:        record.OwnerEmail,
		Visibility:        visibility,
		Cover:             cover,
		Status:            status,
		BannedReason:      space.BannedReason,
		BannedAt:          space.BannedAt,
		DeletedAt:         space.DeletedAt,
		CreatedAt:         space.CreatedAt,
		UpdatedAt:         space.UpdatedAt,
	}
}

func mapAdminSpaceCoverAsset(value models.SpaceCoverAsset) AdminSpaceCoverAsset {
	width := value.Width
	if width < 0 {
		width = 0
	}
	height := value.Height
	if height < 0 {
		height = 0
	}
	sizeBytes := value.SizeBytes
	if sizeBytes < 0 {
		sizeBytes = 0
	}
	return AdminSpaceCoverAsset{
		AssetID:     strings.TrimSpace(value.AssetID),
		Key:         strings.TrimSpace(value.ObjectKey),
		URL:         strings.TrimSpace(value.ObjectURL),
		Width:       width,
		Height:      height,
		MimeType:    strings.TrimSpace(value.MimeType),
		SizeBytes:   sizeBytes,
		Normalized:  value.Normalized,
		Source:      normalizeAdminSpaceCoverSource(AdminSpaceCoverSource(value.Source)),
		CreatedAt:   value.CreatedAt,
		UpdatedAt:   value.UpdatedAt,
		CreatedByID: strings.TrimSpace(value.CreatedByUserID),
	}
}

func normalizeAdminSpaceCoverSource(source AdminSpaceCoverSource) AdminSpaceCoverSource {
	switch AdminSpaceCoverSource(strings.ToLower(strings.TrimSpace(string(source)))) {
	case AdminSpaceCoverSourceUserUpload:
		return AdminSpaceCoverSourceUserUpload
	case AdminSpaceCoverSourceSystemGenerate:
		return AdminSpaceCoverSourceSystemGenerate
	default:
		return ""
	}
}
