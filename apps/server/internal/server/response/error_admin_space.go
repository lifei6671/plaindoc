package response

import "net/http"

// 当前文件收敛 `admin_space.go` 的错误响应模板。
// 统一在 response 层维护 status/code/message，避免在 handler 中散落字面量。
var (
	// AdminSpaceErrAdminActorMissing 对应场景：admin actor is missing
	AdminSpaceErrAdminActorMissing = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "缺少管理员身份",
	}
	// AdminSpaceErrRequestBody 对应场景：invalid request body
	AdminSpaceErrRequestBody = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "请求体无效",
	}
	// AdminSpaceErrFileRequired 对应场景：file is required
	AdminSpaceErrFileRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidUploadFile,
		Message: "文件不能为空",
	}
	// AdminSpaceErrCannotReadUploadedFile 对应场景：cannot read uploaded file
	AdminSpaceErrCannotReadUploadedFile = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidUploadFile,
		Message: "无法读取上传文件",
	}
	// AdminSpaceErrPagePositiveInteger 对应场景：page must be a positive integer
	AdminSpaceErrPagePositiveInteger = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidPage,
		Message: "页码参数（page）必须是正整数",
	}
	// AdminSpaceErrPageSizePositiveInteger 对应场景：pageSize must be a positive integer
	AdminSpaceErrPageSizePositiveInteger = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidPageSize,
		Message: "每页数量参数（pageSize）必须是正整数",
	}
	// AdminSpaceErrSpaceCategoryID 对应场景：space category id is invalid
	AdminSpaceErrSpaceCategoryID = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidSpaceCategory,
		Message: "空间分类 ID 无效",
	}
	// AdminSpaceErrSpaceIDRequired 对应场景：space id is required
	AdminSpaceErrSpaceIDRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidSpaceID,
		Message: "空间 ID 不能为空",
	}
	// AdminSpaceErrMemberUserIDRequired 对应场景：member user id is required
	AdminSpaceErrMemberUserIDRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidUserID,
		Message: "成员用户 ID 不能为空",
	}
)
