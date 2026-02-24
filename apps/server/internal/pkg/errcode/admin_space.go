package errcode

import (
	"errors"
	"net/http"
)

// AdminSpaceErrorTargets 描述管理员空间管理场景的错误目标。
type AdminSpaceErrorTargets struct {
	Forbidden                error
	InvalidStatusFilter      error
	InvalidVisibilityFilter  error
	InvalidSpaceID           error
	AlreadyExists            error
	InvalidName              error
	InvalidVisibility        error
	InvalidStatus            error
	BanReasonRequired        error
	NoMetadataChange         error
	NotFound                 error
	AlreadyDeleted           error
	InvalidDescription       error
	InvalidCategory          error
	CategoryNotFound         error
	CategoryNameConflict     error
	CategoryDefaultImmutable error
	InvalidCoverSource       error
	CoverFileRequired        error
	CoverSpaceNameRequired   error
	CoverAssetNotFound       error
	CoverImageInvalid        error
	CoverImageTooLarge       error
	CoverImageTooManyPixels  error
	FontUnavailable          error
	TransferTargetRequired   error
	TransferTargetNotFound   error
	TransferTargetNotMember  error
	TransferToSelf           error
	MemberTargetRequired     error
	MemberTargetNotFound     error
	MemberInvalidRole        error
	MemberNotFound           error
	MemberOwnerImmutable     error
}

var (
	ErrAdminSpaceInvalidStatusFilter      = errors.New("invalid admin space status filter")
	ErrAdminSpaceInvalidVisibilityFilter  = errors.New("invalid admin space visibility filter")
	ErrAdminSpaceInvalidSpaceID           = errors.New("admin space id is invalid")
	ErrAdminSpaceAlreadyExists            = errors.New("admin space id already exists")
	ErrAdminSpaceInvalidName              = errors.New("admin space name is invalid")
	ErrAdminSpaceInvalidVisibility        = errors.New("admin space visibility is invalid")
	ErrAdminSpaceInvalidStatus            = errors.New("admin space status is invalid")
	ErrAdminSpaceBanReasonRequired        = errors.New("admin space ban reason is required")
	ErrAdminSpaceNoMetadataChange         = errors.New("admin space metadata change is required")
	ErrAdminSpaceNotFound                 = errors.New("admin space target not found")
	ErrAdminSpaceAlreadyDeleted           = errors.New("admin space target already deleted")
	ErrAdminSpaceInvalidDescription       = errors.New("admin space description is invalid")
	ErrAdminSpaceInvalidCategory          = errors.New("admin space category is invalid")
	ErrAdminSpaceCategoryNotFound         = errors.New("admin space category not found")
	ErrAdminSpaceCategoryNameConflict     = errors.New("admin space category name conflict")
	ErrAdminSpaceCategoryDefaultImmutable = errors.New("admin space default category is immutable")
	ErrAdminSpaceInvalidCoverSource       = errors.New("admin space cover source is invalid")
	ErrAdminSpaceCoverFileRequired        = errors.New("admin space cover file is required")
	ErrAdminSpaceCoverSpaceNameRequired   = errors.New("admin space cover space name is required")
	ErrAdminSpaceCoverAssetNotFound       = errors.New("admin space cover asset not found")
	ErrAdminSpaceCoverImageInvalid        = errors.New("admin space cover image is invalid")
	ErrAdminSpaceCoverImageTooLarge       = errors.New("admin space cover image is too large")
	ErrAdminSpaceCoverImageTooManyPixels  = errors.New("admin space cover image has too many pixels")
	ErrAdminSpaceFontUnavailable          = errors.New("admin space cover font is unavailable")
	ErrAdminSpaceTransferTargetRequired   = errors.New("admin space transfer target is required")
	ErrAdminSpaceTransferTargetNotFound   = errors.New("admin space transfer target not found")
	ErrAdminSpaceTransferTargetNotMember  = errors.New("admin space transfer target not member")
	ErrAdminSpaceTransferToSelf           = errors.New("admin space transfer to self")
	ErrAdminSpaceMemberTargetRequired     = errors.New("admin space member target is required")
	ErrAdminSpaceMemberTargetNotFound     = errors.New("admin space member target not found")
	ErrAdminSpaceMemberInvalidRole        = errors.New("admin space member role is invalid")
	ErrAdminSpaceMemberNotFound           = errors.New("admin space member not found")
	ErrAdminSpaceMemberOwnerImmutable     = errors.New("admin space owner member is immutable")
)

