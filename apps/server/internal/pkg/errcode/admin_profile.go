package errcode

import (
	"errors"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"net/http"
)

// AdminProfileErrorTargets 描述管理员个人资料场景的错误目标。
type AdminProfileErrorTargets struct {
	Forbidden              error
	NotFound               error
	InvalidName            error
	InvalidAvatarURL       error
	CurrentPasswordInvalid error
	PasswordTooShort       error
	PasswordUnchanged      error
}

var (
	ErrAdminProfileNotFound               = errors.New("admin profile user not found")
	ErrAdminProfileInvalidName            = errors.New("admin profile name is invalid")
	ErrAdminProfileInvalidAvatarURL       = errors.New("admin profile avatar url is invalid")
	ErrAdminProfileCurrentPasswordInvalid = errors.New("admin profile current password is invalid")
	ErrAdminProfilePasswordTooShort       = errors.New("admin profile new password is too short")
	ErrAdminProfilePasswordUnchanged      = errors.New("admin profile new password equals current password")
)

var defaultAdminProfileErrorTargets = AdminProfileErrorTargets{
	Forbidden:              ErrAdminForbidden,
	NotFound:               ErrAdminProfileNotFound,
	InvalidName:            ErrAdminProfileInvalidName,
	InvalidAvatarURL:       ErrAdminProfileInvalidAvatarURL,
	CurrentPasswordInvalid: ErrAdminProfileCurrentPasswordInvalid,
	PasswordTooShort:       ErrAdminProfilePasswordTooShort,
	PasswordUnchanged:      ErrAdminProfilePasswordUnchanged,
}

// MapAdminProfileError 统一映射管理员个人资料相关错误。
func MapAdminProfileError(err error, targets ...AdminProfileErrorTargets) error {
	resolvedTargets := defaultAdminProfileErrorTargets
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
			Target:  resolvedTargets.NotFound,
			Status:  http.StatusNotFound,
			Code:    response.CodeUserNotFound,
			Message: "admin user not found",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidName,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidName,
			Message: "name is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidAvatarURL,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidAvatarURL,
			Message: "avatar url is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.CurrentPasswordInvalid,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidCurrentPassword,
			Message: "current password is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.PasswordTooShort,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidNewPassword,
			Message: "new password must be at least 6 characters",
		},
		AppErrorMapping{
			Target:  resolvedTargets.PasswordUnchanged,
			Status:  http.StatusBadRequest,
			Code:    response.CodePasswordUnchanged,
			Message: "new password can not be same as current password",
		},
	)
}
