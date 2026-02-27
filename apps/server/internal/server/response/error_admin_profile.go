package response

import "net/http"

// 当前文件收敛 `admin_profile.go` 的错误响应模板。
// 统一在 response 层维护 status/code/message，避免在 handler 中散落字面量。
var (
	// AdminProfileErrAdminActorMissing 对应场景：admin actor is missing
	AdminProfileErrAdminActorMissing = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "缺少管理员身份",
	}
	// AdminProfileErrRequestBody 对应场景：invalid request body
	AdminProfileErrRequestBody = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "请求体无效",
	}
	// AdminProfileErrNewPasswordConfirmPasswordMismatch 对应场景：new password and confirm password mismatch
	AdminProfileErrNewPasswordConfirmPasswordMismatch = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodePasswordConfirmMismatch,
		Message: "新密码与确认密码不一致",
	}
	// AdminProfileErrFileRequired 对应场景：file is required
	AdminProfileErrFileRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidUploadFile,
		Message: "文件不能为空",
	}
	// AdminProfileErrFileEmpty 对应场景：file is empty
	AdminProfileErrFileEmpty = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidUploadFile,
		Message: "文件为空",
	}
	// AdminProfileErrAvatarFileExceeds10mbLimit 对应场景：avatar file exceeds 10MB limit
	AdminProfileErrAvatarFileExceeds10mbLimit = ErrorTemplate{
		Status:  http.StatusRequestEntityTooLarge,
		Code:    CodeFileTooLarge,
		Message: "头像文件超过 10MB 限制",
	}
	// AdminProfileErrCannotReadUploadedFile 对应场景：cannot read uploaded file
	AdminProfileErrCannotReadUploadedFile = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidUploadFile,
		Message: "无法读取上传文件",
	}
	// AdminProfileErrOnlyImageFileAllowed 对应场景：only image file is allowed
	AdminProfileErrOnlyImageFileAllowed = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidUploadFile,
		Message: "仅允许图片文件",
	}
)