var defaultAdminSpaceErrorTargets = AdminSpaceErrorTargets{
	Forbidden:                ErrAdminForbidden,
	InvalidStatusFilter:      ErrAdminSpaceInvalidStatusFilter,
	InvalidVisibilityFilter:  ErrAdminSpaceInvalidVisibilityFilter,
	InvalidSpaceID:           ErrAdminSpaceInvalidSpaceID,
	AlreadyExists:            ErrAdminSpaceAlreadyExists,
	InvalidName:              ErrAdminSpaceInvalidName,
	InvalidVisibility:        ErrAdminSpaceInvalidVisibility,
	InvalidStatus:            ErrAdminSpaceInvalidStatus,
	BanReasonRequired:        ErrAdminSpaceBanReasonRequired,
	NoMetadataChange:         ErrAdminSpaceNoMetadataChange,
	NotFound:                 ErrAdminSpaceNotFound,
	AlreadyDeleted:           ErrAdminSpaceAlreadyDeleted,
	InvalidDescription:       ErrAdminSpaceInvalidDescription,
	InvalidCategory:          ErrAdminSpaceInvalidCategory,
	CategoryNotFound:         ErrAdminSpaceCategoryNotFound,
	CategoryNameConflict:     ErrAdminSpaceCategoryNameConflict,
	CategoryDefaultImmutable: ErrAdminSpaceCategoryDefaultImmutable,
	InvalidCoverSource:       ErrAdminSpaceInvalidCoverSource,
	CoverFileRequired:        ErrAdminSpaceCoverFileRequired,
	CoverSpaceNameRequired:   ErrAdminSpaceCoverSpaceNameRequired,
	CoverAssetNotFound:       ErrAdminSpaceCoverAssetNotFound,
	CoverImageInvalid:        ErrAdminSpaceCoverImageInvalid,
	CoverImageTooLarge:       ErrAdminSpaceCoverImageTooLarge,
	CoverImageTooManyPixels:  ErrAdminSpaceCoverImageTooManyPixels,
	FontUnavailable:          ErrAdminSpaceFontUnavailable,
	TransferTargetRequired:   ErrAdminSpaceTransferTargetRequired,
	TransferTargetNotFound:   ErrAdminSpaceTransferTargetNotFound,
	TransferTargetNotMember:  ErrAdminSpaceTransferTargetNotMember,
	TransferToSelf:           ErrAdminSpaceTransferToSelf,
	MemberTargetRequired:     ErrAdminSpaceMemberTargetRequired,
	MemberTargetNotFound:     ErrAdminSpaceMemberTargetNotFound,
	MemberInvalidRole:        ErrAdminSpaceMemberInvalidRole,
	MemberNotFound:           ErrAdminSpaceMemberNotFound,
	MemberOwnerImmutable:     ErrAdminSpaceMemberOwnerImmutable,
}

