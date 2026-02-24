package response

import "net/http"

// 当前文件收敛 `auth.go` 的错误响应模板。
// 统一在 response 层维护 status/code/message，避免在 handler 中散落字面量。
var (
	// AuthErrRegistrationDisabled 对应场景：registration is disabled
	AuthErrRegistrationDisabled = ErrorTemplate{
		Status:  http.StatusForbidden,
		Code:    CodeRegistrationDisabled,
		Message: "registration is disabled",
	}
	// AuthErrRequestBody 对应场景：invalid request body
	AuthErrRequestBody = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "invalid request body",
	}
	// AuthErrEmail 对应场景：email is invalid
	AuthErrEmail = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidEmail,
		Message: "email is invalid",
	}
	// AuthErrPasswordLeast6Characters 对应场景：password must be at least 6 characters
	AuthErrPasswordLeast6Characters = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidPassword,
		Message: "password must be at least 6 characters",
	}
	// AuthErrNameRequired 对应场景：name is required
	AuthErrNameRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidName,
		Message: "name is required",
	}
	// AuthErrEmailAlreadyExists 对应场景：email already exists
	AuthErrEmailAlreadyExists = ErrorTemplate{
		Status:  http.StatusConflict,
		Code:    CodeEmailAlreadyExists,
		Message: "email already exists",
	}
	// AuthErrEmailPasswordRequired 对应场景：email and password are required
	AuthErrEmailPasswordRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "email and password are required",
	}
	// AuthErrEmailPassword 对应场景：invalid email or password
	AuthErrEmailPassword = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeInvalidCredentials,
		Message: "invalid email or password",
	}
	// AuthErrUserHasBeenBanned 对应场景：user has been banned
	AuthErrUserHasBeenBanned = ErrorTemplate{
		Status:  http.StatusForbidden,
		Code:    CodeUserBanned,
		Message: "user has been banned",
	}
	// AuthErrUserHasBeenDeleted 对应场景：user has been deleted
	AuthErrUserHasBeenDeleted = ErrorTemplate{
		Status:  http.StatusForbidden,
		Code:    CodeUserDeleted,
		Message: "user has been deleted",
	}
	// AuthErrRefreshTokenRequired 对应场景：refresh token is required
	AuthErrRefreshTokenRequired = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "refresh token is required",
	}
	// AuthErrRefreshToken 对应场景：invalid refresh token
	AuthErrRefreshToken = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "invalid refresh token",
	}
	// AuthErrUserNotFound 对应场景：user not found
	AuthErrUserNotFound = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "user not found",
	}
	// AuthErrAuthorizationTokenRequired 对应场景：authorization token is required
	AuthErrAuthorizationTokenRequired = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "authorization token is required",
	}
	// AuthErrAccessToken 对应场景：invalid access token
	AuthErrAccessToken = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "invalid access token",
	}
)
