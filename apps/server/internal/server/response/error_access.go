package response

import "net/http"

// 当前文件收敛 `access.go` 的错误响应模板。
// 统一在 response 层维护 status/code/message，避免在 handler 中散落字面量。
var (
	// AccessErrSpaceIDRequired 对应场景：space id is required
	AccessErrSpaceIDRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidSpaceID,
		Message: "空间 ID 不能为空",
	}
	// AccessErrAccessToken 对应场景：invalid access token
	AccessErrAccessToken = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "访问令牌无效",
	}
	// AccessErrSpaceNotFound 对应场景：space not found
	AccessErrSpaceNotFound = ErrorTemplate{
		Status:  http.StatusNotFound,
		Code:    CodeSpaceNotFound,
		Message: "空间不存在",
	}
	// AccessErrLoginRequired 对应场景：login required
	AccessErrLoginRequired = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "需要登录",
	}
	// AccessErrInsufficientSpacePermission 对应场景：insufficient space permission
	AccessErrInsufficientSpacePermission = ErrorTemplate{
		Status:  http.StatusForbidden,
		Code:    CodeForbidden,
		Message: "空间权限不足",
	}
	// AccessErrVisibilityRequired 对应场景：visibility is required
	AccessErrVisibilityRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "可见性不能为空",
	}
	// AccessErrVisibilityPublicAuthenticatedMember 对应场景：visibility must be one of public/authenticated/member
	AccessErrVisibilityPublicAuthenticatedMember = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidVisibility,
		Message: "可见性必须是 public/authenticated/member 之一",
	}
	// AccessErrOnlyOwnerCanUpdateSpaceVisibility 对应场景：only owner can update space visibility
	AccessErrOnlyOwnerCanUpdateSpaceVisibility = ErrorTemplate{
		Status:  http.StatusForbidden,
		Code:    CodeForbidden,
		Message: "仅空间所有者可以修改空间可见性",
	}
	// AccessErrDocumentIDRequired 对应场景：document id is required
	AccessErrDocumentIDRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidDocumentID,
		Message: "文档 ID 不能为空",
	}
	// AccessErrDocumentNotFound 对应场景：document not found
	AccessErrDocumentNotFound = ErrorTemplate{
		Status:  http.StatusNotFound,
		Code:    CodeDocumentNotFound,
		Message: "文档不存在",
	}
	// AccessErrInsufficientDocumentPermission 对应场景：insufficient document permission
	AccessErrInsufficientDocumentPermission = ErrorTemplate{
		Status:  http.StatusForbidden,
		Code:    CodeForbidden,
		Message: "文档权限不足",
	}
	// AccessErrAuthorizationTokenRequired 对应场景：authorization token is required
	AccessErrAuthorizationTokenRequired = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "缺少授权令牌",
	}
)
