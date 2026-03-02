package errcode

import (
	"errors"
	"net/http"

	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
)

// AdminSearchAnalyzerErrorTargets 描述管理员分词治理场景错误目标。
type AdminSearchAnalyzerErrorTargets struct {
	Forbidden         error
	InvalidAnalyzer   error
	InvalidMode       error
	InvalidStatus     error
	InvalidDictEntry  error
	InvalidTerm       error
	InvalidWeight     error
	AlreadyExists     error
	NotFound          error
	AnalyzerNotActive error
}

var (
	ErrAdminSearchAnalyzerInvalidAnalyzer   = errors.New("admin search analyzer is invalid")
	ErrAdminSearchAnalyzerInvalidMode       = errors.New("admin search analyzer mode is invalid")
	ErrAdminSearchAnalyzerInvalidStatus     = errors.New("admin search analyzer dict entry status is invalid")
	ErrAdminSearchAnalyzerInvalidDictEntry  = errors.New("admin search analyzer dict entry id is invalid")
	ErrAdminSearchAnalyzerInvalidTerm       = errors.New("admin search analyzer dict entry term is invalid")
	ErrAdminSearchAnalyzerInvalidWeight     = errors.New("admin search analyzer dict entry weight is invalid")
	ErrAdminSearchAnalyzerDictEntryExists   = errors.New("admin search analyzer dict entry already exists")
	ErrAdminSearchAnalyzerDictEntryNotFound = errors.New("admin search analyzer dict entry not found")
	ErrAdminSearchAnalyzerNotActive         = errors.New("admin search analyzer is not active")
)

var defaultAdminSearchAnalyzerErrorTargets = AdminSearchAnalyzerErrorTargets{
	Forbidden:         ErrAdminForbidden,
	InvalidAnalyzer:   ErrAdminSearchAnalyzerInvalidAnalyzer,
	InvalidMode:       ErrAdminSearchAnalyzerInvalidMode,
	InvalidStatus:     ErrAdminSearchAnalyzerInvalidStatus,
	InvalidDictEntry:  ErrAdminSearchAnalyzerInvalidDictEntry,
	InvalidTerm:       ErrAdminSearchAnalyzerInvalidTerm,
	InvalidWeight:     ErrAdminSearchAnalyzerInvalidWeight,
	AlreadyExists:     ErrAdminSearchAnalyzerDictEntryExists,
	NotFound:          ErrAdminSearchAnalyzerDictEntryNotFound,
	AnalyzerNotActive: ErrAdminSearchAnalyzerNotActive,
}

// MapAdminSearchAnalyzerError 统一映射管理员分词治理相关错误。
func MapAdminSearchAnalyzerError(err error, targets ...AdminSearchAnalyzerErrorTargets) error {
	resolvedTargets := defaultAdminSearchAnalyzerErrorTargets
	if len(targets) > 0 {
		resolvedTargets = targets[0]
	}

	return MapAppError(
		err,
		AppErrorMapping{
			Target:  resolvedTargets.Forbidden,
			Status:  http.StatusForbidden,
			Code:    response.CodeForbidden,
			Message: "insufficient platform admin permission",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidAnalyzer,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidSource,
			Message: "analyzer is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidMode,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidRequest,
			Message: "mode is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidStatus,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidStatus,
			Message: "status is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidDictEntry,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidRequest,
			Message: "dict entry id is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidTerm,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidRequest,
			Message: "term is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.InvalidWeight,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidRequest,
			Message: "weight is invalid",
		},
		AppErrorMapping{
			Target:  resolvedTargets.AlreadyExists,
			Status:  http.StatusConflict,
			Code:    response.CodeInvalidRequest,
			Message: "dict entry already exists",
		},
		AppErrorMapping{
			Target:  resolvedTargets.NotFound,
			Status:  http.StatusNotFound,
			Code:    response.CodeFileNotFound,
			Message: "dict entry not found",
		},
		AppErrorMapping{
			Target:  resolvedTargets.AnalyzerNotActive,
			Status:  http.StatusBadRequest,
			Code:    response.CodeInvalidOperation,
			Message: "analyzer is not active",
		},
	)
}