// MapAdminSpaceError 统一映射管理员空间管理相关错误。
func MapAdminSpaceError(err error, targets ...AdminSpaceErrorTargets) error {
	resolvedTargets := defaultAdminSpaceErrorTargets
	if len(targets) > 0 {
		resolvedTargets = targets[0]
	}

	return MapAppError(
		err,
		AppErrorMapping{
			Target:  resolvedTargets.Forbidden,
			Status:  http.StatusForbidden,
			Code:    "FORBIDDEN",
			Message: "insufficient space admin permission",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidStatusFilter,
			Status:  http.StatusBadRequest,
			Code:    "INVALID_STATUS",
			Message: "status filter is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidVisibilityFilter,
			Status:  http.StatusBadRequest,
			Code:    "INVALID_VISIBILITY",
			Message: "visibility filter is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidSpaceID,
			Status:  http.StatusBadRequest,
			Code:    "INVALID_SPACE_ID",
			Message: "space id is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.AlreadyExists,
			Status:  http.StatusConflict,
			Code:    "SPACE_ALREADY_EXISTS",
			Message: "space id already exists",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidName,
			Status:  http.StatusBadRequest,
			Code:    "INVALID_NAME",
			Message: "space name is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidVisibility,
			Status:  http.StatusBadRequest,
			Code:    "INVALID_VISIBILITY",
			Message: "space visibility is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidStatus,
			Status:  http.StatusBadRequest,
			Code:    "INVALID_STATUS",
			Message: "status must be active or banned",
		},
		AppErrorMapping{
			Target:  resolvedTargets.BanReasonRequired,
			Status:  http.StatusBadRequest,
			Code:    "INVALID_REASON",
			Message: "ban reason is required",
		},
		AppErrorMapping{
			Target:  resolvedTargets.NoMetadataChange,
			Status:  http.StatusBadRequest,
			Code:    "INVALID_REQUEST",
			Message: "metadata change is required",
		},
		AppErrorMapping{
			Target:  resolvedTargets.NotFound,
			Status:  http.StatusNotFound,
			Code:    "SPACE_NOT_FOUND",
			Message: "space not found",
		},
		AppErrorMapping{
			Target:  resolvedTargets.AlreadyDeleted,
			Status:  http.StatusBadRequest,
			Code:    "SPACE_DELETED",
			Message: "space has been deleted",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidDescription,
			Status:  http.StatusBadRequest,
			Code:    "INVALID_DESCRIPTION",
			Message: "space description is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidCategory,
			Status:  http.StatusBadRequest,
			Code:    "INVALID_SPACE_CATEGORY",
			Message: "space category is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.CategoryNotFound,
			Status:  http.StatusNotFound,
			Code:    "SPACE_CATEGORY_NOT_FOUND",
			Message: "space category not found",
		},
		AppErrorMapping{
			Target:  resolvedTargets.CategoryNameConflict,
			Status:  http.StatusConflict,
			Code:    "SPACE_CATEGORY_NAME_EXISTS",
			Message: "space category name already exists",
		},
		AppErrorMapping{
			Target:  resolvedTargets.CategoryDefaultImmutable,
			Status:  http.StatusBadRequest,
			Code:    "SPACE_CATEGORY_DEFAULT_IMMUTABLE",
			Message: "default category cannot be modified",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidCoverSource,
			Status:  http.StatusBadRequest,
			Code:    "INVALID_SOURCE",
			Message: "source must be user_upload or system_generated",
		},
		AppErrorMapping{
			Target:  resolvedTargets.CoverFileRequired,
			Status:  http.StatusBadRequest,
			Code:    "INVALID_UPLOAD_FILE",
			Message: "file is required",
		},
		AppErrorMapping{
			Target:  resolvedTargets.CoverSpaceNameRequired,
			Status:  http.StatusBadRequest,
			Code:    "INVALID_SPACE_NAME",
			Message: "space name is required for system generated cover",
		},
		AppErrorMapping{
			Target:  resolvedTargets.CoverAssetNotFound,
			Status:  http.StatusNotFound,
			Code:    "COVER_ASSET_NOT_FOUND",
			Message: "cover asset not found",
		},
		AppErrorMapping{
			Target:  resolvedTargets.CoverImageInvalid,
			Status:  http.StatusBadRequest,
			Code:    "INVALID_COVER_IMAGE",
			Message: "cover image is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.CoverImageTooLarge,
			Status:  http.StatusRequestEntityTooLarge,
			Code:    "COVER_IMAGE_TOO_LARGE",
			Message: "cover image exceeds 10MB limit",
		},
		AppErrorMapping{
			Target:  resolvedTargets.CoverImageTooManyPixels,
			Status:  http.StatusBadRequest,
			Code:    "COVER_IMAGE_TOO_LARGE",
			Message: "cover image dimensions are too large",
		},
		AppErrorMapping{
			Target:  resolvedTargets.FontUnavailable,
			Status:  http.StatusServiceUnavailable,
			Code:    "COVER_FONT_UNAVAILABLE",
			Message: "system cover font is unavailable",
		},
		AppErrorMapping{
			Target:  resolvedTargets.TransferTargetRequired,
			Status:  http.StatusBadRequest,
			Code:    "TRANSFER_TARGET_REQUIRED",
			Message: "transfer target is required",
		},
		AppErrorMapping{
			Target:  resolvedTargets.TransferTargetNotFound,
			Status:  http.StatusNotFound,
			Code:    "TRANSFER_TARGET_NOT_FOUND",
			Message: "transfer target not found",
		},
		AppErrorMapping{
			Target:  resolvedTargets.TransferTargetNotMember,
			Status:  http.StatusBadRequest,
			Code:    "TRANSFER_TARGET_NOT_MEMBER",
			Message: "transfer target is not a space member",
		},
		AppErrorMapping{
			Target:  resolvedTargets.TransferToSelf,
			Status:  http.StatusBadRequest,
			Code:    "TRANSFER_TARGET_SELF",
			Message: "transfer target is current owner",
		},
		AppErrorMapping{
			Target:  resolvedTargets.MemberTargetRequired,
			Status:  http.StatusBadRequest,
			Code:    "MEMBER_TARGET_REQUIRED",
			Message: "member target is required",
		},
		AppErrorMapping{
			Target:  resolvedTargets.MemberTargetNotFound,
			Status:  http.StatusNotFound,
			Code:    "MEMBER_TARGET_NOT_FOUND",
			Message: "member target not found",
		},
		AppErrorMapping{
			Target:  resolvedTargets.MemberInvalidRole,
			Status:  http.StatusBadRequest,
			Code:    "INVALID_MEMBER_ROLE",
			Message: "member role must be collaborator or reader",
		},
		AppErrorMapping{
			Target:  resolvedTargets.MemberNotFound,
			Status:  http.StatusNotFound,
			Code:    "MEMBER_NOT_FOUND",
			Message: "space member not found",
		},
		AppErrorMapping{
			Target:  resolvedTargets.MemberOwnerImmutable,
			Status:  http.StatusBadRequest,
			Code:    "OWNER_MEMBER_IMMUTABLE",
			Message: "owner member role cannot be changed",
		},
	)
}
