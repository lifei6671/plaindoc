package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"
)

const (
	defaultAdminSpacePage     = 1
	defaultAdminSpacePageSize = 20
	maxAdminSpacePageSize     = 100
	maxAdminSpaceNameLength   = 120
	maxAdminSpaceDescLength   = 280
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
	ErrAdminSpaceInvalidDescription      = errors.New("admin space description is invalid")
	ErrAdminSpaceInvalidCoverSource      = errors.New("admin space cover source is invalid")
	ErrAdminSpaceCoverFileRequired       = errors.New("admin space cover file is required")
	ErrAdminSpaceCoverSpaceNameRequired  = errors.New("admin space cover space name is required")
	ErrAdminSpaceCoverAssetNotFound      = errors.New("admin space cover asset not found")
	ErrAdminSpaceCoverImageInvalid       = errors.New("admin space cover image is invalid")
	ErrAdminSpaceCoverImageTooLarge      = errors.New("admin space cover image is too large")
	ErrAdminSpaceCoverImageTooManyPixels = errors.New("admin space cover image has too many pixels")
	ErrAdminSpaceFontUnavailable         = errors.New("admin space cover font is unavailable")
	ErrAdminSpaceTransferTargetRequired  = errors.New("admin space transfer target is required")
	ErrAdminSpaceTransferTargetNotFound  = errors.New("admin space transfer target not found")
	ErrAdminSpaceTransferTargetNotMember = errors.New("admin space transfer target not member")
	ErrAdminSpaceTransferToSelf          = errors.New("admin space transfer to self")
)

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
	Name         string
	Description  string
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
	SpaceID      string
	Name         string
	Description  string
	OwnerUserID  string
	OwnerName    string
	OwnerEmail   string
	Visibility   models.Visibility
	Cover        *AdminSpaceCoverAsset
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
	Description *string
	Visibility  *models.Visibility
	CoverAssetID *string
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
	userRepo           repository.UserRepository
	adminRoleRepo      repository.AdminRoleRepository
	spaceScopeRepo     repository.SpaceAdminScopeRepository
	adminAccessService *AdminAccessService
	adminAuditService  *AdminAuditService
}

// NewAdminSpaceService 创建后台空间管理服务。
func NewAdminSpaceService(
	spaceRepo repository.SpaceRepository,
	userRepo repository.UserRepository,
	adminRoleRepo repository.AdminRoleRepository,
	spaceScopeRepo repository.SpaceAdminScopeRepository,
	adminAccessService *AdminAccessService,
	adminAuditService *AdminAuditService,
) *AdminSpaceService {
	return &AdminSpaceService{
		spaceRepo:          spaceRepo,
		userRepo:           userRepo,
		adminRoleRepo:      adminRoleRepo,
		spaceScopeRepo:     spaceScopeRepo,
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

// CreateSpace 后台创建空间。
func (s *AdminSpaceService) CreateSpace(
	ctx context.Context,
	input CreateAdminSpaceInput,
) (AdminSpaceRecord, error) {
	if s == nil || s.spaceRepo == nil || s.adminAccessService == nil {
		return AdminSpaceRecord{}, errors.New("admin space service dependencies are nil")
	}

	actorUserID := strings.TrimSpace(input.ActorUserID)
	if actorUserID == "" {
		return AdminSpaceRecord{}, ErrAdminForbidden
	}
	isAdmin, err := s.adminAccessService.IsAdmin(ctx, actorUserID)
	if err != nil {
		return AdminSpaceRecord{}, err
	}
	if !isAdmin {
		return AdminSpaceRecord{}, ErrAdminForbidden
	}

	name := strings.TrimSpace(input.Name)
	if name == "" || len([]rune(name)) > maxAdminSpaceNameLength {
		return AdminSpaceRecord{}, ErrAdminSpaceInvalidName
	}

	description := strings.TrimSpace(input.Description)
	if len([]rune(description)) > maxAdminSpaceDescLength {
		return AdminSpaceRecord{}, ErrAdminSpaceInvalidDescription
	}

	visibility := input.Visibility
	if !models.IsValidVisibility(visibility) {
		if strings.TrimSpace(string(visibility)) != "" {
			return AdminSpaceRecord{}, ErrAdminSpaceInvalidVisibility
		}
		visibility = models.VisibilityMember
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
				return AdminSpaceRecord{}, ErrAdminSpaceCoverAssetNotFound
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
		SpaceID:      strings.ToLower(ulid.Make().String()),
		Name:         name,
		Description:  description,
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
		OwnerName:  ownerName,
		OwnerEmail: ownerEmail,
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
			"visibility":   record.Visibility,
			"coverAssetId": strings.TrimSpace(input.CoverAssetID),
		},
	}); err != nil {
		return AdminSpaceRecord{}, err
	}

	return record, nil
}

