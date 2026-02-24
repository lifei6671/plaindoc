package errcode

import (
	"errors"
	"net/http"
)

// AdminSystemConfigErrorTargets 描述管理员系统配置场景的错误目标。
type AdminSystemConfigErrorTargets struct {
	Forbidden       error
	InvalidKey      error
	InvalidValue    error
	ExpectedVersion error
	VersionConflict error
}

var (
	ErrAdminSystemConfigInvalidKey      = errors.New("admin system config key is invalid")
	ErrAdminSystemConfigInvalidValue    = errors.New("admin system config value is invalid")
	ErrAdminSystemConfigExpectedVersion = errors.New("admin system config expected version is invalid")
	ErrAdminSystemConfigVersionConflict = errors.New("admin system config version conflict")
)

var defaultAdminSystemConfigErrorTargets = AdminSystemConfigErrorTargets{
	Forbidden:       ErrAdminForbidden,
	InvalidKey:      ErrAdminSystemConfigInvalidKey,
	InvalidValue:    ErrAdminSystemConfigInvalidValue,
	ExpectedVersion: ErrAdminSystemConfigExpectedVersion,
	VersionConflict: ErrAdminSystemConfigVersionConflict,
}

// MapAdminSystemConfigError 统一映射管理员系统配置相关错误。
func MapAdminSystemConfigError(err error, targets ...AdminSystemConfigErrorTargets) error {
	resolvedTargets := defaultAdminSystemConfigErrorTargets
	if len(targets) > 0 {
		resolvedTargets = targets[0]
	}

	return MapAppError(
		err,
		AppErrorMapping{
			Target:  resolvedTargets.Forbidden,
			Status:  http.StatusForbidden,
			Code:    "FORBIDDEN",
			Message: "platform admin role is required",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidKey,
			Status:  http.StatusBadRequest,
			Code:    "INVALID_CONFIG_KEY",
			Message: "config key is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidValue,
			Status:  http.StatusBadRequest,
			Code:    "INVALID_CONFIG_VALUE",
			Message: "config value is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.ExpectedVersion,
			Status:  http.StatusBadRequest,
			Code:    "INVALID_EXPECTED_VERSION",
			Message: "expectedVersion must be positive integer",
		},
		AppErrorMapping{
			Target:  resolvedTargets.VersionConflict,
			Status:  http.StatusConflict,
			Code:    "CONFIG_VERSION_CONFLICT",
			Message: "config version conflict",
		},
	)
}
