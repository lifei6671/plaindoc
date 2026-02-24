package errcode

import (
	"errors"
	"net/http"
)

// AdminOperationTokenErrorTargets 描述管理员一次性操作 token 场景的错误目标。
type AdminOperationTokenErrorTargets struct {
	Forbidden        error
	TokenRequired    error
	InvalidOperation error
	Replayed         error
	Expired          error
	ScopeMismatch    error
	Invalid          error
}

var (
	ErrAdminOperationTokenRequired         = errors.New("admin operation token is required")
	ErrAdminOperationTokenInvalid          = errors.New("admin operation token is invalid")
	ErrAdminOperationTokenExpired          = errors.New("admin operation token is expired")
	ErrAdminOperationTokenReplayed         = errors.New("admin operation token is replayed")
	ErrAdminOperationTokenScopeMismatch    = errors.New("admin operation token scope mismatch")
	ErrAdminOperationTokenInvalidOperation = errors.New("admin operation token operation is invalid")
)

var defaultAdminOperationTokenErrorTargets = AdminOperationTokenErrorTargets{
	Forbidden:        ErrAdminForbidden,
	TokenRequired:    ErrAdminOperationTokenRequired,
	InvalidOperation: ErrAdminOperationTokenInvalidOperation,
	Replayed:         ErrAdminOperationTokenReplayed,
	Expired:          ErrAdminOperationTokenExpired,
	ScopeMismatch:    ErrAdminOperationTokenScopeMismatch,
	Invalid:          ErrAdminOperationTokenInvalid,
}

// MapAdminOperationTokenError 统一映射管理员一次性操作 token 相关错误。
func MapAdminOperationTokenError(err error, targets ...AdminOperationTokenErrorTargets) error {
	resolvedTargets := defaultAdminOperationTokenErrorTargets
	if len(targets) > 0 {
		resolvedTargets = targets[0]
	}

	return MapAppError(
		err,
		AppErrorMapping{
			Target:  resolvedTargets.Forbidden,
			Status:  http.StatusForbidden,
			Code:    "FORBIDDEN",
			Message: "admin role is required",
		},
		AppErrorMapping{
			Target:  resolvedTargets.TokenRequired,
			Status:  http.StatusBadRequest,
			Code:    "OPERATION_TOKEN_REQUIRED",
			Message: "operation token is required",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidOperation,
			Status:  http.StatusBadRequest,
			Code:    "INVALID_OPERATION",
			Message: "operation is required",
		},
		AppErrorMapping{
			Target:  resolvedTargets.Replayed,
			Status:  http.StatusConflict,
			Code:    "OPERATION_TOKEN_REPLAYED",
			Message: "operation token already used",
		},
		AppErrorMapping{
			Target:  resolvedTargets.Expired,
			Status:  http.StatusConflict,
			Code:    "OPERATION_TOKEN_EXPIRED",
			Message: "operation token is expired",
		},
		AppErrorMapping{
			Target:  resolvedTargets.ScopeMismatch,
			Status:  http.StatusConflict,
			Code:    "OPERATION_TOKEN_SCOPE_MISMATCH",
			Message: "operation token scope mismatch",
		},
		AppErrorMapping{
			Target:  resolvedTargets.Invalid,
			Status:  http.StatusConflict,
			Code:    "OPERATION_TOKEN_INVALID",
			Message: "operation token is invalid",
		},
	)
}
