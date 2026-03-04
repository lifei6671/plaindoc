package errcode

import (
	"errors"
	"net/http"

	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
)

// AdminDocumentTemplateSceneErrorTargets 描述管理员文档模板场景治理错误目标。
type AdminDocumentTemplateSceneErrorTargets struct {
	Forbidden          error
	InvalidSceneKey    error
	InvalidSceneName   error
	InvalidDescription error
	InvalidSort        error
	InvalidKeyword     error
	AlreadyExists      error
	NoChanges          error
	NotFound           error
	BuiltinImmutable   error
	SceneInUse         error
}

var (
	ErrAdminDocumentTemplateSceneInvalidSceneKey    = errors.New("admin document template scene key is invalid")
	ErrAdminDocumentTemplateSceneInvalidSceneName   = errors.New("admin document template scene name is invalid")
	ErrAdminDocumentTemplateSceneInvalidDescription = errors.New("admin document template scene description is invalid")
	ErrAdminDocumentTemplateSceneInvalidSort        = errors.New("admin document template scene sort is invalid")
	ErrAdminDocumentTemplateSceneInvalidKeyword     = errors.New("admin document template scene keyword is invalid")
	ErrAdminDocumentTemplateSceneAlreadyExists      = errors.New("admin document template scene already exists")
	ErrAdminDocumentTemplateSceneNoChanges          = errors.New("admin document template scene no changes")
	ErrAdminDocumentTemplateSceneNotFound           = errors.New("admin document template scene not found")
	ErrAdminDocumentTemplateSceneBuiltinImmutable   = errors.New("admin document template scene builtin immutable")
	ErrAdminDocumentTemplateSceneInUse              = errors.New("admin document template scene in use")
)

var defaultAdminDocumentTemplateSceneErrorTargets = AdminDocumentTemplateSceneErrorTargets{
	Forbidden:          ErrAdminForbidden,
	InvalidSceneKey:    ErrAdminDocumentTemplateSceneInvalidSceneKey,
	InvalidSceneName:   ErrAdminDocumentTemplateSceneInvalidSceneName,
	InvalidDescription: ErrAdminDocumentTemplateSceneInvalidDescription,
	InvalidSort:        ErrAdminDocumentTemplateSceneInvalidSort,
	InvalidKeyword:     ErrAdminDocumentTemplateSceneInvalidKeyword,
	AlreadyExists:      ErrAdminDocumentTemplateSceneAlreadyExists,
	NoChanges:          ErrAdminDocumentTemplateSceneNoChanges,
	NotFound:           ErrAdminDocumentTemplateSceneNotFound,
	BuiltinImmutable:   ErrAdminDocumentTemplateSceneBuiltinImmutable,
	SceneInUse:         ErrAdminDocumentTemplateSceneInUse,
}

// MapAdminDocumentTemplateSceneError 统一映射管理员文档模板场景治理错误。
func MapAdminDocumentTemplateSceneError(err error, targets ...AdminDocumentTemplateSceneErrorTargets) error {
	resolvedTargets := defaultAdminDocumentTemplateSceneErrorTargets
	if len(targets) > 0 {
		resolvedTargets = targets[0]
	}

	return MapAppError(
		err,
		AppErrorMapping{
			Target:  resolvedTargets.Forbidden,
			Status:  http.StatusForbidden,
			Code:    response.CodeForbidden,
			Message: "admin role is required",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidSceneKey,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidRequest,
			Message: "scene key is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidSceneName,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidName,
			Message: "scene name is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidDescription,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidDescription,
			Message: "description is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidSort,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidRequest,
			Message: "sort is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidKeyword,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidRequest,
			Message: "keyword is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.AlreadyExists,
			Status:  http.StatusConflict,
			Code:    response.CodeTemplateAlreadyExists,
			Message: "scene key already exists",
		},
		AppErrorMapping{
			Target:  resolvedTargets.NoChanges,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidRequest,
			Message: "scene update changes are required",
		},
		AppErrorMapping{
			Target:  resolvedTargets.NotFound,
			Status:  http.StatusNotFound,
			Code:    response.CodeTemplateNotFound,
			Message: "scene not found",
		},
		AppErrorMapping{
			Target:  resolvedTargets.BuiltinImmutable,
			Status:  http.StatusBadRequest,
			Code:    response.CodeTemplateBuiltinImmutable,
			Message: "builtin scene can not be modified",
		},
		AppErrorMapping{
			Target:  resolvedTargets.SceneInUse,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidRequest,
			Message: "scene is referenced by templates",
		},
	)
}
