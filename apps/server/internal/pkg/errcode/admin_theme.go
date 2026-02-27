package errcode

import (
	"errors"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"net/http"
)

// AdminThemeErrorTargets 描述管理员主题管理场景的错误目标。
type AdminThemeErrorTargets struct {
	Forbidden        error
	InvalidThemeID   error
	InvalidName      error
	InvalidSyntax    error
	AlreadyExists    error
	NoChanges        error
	NotFound         error
	BuiltinImmutable error
	InUse            error
}

var (
	ErrAdminThemeInvalidThemeID   = errors.New("admin theme id is invalid")
	ErrAdminThemeInvalidName      = errors.New("admin theme name is invalid")
	ErrAdminThemeInvalidSyntax    = errors.New("admin theme syntax is invalid")
	ErrAdminThemeAlreadyExists    = errors.New("admin theme already exists")
	ErrAdminThemeNotFound         = errors.New("admin theme not found")
	ErrAdminThemeNoChanges        = errors.New("admin theme no changes")
	ErrAdminThemeBuiltinImmutable = errors.New("admin theme builtin immutable")
	ErrAdminThemeInUse            = errors.New("admin theme in use")
)

var defaultAdminThemeErrorTargets = AdminThemeErrorTargets{
	Forbidden:        ErrAdminForbidden,
	InvalidThemeID:   ErrAdminThemeInvalidThemeID,
	InvalidName:      ErrAdminThemeInvalidName,
	InvalidSyntax:    ErrAdminThemeInvalidSyntax,
	AlreadyExists:    ErrAdminThemeAlreadyExists,
	NoChanges:        ErrAdminThemeNoChanges,
	NotFound:         ErrAdminThemeNotFound,
	BuiltinImmutable: ErrAdminThemeBuiltinImmutable,
	InUse:            ErrAdminThemeInUse,
}

// MapAdminThemeError 统一映射管理员主题管理相关错误。
func MapAdminThemeError(err error, targets ...AdminThemeErrorTargets) error {
	resolvedTargets := defaultAdminThemeErrorTargets
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
			Target:  resolvedTargets.InvalidThemeID,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidThemeID,
			Message: "theme id is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidName,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidName,
			Message: "theme name is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidSyntax,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidSyntaxTheme,
			Message: "syntax theme must be one-light or one-dark",
		},
		AppErrorMapping{
			Target:  resolvedTargets.AlreadyExists,
			Status:  http.StatusConflict,
			Code:    response.CodeThemeAlreadyExists,
			Message: "theme id already exists",
		},
		AppErrorMapping{
			Target:  resolvedTargets.NoChanges,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidRequest,
			Message: "theme update changes are required",
		},
		AppErrorMapping{
			Target:  resolvedTargets.NotFound,
			Status:  http.StatusNotFound,
			Code:    response.CodeThemeNotFound,
			Message: "theme not found",
		},
		AppErrorMapping{
			Target:  resolvedTargets.BuiltinImmutable,
			Status:  http.StatusBadRequest,
			Code:    response.CodeThemeBuiltinImmutable,
			Message: "builtin theme can not be modified",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InUse,
			Status:  http.StatusConflict,
			Code:    response.CodeThemeInUse,
			Message: "theme is referenced by documents",
		},
	)
}
