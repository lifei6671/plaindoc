package response

import "net/http"

// 当前文件收敛 `auth.go` 的错误响应模板。
// 统一在 response 层维护 status/code/message，避免在 handler 中散落字面量。
var (
	// AuthErrRegistrationDisabled 对应场景：registration is disabled
	AuthErrRegistrationDisabled = ErrorTemplate{
		Status:  http.StatusForbidden,
		Code:    CodeRegistrationDisabled,
		Message: "注册功能已关闭",
	}
	// AuthErrRequestBody 对应场景：invalid request body
	AuthErrRequestBody = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "请求体无效",
	}
	// AuthErrEmail 对应场景：email is invalid
	AuthErrEmail = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidEmail,
		Message: "邮箱格式无效",
	}
	// AuthErrPasswordLeast6Characters 对应场景：password must be at least 6 characters
	AuthErrPasswordLeast6Characters = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidPassword,
		Message: "密码长度至少为 6 位",
	}
	// AuthErrNameRequired 对应场景：name is required
	AuthErrNameRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidName,
		Message: "名称不能为空",
	}
	// AuthErrEmailAlreadyExists 对应场景：email already exists
	AuthErrEmailAlreadyExists = ErrorTemplate{
		Status:  http.StatusConflict,
		Code:    CodeEmailAlreadyExists,
		Message: "邮箱已存在",
	}
	// AuthErrEmailPasswordRequired 对应场景：email and password are required
	AuthErrEmailPasswordRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "邮箱和密码不能为空",
	}
	// AuthErrEmailPassword 对应场景：invalid email or password
	AuthErrEmailPassword = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeInvalidCredentials,
		Message: "邮箱或密码错误",
	}
	// AuthErrUserHasBeenBanned 对应场景：user has been banned
	AuthErrUserHasBeenBanned = ErrorTemplate{
		Status:  http.StatusForbidden,
		Code:    CodeUserBanned,
		Message: "用户已被封禁",
	}
	// AuthErrUserHasBeenDeleted 对应场景：user has been deleted
	AuthErrUserHasBeenDeleted = ErrorTemplate{
		Status:  http.StatusForbidden,
		Code:    CodeUserDeleted,
		Message: "用户已被删除",
	}
	// AuthErrRefreshTokenRequired 对应场景：refresh token is required
	AuthErrRefreshTokenRequired = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "缺少刷新令牌",
	}
	// AuthErrRefreshToken 对应场景：invalid refresh token
	AuthErrRefreshToken = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "刷新令牌无效",
	}
	// AuthErrCaptchaRequired 对应场景：captcha is required.
	AuthErrCaptchaRequired = ErrorTemplate{
		Status:  http.StatusTooManyRequests,
		Code:    CodeCaptchaRequired,
		Message: "需要验证码校验",
	}
	// AuthErrCaptchaInvalid 对应场景：captcha is invalid.
	AuthErrCaptchaInvalid = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeCaptchaInvalid,
		Message: "验证码错误或已过期",
	}
	// AuthErrTemporarilyLocked 对应场景：auth operation is temporarily locked.
	AuthErrTemporarilyLocked = ErrorTemplate{
		Status:  http.StatusTooManyRequests,
		Code:    CodeAuthTemporarilyLocked,
		Message: "操作过于频繁，请稍后再试",
	}
	// AuthErrUserNotFound 对应场景：user not found
	AuthErrUserNotFound = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "用户不存在",
	}
	// AuthErrAuthorizationTokenRequired 对应场景：authorization token is required
	AuthErrAuthorizationTokenRequired = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "缺少授权令牌",
	}
	// AuthErrAccessToken 对应场景：invalid access token
	AuthErrAccessToken = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "访问令牌无效",
	}
	// AuthErrPasswordResetUnavailable 对应场景：password reset feature is unavailable.
	AuthErrPasswordResetUnavailable = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodePasswordResetUnavailable,
		Message: "密码找回功能暂不可用",
	}
	// AuthErrPasswordResetRateLimited 对应场景：password reset request rate limited.
	AuthErrPasswordResetRateLimited = ErrorTemplate{
		Status:  http.StatusTooManyRequests,
		Code:    CodePasswordResetRateLimited,
		Message: "请求过于频繁，请稍后再试",
	}
	// AuthErrPasswordResetSendFailed 对应场景：password reset mail send failed.
	AuthErrPasswordResetSendFailed = ErrorTemplate{
		Status:  http.StatusBadGateway,
		Code:    CodePasswordResetSendFailed,
		Message: "发送重置邮件失败，请稍后再试",
	}
	// AuthErrPasswordResetTokenInvalid 对应场景：password reset token invalid.
	AuthErrPasswordResetTokenInvalid = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodePasswordResetTokenInvalid,
		Message: "重置链接无效",
	}
	// AuthErrPasswordResetTokenExpired 对应场景：password reset token expired.
	AuthErrPasswordResetTokenExpired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodePasswordResetTokenExpired,
		Message: "重置链接已过期",
	}
	// AuthErrPasswordResetTokenConsumed 对应场景：password reset token consumed.
	AuthErrPasswordResetTokenConsumed = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodePasswordResetTokenConsumed,
		Message: "重置链接已使用",
	}
	// AuthErrPasswordResetUnsupported 对应场景：password reset not supported for current user.
	AuthErrPasswordResetUnsupported = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodePasswordResetUnsupported,
		Message: "当前账号不支持密码重置",
	}
	// AuthErrPasswordConfirmMismatch 对应场景：new password and confirm password mismatch.
	AuthErrPasswordConfirmMismatch = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodePasswordConfirmMismatch,
		Message: "两次输入的密码不一致",
	}
	// AuthErrPasswordUnchanged 对应场景：new password is unchanged.
	AuthErrPasswordUnchanged = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodePasswordUnchanged,
		Message: "新密码不能与旧密码相同",
	}
)
