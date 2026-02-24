package response

import "net/http"

// 当前文件收敛 `admin_space.go` 的错误响应模板。
// 统一在 response 层维护 status/code/message，避免在 handler 中散落字面量。
var (
	// AdminSpaceErrAdminActorMissing 对应场景：admin actor is missing
	AdminSpaceErrAdminActorMissing = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "admin actor is missing",
	}
	// AdminSpaceErrRequestBody 对应场景：invalid request body
	AdminSpaceErrRequestBody = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "invalid request body",
	}
	// AdminSpaceErrFileRequired 对应场景：file is required
	AdminSpaceErrFileRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidUploadFile,
		Message: "file is required",
	}
	// AdminSpaceErrCannotReadUploadedFile 对应场景：cannot read uploaded file
	AdminSpaceErrCannotReadUploadedFile = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidUploadFile,
		Message: "cannot read uploaded file",
	}
	// AdminSpaceErrPagePositiveInteger 对应场景：page must be a positive integer
	AdminSpaceErrPagePositiveInteger = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidPage,
		Message: "page must be a positive integer",
	}
	// AdminSpaceErrPageSizePositiveInteger 对应场景：pageSize must be a positive integer
	AdminSpaceErrPageSizePositiveInteger = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidPageSize,
		Message: "pageSize must be a positive integer",
	}
	// AdminSpaceErrSpaceCategoryID 对应场景：space category id is invalid
	AdminSpaceErrSpaceCategoryID = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidSpaceCategory,
		Message: "space category id is invalid",
	}
	// AdminSpaceErrSpaceIDRequired 对应场景：space id is required
	AdminSpaceErrSpaceIDRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidSpaceID,
		Message: "space id is required",
	}
	// AdminSpaceErrMemberUserIDRequired 对应场景：member user id is required
	AdminSpaceErrMemberUserIDRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidUserID,
		Message: "member user id is required",
	}
)
