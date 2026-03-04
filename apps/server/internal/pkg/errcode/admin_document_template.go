package errcode

import (
	"errors"
	"net/http"

	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
)

// AdminDocumentTemplateErrorTargets 描述管理员文档模板治理错误目标。
type AdminDocumentTemplateErrorTargets struct {
	Forbidden           error
	InvalidTemplateID   error
	InvalidSceneKey     error
	InvalidSceneName    error
	InvalidName         error
	InvalidDescription  error
	InvalidDefaultTitle error
	InvalidSort         error
	InvalidContent      error
	InvalidKeyword      error
	AlreadyExists       error
	NoChanges           error
	NotFound            error
	BuiltinImmutable    error
}

var (
	ErrAdminDocumentTemplateInvalidTemplateID   = errors.New("admin document template id is invalid")
	ErrAdminDocumentTemplateInvalidSceneKey     = errors.New("admin document template scene key is invalid")
	ErrAdminDocumentTemplateInvalidSceneName    = errors.New("admin document template scene name is invalid")
	ErrAdminDocumentTemplateInvalidName         = errors.New("admin document template name is invalid")
	ErrAdminDocumentTemplateInvalidDescription  = errors.New("admin document template description is invalid")
	ErrAdminDocumentTemplateInvalidDefaultTitle = errors.New("admin document template default title is invalid")
	ErrAdminDocumentTemplateInvalidSort         = errors.New("admin document template sort is invalid")
	ErrAdminDocumentTemplateInvalidContent      = errors.New("admin document template content is invalid")
	ErrAdminDocumentTemplateInvalidKeyword      = errors.New("admin document template keyword is invalid")
	ErrAdminDocumentTemplateAlreadyExists       = errors.New("admin document template already exists")
	ErrAdminDocumentTemplateNoChanges           = errors.New("admin document template no changes")
	ErrAdminDocumentTemplateNotFound            = errors.New("admin document template not found")
	ErrAdminDocumentTemplateBuiltinImmutable    = errors.New("admin document template builtin immutable")
)

var defaultAdminDocumentTemplateErrorTargets = AdminDocumentTemplateErrorTargets{
	Forbidden:           ErrAdminForbidden,
	InvalidTemplateID:   ErrAdminDocumentTemplateInvalidTemplateID,
	InvalidSceneKey:     ErrAdminDocumentTemplateInvalidSceneKey,
	InvalidSceneName:    ErrAdminDocumentTemplateInvalidSceneName,
	InvalidName:         ErrAdminDocumentTemplateInvalidName,
	InvalidDescription:  ErrAdminDocumentTemplateInvalidDescription,
	InvalidDefaultTitle: ErrAdminDocumentTemplateInvalidDefaultTitle,
	InvalidSort:         ErrAdminDocumentTemplateInvalidSort,
	InvalidContent:      ErrAdminDocumentTemplateInvalidContent,
	InvalidKeyword:      ErrAdminDocumentTemplateInvalidKeyword,
	AlreadyExists:       ErrAdminDocumentTemplateAlreadyExists,
	NoChanges:           ErrAdminDocumentTemplateNoChanges,
	NotFound:            ErrAdminDocumentTemplateNotFound,
	BuiltinImmutable:    ErrAdminDocumentTemplateBuiltinImmutable,
}

// MapAdminDocumentTemplateError 统一映射管理员文档模板治理错误。
func MapAdminDocumentTemplateError(err error, targets ...AdminDocumentTemplateErrorTargets) error {
	resolvedTargets := defaultAdminDocumentTemplateErrorTargets
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
			Target:  resolvedTargets.InvalidTemplateID,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidTemplateID,
			Message: "template id is invalid",
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
			Target:  resolvedTargets.InvalidName,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidName,
			Message: "template name is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidDescription,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidDescription,
			Message: "description is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidDefaultTitle,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidRequest,
			Message: "default title is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidSort,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidRequest,
			Message: "sort is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidContent,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidRequest,
			Message: "content is invalid",
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
			Message: "template id already exists",
		},
		AppErrorMapping{
			Target:  resolvedTargets.NoChanges,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidRequest,
			Message: "template update changes are required",
		},
		AppErrorMapping{
			Target:  resolvedTargets.NotFound,
			Status:  http.StatusNotFound,
			Code:    response.CodeTemplateNotFound,
			Message: "template not found",
		},
		AppErrorMapping{
			Target:  resolvedTargets.BuiltinImmutable,
			Status:  http.StatusBadRequest,
			Code:    response.CodeTemplateBuiltinImmutable,
			Message: "builtin template can not be modified",
		},
	)
}
