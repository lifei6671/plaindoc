package response

import "net/http"

// 当前文件收敛 `middleware/admin_auth.go` 的错误响应模板。
// 统一在 response 层维护 status/code/message，保证鉴权失败语义一致。
var (
	// MiddlewareAdminAuthErrAuthorizationTokenRequired 对应场景：authorization token is required
	MiddlewareAdminAuthErrAuthorizationTokenRequired = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "authorization token is required",
	}
	// MiddlewareAdminAuthErrInvalidAccessToken 对应场景：invalid access token
	MiddlewareAdminAuthErrInvalidAccessToken = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "invalid access token",
	}
	// MiddlewareAdminAuthErrAdminRoleRequired 对应场景：admin role is required
	MiddlewareAdminAuthErrAdminRoleRequired = ErrorTemplate{
		Status:  http.StatusForbidden,
		Code:    CodeForbidden,
		Message: "admin role is required",
	}
	// MiddlewareAdminAuthErrAdminActorMissing 对应场景：admin actor is missing
	MiddlewareAdminAuthErrAdminActorMissing = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "admin actor is missing",
	}
	// MiddlewareAdminAuthErrAdminActorInvalid 对应场景：admin actor is invalid
	MiddlewareAdminAuthErrAdminActorInvalid = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "admin actor is invalid",
	}
	// MiddlewareAdminAuthErrSpaceIDRequired 对应场景：space id is required
	MiddlewareAdminAuthErrSpaceIDRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidSpaceID,
		Message: "space id is required",
	}
	// MiddlewareAdminAuthErrInsufficientSpaceAdminPermission 对应场景：insufficient space admin permission
	MiddlewareAdminAuthErrInsufficientSpaceAdminPermission = ErrorTemplate{
		Status:  http.StatusForbidden,
		Code:    CodeForbidden,
		Message: "insufficient space admin permission",
	}
	// MiddlewareAdminAuthErrPlatformAdminRoleRequired 对应场景：platform admin role is required
	MiddlewareAdminAuthErrPlatformAdminRoleRequired = ErrorTemplate{
		Status:  http.StatusForbidden,
		Code:    CodeForbidden,
		Message: "platform admin role is required",
	}
)
