package errcode

import (
	"errors"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"net/http"
)

// AdminUserErrorTargets 描述管理员用户管理场景的错误目标。
type AdminUserErrorTargets struct {
	Forbidden            error
	InvalidStatusFilter  error
	InvalidStatus        error
	BanReasonRequired    error
	NotFound             error
	InvalidUserID        error
	SelfOperationBlocked error
	AlreadyDeleted       error
	InvalidEmail         error
	InvalidName          error
	InvalidPassword      error
	EmailAlreadyExists   error
	InvalidRole          error
	RoleForbidden        error
}

var (
	ErrAdminUserInvalidStatusFilter  = errors.New("invalid admin user status filter")
	ErrAdminUserInvalidStatus        = errors.New("invalid admin user status")
	ErrAdminUserBanReasonRequired    = errors.New("admin user ban reason is required")
	ErrAdminUserNotFound             = errors.New("admin user target not found")
	ErrAdminUserInvalidUserID        = errors.New("admin user id is invalid")
	ErrAdminUserSelfOperationBlocked = errors.New("admin user self operation is blocked")
	ErrAdminUserAlreadyDeleted       = errors.New("admin user target already deleted")
	ErrAdminUserInvalidEmail         = errors.New("admin user email is invalid")
	ErrAdminUserInvalidName          = errors.New("admin user name is invalid")
	ErrAdminUserInvalidPassword      = errors.New("admin user password is invalid")
	ErrAdminUserEmailAlreadyExists   = errors.New("admin user email already exists")
	ErrAdminUserInvalidRole          = errors.New("admin user role is invalid")
	ErrAdminUserRoleForbidden        = errors.New("admin user role operation forbidden")
)

var defaultAdminUserErrorTargets = AdminUserErrorTargets{
	Forbidden:            ErrAdminForbidden,
	InvalidStatusFilter:  ErrAdminUserInvalidStatusFilter,
	InvalidStatus:        ErrAdminUserInvalidStatus,
	BanReasonRequired:    ErrAdminUserBanReasonRequired,
	NotFound:             ErrAdminUserNotFound,
	InvalidUserID:        ErrAdminUserInvalidUserID,
	SelfOperationBlocked: ErrAdminUserSelfOperationBlocked,
	AlreadyDeleted:       ErrAdminUserAlreadyDeleted,
	InvalidEmail:         ErrAdminUserInvalidEmail,
	InvalidName:          ErrAdminUserInvalidName,
	InvalidPassword:      ErrAdminUserInvalidPassword,
	EmailAlreadyExists:   ErrAdminUserEmailAlreadyExists,
	InvalidRole:          ErrAdminUserInvalidRole,
	RoleForbidden:        ErrAdminUserRoleForbidden,
}

// MapAdminUserError 统一映射管理员用户管理相关错误。
func MapAdminUserError(err error, targets ...AdminUserErrorTargets) error {
	resolvedTargets := defaultAdminUserErrorTargets
	if len(targets) > 0 {
		resolvedTargets = targets[0]
	}

	return MapAppError(
		err,
		AppErrorMapping{
			Target:  resolvedTargets.Forbidden,
			Status:  http.StatusForbidden,
			Code:    response.CodeForbidden,
			Message: "platform admin role is required",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidStatusFilter,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidStatus,
			Message: "status filter is invalid",
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
			Code:    response.CodeUserNotFound,
			Message: "user not found",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidUserID,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidUserID,
			Message: "user id is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.SelfOperationBlocked,
			Status:  http.StatusBadRequest,
			Code:    response.CodeSelfOperationForbidden,
			Message: "self operation is not allowed",
		},
		AppErrorMapping{
			Target:  resolvedTargets.AlreadyDeleted,
			Status:  http.StatusBadRequest,
			Code:    response.CodeUserDeleted,
			Message: "user has been deleted",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidEmail,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidEmail,
			Message: "email is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidName,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidName,
			Message: "name is required",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidPassword,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidPassword,
			Message: "password must be at least 6 characters",
		},
		AppErrorMapping{
			Target:  resolvedTargets.EmailAlreadyExists,
			Status:  http.StatusConflict,
			Code:    response.CodeEmailAlreadyExists,
			Message: "email already exists",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidRole,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidRole,
			Message: "role must be user or space_admin or platform_admin",
		},
		AppErrorMapping{
			Target:  resolvedTargets.RoleForbidden,
			Status:  http.StatusForbidden,
			Code:    response.CodeRoleForbidden,
			Message: "can not edit higher role user",
		},
	)
}
