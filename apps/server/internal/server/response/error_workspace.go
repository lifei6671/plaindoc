package response

import "net/http"

// 当前文件收敛 `workspace.go` 的错误响应模板。
// 统一在 response 层维护 status/code/message，避免在 handler 中散落字面量。
var (
	// WorkspaceErrSpaceNameRequired 对应场景：space name is required
	WorkspaceErrSpaceNameRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "space name is required",
	}
	// WorkspaceErrSpaceName 对应场景：invalid space name
	WorkspaceErrSpaceName = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidSpaceName,
		Message: "invalid space name",
	}
	// WorkspaceErrSpaceIDRequired 对应场景：space id is required
	WorkspaceErrSpaceIDRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidSpaceID,
		Message: "space id is required",
	}
	// WorkspaceErrSpaceNotFound 对应场景：space not found
	WorkspaceErrSpaceNotFound = ErrorTemplate{
		Status:  http.StatusNotFound,
		Code:    CodeSpaceNotFound,
		Message: "space not found",
	}
	// WorkspaceErrLoginRequired 对应场景：login required
	WorkspaceErrLoginRequired = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "login required",
	}
	// WorkspaceErrInsufficientSpacePermission 对应场景：insufficient space permission
	WorkspaceErrInsufficientSpacePermission = ErrorTemplate{
		Status:  http.StatusForbidden,
		Code:    CodeForbidden,
		Message: "insufficient space permission",
	}
	// WorkspaceErrCreateNodeRequest 对应场景：invalid create node request
	WorkspaceErrCreateNodeRequest = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "invalid create node request",
	}
	// WorkspaceErrNodeType 对应场景：invalid node type
	WorkspaceErrNodeType = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "invalid node type",
	}
	// WorkspaceErrParentNodeNotFound 对应场景：parent node not found
	WorkspaceErrParentNodeNotFound = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "parent node not found",
	}
	// WorkspaceErrParentNodeNotTargetSpace 对应场景：parent node not in target space
	WorkspaceErrParentNodeNotTargetSpace = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "parent node not in target space",
	}
	// WorkspaceErrNodeIDRequired 对应场景：node id is required
	WorkspaceErrNodeIDRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidNodeID,
		Message: "node id is required",
	}
	// WorkspaceErrUpdateNodeRequest 对应场景：invalid update node request
	WorkspaceErrUpdateNodeRequest = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "invalid update node request",
	}
	// WorkspaceErrMoveNodeRequest 对应场景：invalid move node request
	WorkspaceErrMoveNodeRequest = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "invalid move node request",
	}
	// WorkspaceErrNodeNotFound 对应场景：node not found
	WorkspaceErrNodeNotFound = ErrorTemplate{
		Status:  http.StatusNotFound,
		Code:    CodeNodeNotFound,
		Message: "node not found",
	}
	// WorkspaceErrNodeCannotItsOwnParent 对应场景：node cannot be its own parent
	WorkspaceErrNodeCannotItsOwnParent = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "node cannot be its own parent",
	}
	// WorkspaceErrNodeMoveCycleDetected 对应场景：node cannot move to its own descendant
	WorkspaceErrNodeMoveCycleDetected = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidNodeMove,
		Message: "node cannot move to its own descendant",
	}
	// WorkspaceErrDocumentIDRequired 对应场景：document id is required
	WorkspaceErrDocumentIDRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidDocumentID,
		Message: "document id is required",
	}
	// WorkspaceErrSaveDocumentRequest 对应场景：invalid save document request
	WorkspaceErrSaveDocumentRequest = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "invalid save document request",
	}
	// WorkspaceErrLocalizeRemoteImagesRequest 对应场景：invalid localize remote images request
	WorkspaceErrLocalizeRemoteImagesRequest = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "invalid localize remote images request",
	}
	// WorkspaceErrBaseversionRequired 对应场景：baseVersion is required
	WorkspaceErrBaseversionRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "baseVersion is required",
	}
	// WorkspaceErrDocumentNotFound 对应场景：document not found
	WorkspaceErrDocumentNotFound = ErrorTemplate{
		Status:  http.StatusNotFound,
		Code:    CodeDocumentNotFound,
		Message: "document not found",
	}
	// WorkspaceErrInsufficientDocumentPermission 对应场景：insufficient document permission
	WorkspaceErrInsufficientDocumentPermission = ErrorTemplate{
		Status:  http.StatusForbidden,
		Code:    CodeForbidden,
		Message: "insufficient document permission",
	}
	// WorkspaceErrAuthorizationTokenRequired 对应场景：authorization token is required
	WorkspaceErrAuthorizationTokenRequired = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "authorization token is required",
	}
	// WorkspaceErrAccessToken 对应场景：invalid access token
	WorkspaceErrAccessToken = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "invalid access token",
	}
)
