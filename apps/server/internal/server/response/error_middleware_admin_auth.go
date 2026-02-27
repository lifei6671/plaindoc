package response

import "net/http"

// 当前文件收敛 `middleware/admin_auth.go` 的错误响应模板。
// 统一在 response 层维护 status/code/message，保证鉴权失败语义一致。
var (
	// MiddlewareAdminAuthErrAuthorizationTokenRequired 对应场景：authorization token is required
	MiddlewareAdminAuthErrAuthorizationTokenRequired = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "缺少授权令牌",
	}
	// MiddlewareAdminAuthErrInvalidAccessToken 对应场景：invalid access token
	MiddlewareAdminAuthErrInvalidAccessToken = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "访问令牌无效",
	}
	// MiddlewareAdminAuthErrAdminRoleRequired 对应场景：admin role is required
	MiddlewareAdminAuthErrAdminRoleRequired = ErrorTemplate{
		Status:  http.StatusForbidden,
		Code:    CodeForbidden,
		Message: "需要管理员角色",
	}
	// MiddlewareAdminAuthErrAdminActorMissing 对应场景：admin actor is missing
	MiddlewareAdminAuthErrAdminActorMissing = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "缺少管理员身份",
	}
	// MiddlewareAdminAuthErrAdminActorInvalid 对应场景：admin actor is invalid
	MiddlewareAdminAuthErrAdminActorInvalid = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "管理员身份无效",
	}
	// MiddlewareAdminAuthErrSpaceIDRequired 对应场景：space id is required
	MiddlewareAdminAuthErrSpaceIDRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidSpaceID,
		Message: "空间 ID 不能为空",
	}
	// MiddlewareAdminAuthErrInsufficientSpaceAdminPermission 对应场景：insufficient space admin permission
	MiddlewareAdminAuthErrInsufficientSpaceAdminPermission = ErrorTemplate{
		Status:  http.StatusForbidden,
		Code:    CodeForbidden,
		Message: "空间管理权限不足",
	}
	// MiddlewareAdminAuthErrPlatformAdminRoleRequired 对应场景：platform admin role is required
	MiddlewareAdminAuthErrPlatformAdminRoleRequired = ErrorTemplate{
		Status:  http.StatusForbidden,
		Code:    CodeForbidden,
		Message: "需要平台管理员角色",
	}
)
