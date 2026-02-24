package response

import "net/http"

// 当前文件收敛 `access.go` 的错误响应模板。
// 统一在 response 层维护 status/code/message，避免在 handler 中散落字面量。
var (
	// AccessErrSpaceIDRequired 对应场景：space id is required
	AccessErrSpaceIDRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidSpaceID,
		Message: "space id is required",
	}
	// AccessErrAccessToken 对应场景：invalid access token
	AccessErrAccessToken = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "invalid access token",
	}
	// AccessErrSpaceNotFound 对应场景：space not found
	AccessErrSpaceNotFound = ErrorTemplate{
		Status:  http.StatusNotFound,
		Code:    CodeSpaceNotFound,
		Message: "space not found",
	}
	// AccessErrLoginRequired 对应场景：login required
	AccessErrLoginRequired = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "login required",
	}
	// AccessErrInsufficientSpacePermission 对应场景：insufficient space permission
	AccessErrInsufficientSpacePermission = ErrorTemplate{
		Status:  http.StatusForbidden,
		Code:    CodeForbidden,
		Message: "insufficient space permission",
	}
	// AccessErrVisibilityRequired 对应场景：visibility is required
	AccessErrVisibilityRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "visibility is required",
	}
	// AccessErrVisibilityPublicAuthenticatedMember 对应场景：visibility must be one of public/authenticated/member
	AccessErrVisibilityPublicAuthenticatedMember = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidVisibility,
		Message: "visibility must be one of public/authenticated/member",
	}
	// AccessErrOnlyOwnerCanUpdateSpaceVisibility 对应场景：only owner can update space visibility
	AccessErrOnlyOwnerCanUpdateSpaceVisibility = ErrorTemplate{
		Status:  http.StatusForbidden,
		Code:    CodeForbidden,
		Message: "only owner can update space visibility",
	}
	// AccessErrDocumentIDRequired 对应场景：document id is required
	AccessErrDocumentIDRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidDocumentID,
		Message: "document id is required",
	}
	// AccessErrDocumentNotFound 对应场景：document not found
	AccessErrDocumentNotFound = ErrorTemplate{
		Status:  http.StatusNotFound,
		Code:    CodeDocumentNotFound,
		Message: "document not found",
	}
	// AccessErrInsufficientDocumentPermission 对应场景：insufficient document permission
	AccessErrInsufficientDocumentPermission = ErrorTemplate{
		Status:  http.StatusForbidden,
		Code:    CodeForbidden,
		Message: "insufficient document permission",
	}
	// AccessErrAuthorizationTokenRequired 对应场景：authorization token is required
	AccessErrAuthorizationTokenRequired = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "authorization token is required",
	}
)
