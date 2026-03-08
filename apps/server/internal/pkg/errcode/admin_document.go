package errcode

import (
	"errors"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"net/http"
)

// AdminDocumentErrorTargets 描述管理员文档管理场景的错误目标。
type AdminDocumentErrorTargets struct {
	Forbidden               error
	InvalidStatusFilter     error
	InvalidVisibilityFilter error
	InvalidFormatFilter     error
	InvalidDocumentID       error
	InvalidStatus           error
	BanReasonRequired       error
	NotFound                error
	AlreadyDeleted          error
}

var (
	ErrAdminDocumentInvalidStatusFilter     = errors.New("invalid admin document status filter")
	ErrAdminDocumentInvalidVisibilityFilter = errors.New("invalid admin document visibility filter")
	ErrAdminDocumentInvalidFormatFilter     = errors.New("invalid admin document format filter")
	ErrAdminDocumentInvalidDocumentID       = errors.New("admin document id is invalid")
	ErrAdminDocumentInvalidStatus           = errors.New("admin document status is invalid")
	ErrAdminDocumentBanReasonRequired       = errors.New("admin document ban reason is required")
	ErrAdminDocumentNotFound                = errors.New("admin document target not found")
	ErrAdminDocumentAlreadyDeleted          = errors.New("admin document target already deleted")
)

var defaultAdminDocumentErrorTargets = AdminDocumentErrorTargets{
	Forbidden:               ErrAdminForbidden,
	InvalidStatusFilter:     ErrAdminDocumentInvalidStatusFilter,
	InvalidVisibilityFilter: ErrAdminDocumentInvalidVisibilityFilter,
	InvalidFormatFilter:     ErrAdminDocumentInvalidFormatFilter,
	InvalidDocumentID:       ErrAdminDocumentInvalidDocumentID,
	InvalidStatus:           ErrAdminDocumentInvalidStatus,
	BanReasonRequired:       ErrAdminDocumentBanReasonRequired,
	NotFound:                ErrAdminDocumentNotFound,
	AlreadyDeleted:          ErrAdminDocumentAlreadyDeleted,
}

// MapAdminDocumentError 统一映射管理员文档管理相关错误。
func MapAdminDocumentError(err error, targets ...AdminDocumentErrorTargets) error {
	resolvedTargets := defaultAdminDocumentErrorTargets
	if len(targets) > 0 {
		resolvedTargets = targets[0]
	}

	return MapAppError(
		err,
		AppErrorMapping{
			Target:  resolvedTargets.Forbidden,
			Status:  http.StatusForbidden,
			Code:    response.CodeForbidden,
			Message: "insufficient space admin permission",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidStatusFilter,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidStatus,
			Message: "status filter is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidVisibilityFilter,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidVisibility,
			Message: "visibility filter is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidFormatFilter,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidRequest,
			Message: "document format filter is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidDocumentID,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidDocumentID,
			Message: "document id is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidStatus,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidStatus,
			Message: "status must be active or banned",
		},
		AppErrorMapping{
			Target:  resolvedTargets.BanReasonRequired,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidReason,
			Message: "ban reason is required",
		},
		AppErrorMapping{
			Target:  resolvedTargets.NotFound,
			Status:  http.StatusNotFound,
			Code:    response.CodeDocumentNotFound,
			Message: "document not found",
		},
		AppErrorMapping{
			Target:  resolvedTargets.AlreadyDeleted,
			Status:  http.StatusBadRequest,
			Code:    response.CodeDocumentDeleted,
			Message: "document has been deleted",
		},
	)
}
