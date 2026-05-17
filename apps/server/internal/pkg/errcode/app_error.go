package errcode

import (
	"errors"
	"net/http"
)

// AppError 为可直接输出到 API 响应的业务错误。
//
// 该类型通过 Unwrap 保留原始错误链，兼容 errors.Is / errors.As。
type AppError struct {
	status  int
	code    string
	message string
	cause   error
}

// AppErrorMapping 定义 sentinel 错误到响应语义的映射规则。
type AppErrorMapping struct {
	Target  error
	Status  int
	Code    string
	Message string
}

// NewAppError 创建新的业务错误。
func NewAppError(status int, code string, message string) *AppError {
	return &AppError{
		status:  normalizeAppErrorStatus(status),
		code:    code,
		message: message,
	}
}

// WrapAppError 基于已有错误创建业务错误，保留原始错误链。
func WrapAppError(err error, status int, code string, message string) *AppError {
	if err == nil {
		return NewAppError(status, code, message)
	}
	return &AppError{
		status:  normalizeAppErrorStatus(status),
		code:    code,
		message: message,
		cause:   err,
	}
}

func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	if e.cause != nil {
		return e.cause.Error()
	}
	return e.message
}

// Unwrap 暴露原始错误以支持 errors.Is / errors.As。
func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// HTTPStatus 返回该错误对应的 HTTP 状态。
func (e *AppError) HTTPStatus() int {
	if e == nil {
		return http.StatusInternalServerError
	}
	return normalizeAppErrorStatus(e.status)
}

// ErrorCode 返回统一错误码 key（如 FORBIDDEN / INVALID_REQUEST）。
func (e *AppError) ErrorCode() string {
	if e == nil {
		return "INTERNAL_ERROR"
	}
	if e.code == "" {
		return "INTERNAL_ERROR"
	}
	return e.code
}

// ErrorMessage 返回对外展示的错误信息。
func (e *AppError) ErrorMessage() string {
	if e == nil {
		return "internal server error"
	}
	if e.message == "" {
		return "internal server error"
	}
	return e.message
}

// AsAppError 读取错误链中的 AppError。
func AsAppError(err error) (*AppError, bool) {
	if err == nil {
		return nil, false
	}
	if appErr, ok := errors.AsType[*AppError](err); ok {
		return appErr, true
	}
	return nil, false
}

// MapAppError 将已知 sentinel 错误映射为 AppError。
//
// 如果 err 已经是 AppError 或未命中映射规则，则原样返回。
func MapAppError(err error, mappings ...AppErrorMapping) error {
	if err == nil {
		return nil
	}
	if _, ok := AsAppError(err); ok {
		return err
	}
	for _, mapping := range mappings {
		if mapping.Target == nil {
			continue
		}
		if errors.Is(err, mapping.Target) {
			return WrapAppError(err, mapping.Status, mapping.Code, mapping.Message)
		}
	}
	return err
}

func normalizeAppErrorStatus(status int) int {
	if status < 100 || status > 599 {
		return http.StatusInternalServerError
	}
	return status
}
