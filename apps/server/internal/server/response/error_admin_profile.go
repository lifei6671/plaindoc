package response

import "net/http"

// 当前文件收敛 `admin_profile.go` 的错误响应模板。
// 统一在 response 层维护 status/code/message，避免在 handler 中散落字面量。
var (
	// AdminProfileErrAdminActorMissing 对应场景：admin actor is missing
	AdminProfileErrAdminActorMissing = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "admin actor is missing",
	}
	// AdminProfileErrRequestBody 对应场景：invalid request body
	AdminProfileErrRequestBody = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "invalid request body",
	}
	// AdminProfileErrNewPasswordConfirmPasswordMismatch 对应场景：new password and confirm password mismatch
	AdminProfileErrNewPasswordConfirmPasswordMismatch = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodePasswordConfirmMismatch,
		Message: "new password and confirm password mismatch",
	}
	// AdminProfileErrFileRequired 对应场景：file is required
	AdminProfileErrFileRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidUploadFile,
		Message: "file is required",
	}
	// AdminProfileErrFileEmpty 对应场景：file is empty
	AdminProfileErrFileEmpty = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidUploadFile,
		Message: "file is empty",
	}
	// AdminProfileErrAvatarFileExceeds10mbLimit 对应场景：avatar file exceeds 10MB limit
	AdminProfileErrAvatarFileExceeds10mbLimit = ErrorTemplate{
		Status:  http.StatusRequestEntityTooLarge,
		Code:    CodeFileTooLarge,
		Message: "avatar file exceeds 10MB limit",
	}
	// AdminProfileErrCannotReadUploadedFile 对应场景：cannot read uploaded file
	AdminProfileErrCannotReadUploadedFile = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidUploadFile,
		Message: "cannot read uploaded file",
	}
	// AdminProfileErrOnlyImageFileAllowed 对应场景：only image file is allowed
	AdminProfileErrOnlyImageFileAllowed = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidUploadFile,
		Message: "only image file is allowed",
	}
)
