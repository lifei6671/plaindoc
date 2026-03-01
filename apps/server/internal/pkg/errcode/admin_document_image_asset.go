package errcode

import (
	"errors"
	"net/http"

	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
)

// AdminDocumentImageAssetErrorTargets 描述管理员文档图片治理场景的错误目标。
type AdminDocumentImageAssetErrorTargets struct {
	Forbidden                    error
	InvalidStatusFilter          error
	InvalidStorageProviderFilter error
	InvalidImageAssetID          error
	NotFound                     error
}

var (
	ErrAdminDocumentImageAssetInvalidStatusFilter          = errors.New("invalid admin document image asset status filter")
	ErrAdminDocumentImageAssetInvalidStorageProviderFilter = errors.New("invalid admin document image asset storage provider filter")
	ErrAdminDocumentImageAssetInvalidImageAssetID          = errors.New("admin document image asset id is invalid")
	ErrAdminDocumentImageAssetNotFound                     = errors.New("admin document image asset target not found")
)

var defaultAdminDocumentImageAssetErrorTargets = AdminDocumentImageAssetErrorTargets{
	Forbidden:                    ErrAdminForbidden,
	InvalidStatusFilter:          ErrAdminDocumentImageAssetInvalidStatusFilter,
	InvalidStorageProviderFilter: ErrAdminDocumentImageAssetInvalidStorageProviderFilter,
	InvalidImageAssetID:          ErrAdminDocumentImageAssetInvalidImageAssetID,
	NotFound:                     ErrAdminDocumentImageAssetNotFound,
}

// MapAdminDocumentImageAssetError 统一映射管理员文档图片治理相关错误。
func MapAdminDocumentImageAssetError(err error, targets ...AdminDocumentImageAssetErrorTargets) error {
	resolvedTargets := defaultAdminDocumentImageAssetErrorTargets
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
			Target:  resolvedTargets.InvalidImageAssetID,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidRequest,
			Message: "image asset id is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.NotFound,
			Status:  http.StatusNotFound,
			Code:    response.CodeFileNotFound,
			Message: "image asset not found",
		},
	)
}
