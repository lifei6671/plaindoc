package errcode

import (
	"errors"
	"net/http"

	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
)

// AdminDocumentAttachmentErrorTargets 描述管理员文档附件治理场景的错误目标。
type AdminDocumentAttachmentErrorTargets struct {
	Forbidden                    error
	InvalidStatusFilter          error
	InvalidStorageProviderFilter error
	InvalidAttachmentID          error
	NotFound                     error
}

var (
	ErrAdminDocumentAttachmentInvalidStatusFilter          = errors.New("invalid admin document attachment status filter")
	ErrAdminDocumentAttachmentInvalidStorageProviderFilter = errors.New("invalid admin document attachment storage provider filter")
	ErrAdminDocumentAttachmentInvalidAttachmentID          = errors.New("admin document attachment id is invalid")
	ErrAdminDocumentAttachmentNotFound                     = errors.New("admin document attachment target not found")
)

var defaultAdminDocumentAttachmentErrorTargets = AdminDocumentAttachmentErrorTargets{
	Forbidden:                    ErrAdminForbidden,
	InvalidStatusFilter:          ErrAdminDocumentAttachmentInvalidStatusFilter,
	InvalidStorageProviderFilter: ErrAdminDocumentAttachmentInvalidStorageProviderFilter,
	InvalidAttachmentID:          ErrAdminDocumentAttachmentInvalidAttachmentID,
	NotFound:                     ErrAdminDocumentAttachmentNotFound,
}

// MapAdminDocumentAttachmentError 统一映射管理员文档附件治理相关错误。
func MapAdminDocumentAttachmentError(err error, targets ...AdminDocumentAttachmentErrorTargets) error {
	resolvedTargets := defaultAdminDocumentAttachmentErrorTargets
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
			Target:  resolvedTargets.InvalidStorageProviderFilter,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidSource,
			Message: "storage provider filter is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidAttachmentID,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidRequest,
			Message: "attachment id is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.NotFound,
			Status:  http.StatusNotFound,
			Code:    response.CodeFileNotFound,
			Message: "attachment not found",
		},
	)
}