// CreateCoverAsset 创建空间封面资产（用户上传或系统生成）。
func (s *AdminSpaceService) CreateCoverAsset(
	ctx context.Context,
	input CreateAdminSpaceCoverAssetInput,
) (AdminSpaceCoverAsset, error) {
	if s == nil || s.spaceRepo == nil || s.adminAccessService == nil {
		return AdminSpaceCoverAsset{}, errors.New("admin space service dependencies are nil")
	}

	actorUserID := strings.TrimSpace(input.ActorUserID)
	if actorUserID == "" {
		return AdminSpaceCoverAsset{}, ErrAdminForbidden
	}
	isAdmin, err := s.adminAccessService.IsAdmin(ctx, actorUserID)
	if err != nil {
		return AdminSpaceCoverAsset{}, err
	}
	if !isAdmin {
		return AdminSpaceCoverAsset{}, ErrAdminForbidden
	}

	source := normalizeAdminSpaceCoverSource(input.Source)
	if source == "" {
		return AdminSpaceCoverAsset{}, ErrAdminSpaceInvalidCoverSource
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
			return AdminSpaceCoverAsset{}, ErrAdminSpaceCoverFileRequired
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
			return AdminSpaceCoverAsset{}, ErrAdminSpaceCoverSpaceNameRequired
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
		return AdminSpaceCoverAsset{}, ErrAdminSpaceInvalidCoverSource
	}

	objectKey, err := buildAdminSpaceCoverObjectKey(time.Now().UTC())
	if err != nil {
		return AdminSpaceCoverAsset{}, err
	}
	if err := saveAdminSpaceCoverObject(objectKey, encodedBytes); err != nil {
		return AdminSpaceCoverAsset{}, err
	}

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

	if input.Name == nil && input.Visibility == nil && input.Description == nil && input.CoverAssetID == nil {
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

	var normalizedDescription *string
	if input.Description != nil {
		description := strings.TrimSpace(*input.Description)
		if len([]rune(description)) > maxAdminSpaceDescLength {
			return AdminSpaceRecord{}, ErrAdminSpaceInvalidDescription
		}
		normalizedDescription = &description
	}

	var normalizedVisibility *models.Visibility
	if input.Visibility != nil {
		if !models.IsValidVisibility(*input.Visibility) {
			return AdminSpaceRecord{}, ErrAdminSpaceInvalidVisibility
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
					return AdminSpaceRecord{}, ErrAdminSpaceCoverAssetNotFound
				}
				return AdminSpaceRecord{}, err
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
			return AdminSpaceRecord{}, ErrAdminSpaceNotFound
		}
		return AdminSpaceRecord{}, err
	}
	if normalizeEntityStatus(snapshot.Status) == models.EntityStatusDeleted || snapshot.DeletedAt != nil {
		return AdminSpaceRecord{}, ErrAdminSpaceAlreadyDeleted
	}

	updated, err := s.spaceRepo.UpdateMetadata(ctx, repository.UpdateSpaceMetadataParams{
		SpaceID:      spaceID,
		Name:         normalizedName,
		Description:  normalizedDescription,
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

// TransferOwnership 转让空间归属（当前 owner -> 目标成员）。
func (s *AdminSpaceService) TransferOwnership(
	ctx context.Context,
	input TransferAdminSpaceOwnershipInput,
) (AdminSpaceRecord, error) {
	if s == nil || s.spaceRepo == nil || s.userRepo == nil || s.adminAccessService == nil || s.adminRoleRepo == nil || s.spaceScopeRepo == nil {
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

	targetUserID := strings.TrimSpace(input.TargetUserID)
	targetEmail := strings.TrimSpace(input.TargetEmail)
	if targetUserID == "" && targetEmail == "" {
		return AdminSpaceRecord{}, ErrAdminSpaceTransferTargetRequired
	}

	var targetUser *models.User
	var err error
	if targetUserID != "" {
		targetUser, err = s.userRepo.GetByUserID(ctx, targetUserID)
	} else {
		targetUser, err = s.userRepo.GetByEmail(ctx, targetEmail)
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminSpaceRecord{}, ErrAdminSpaceTransferTargetNotFound
		}
		return AdminSpaceRecord{}, err
	}
	if targetUser == nil {
		return AdminSpaceRecord{}, ErrAdminSpaceTransferTargetNotFound
	}
	targetUserID = strings.TrimSpace(targetUser.UserID)
	if targetUserID == "" {
		return AdminSpaceRecord{}, ErrAdminSpaceTransferTargetNotFound
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

	if strings.TrimSpace(snapshot.OwnerUserID) == targetUserID {
		return AdminSpaceRecord{}, ErrAdminSpaceTransferToSelf
	}

	isMember, err := s.spaceRepo.IsMember(ctx, spaceID, targetUserID)
	if err != nil {
		return AdminSpaceRecord{}, err
	}
	if !isMember {
		return AdminSpaceRecord{}, ErrAdminSpaceTransferTargetNotMember
	}

	updated, err := s.spaceRepo.TransferOwnership(ctx, spaceID, snapshot.OwnerUserID, targetUserID, time.Now().UTC())
	if err != nil {
		return AdminSpaceRecord{}, err
	}
	if !updated {
		return AdminSpaceRecord{}, ErrAdminSpaceNotFound
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
	description := strings.TrimSpace(space.Description)
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
		SpaceID:      space.SpaceID,
		Name:         strings.TrimSpace(space.Name),
		Description:  description,
		OwnerUserID:  space.OwnerUserID,
		OwnerName:    record.OwnerName,
		OwnerEmail:   record.OwnerEmail,
		Visibility:   visibility,
		Cover:        cover,
		Status:       status,
		BannedReason: space.BannedReason,
		BannedAt:     space.BannedAt,
		DeletedAt:    space.DeletedAt,
		CreatedAt:    space.CreatedAt,
		UpdatedAt:    space.UpdatedAt,
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
