package errcode

import (
	"errors"
	"net/http"
)

// AdminAuditErrorTargets 描述管理员审计场景的错误目标。
type AdminAuditErrorTargets struct {
	Forbidden           error
	InvalidModule       error
	InvalidAction       error
	InvalidTargetType   error
	InvalidTargetID     error
	InvalidModuleFilter error
	InvalidActionFilter error
	InvalidTimeRange    error
}

var (
	ErrAdminAuditInvalidModule       = errors.New("admin audit module is invalid")
	ErrAdminAuditInvalidAction       = errors.New("admin audit action is invalid")
	ErrAdminAuditInvalidTargetType   = errors.New("admin audit target type is invalid")
	ErrAdminAuditInvalidTargetID     = errors.New("admin audit target id is invalid")
	ErrAdminAuditInvalidModuleFilter = errors.New("admin audit module filter is invalid")
	ErrAdminAuditInvalidActionFilter = errors.New("admin audit action filter is invalid")
	ErrAdminAuditInvalidTimeRange    = errors.New("admin audit time range is invalid")
)

var defaultAdminAuditErrorTargets = AdminAuditErrorTargets{
	Forbidden:           ErrAdminForbidden,
	InvalidModule:       ErrAdminAuditInvalidModule,
	InvalidAction:       ErrAdminAuditInvalidAction,
	InvalidTargetType:   ErrAdminAuditInvalidTargetType,
	InvalidTargetID:     ErrAdminAuditInvalidTargetID,
	InvalidModuleFilter: ErrAdminAuditInvalidModuleFilter,
	InvalidActionFilter: ErrAdminAuditInvalidActionFilter,
	InvalidTimeRange:    ErrAdminAuditInvalidTimeRange,
}

// MapAdminAuditError 统一映射管理员审计相关错误。
func MapAdminAuditError(err error, targets ...AdminAuditErrorTargets) error {
	resolvedTargets := defaultAdminAuditErrorTargets
	if len(targets) > 0 {
		resolvedTargets = targets[0]
	}

	return MapAppError(
		err,
		AppErrorMapping{
			Target:  resolvedTargets.Forbidden,
			Status:  http.StatusForbidden,
			Code:    "FORBIDDEN",
			Message: "insufficient admin permission",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidModule,
			Status:  http.StatusBadRequest,
			Code:    "INVALID_MODULE",
			Message: "module is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidAction,
			Status:  http.StatusBadRequest,
			Code:    "INVALID_ACTION",
			Message: "action is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidTargetType,
			Status:  http.StatusBadRequest,
			Code:    "INVALID_REQUEST",
			Message: "target type is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidTargetID,
			Status:  http.StatusBadRequest,
			Code:    "INVALID_REQUEST",
			Message: "target id is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidModuleFilter,
			Status:  http.StatusBadRequest,
			Code:    "INVALID_MODULE",
			Message: "module filter is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidActionFilter,
			Status:  http.StatusBadRequest,
			Code:    "INVALID_ACTION",
			Message: "action filter is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidTimeRange,
			Status:  http.StatusBadRequest,
			Code:    "INVALID_TIME_RANGE",
			Message: "from must be before to",
		},
	)
}
