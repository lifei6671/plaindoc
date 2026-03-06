package response

import "net/http"

// 当前文件收敛 `workspace.go` 的错误响应模板。
// 统一在 response 层维护 status/code/message，避免在 handler 中散落字面量。
var (
	// WorkspaceErrSpaceNameRequired 对应场景：space name is required
	WorkspaceErrSpaceNameRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "空间名称不能为空",
	}
	// WorkspaceErrSpaceName 对应场景：invalid space name
	WorkspaceErrSpaceName = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidSpaceName,
		Message: "空间名称无效",
	}
	// WorkspaceErrSpaceIDRequired 对应场景：space id is required
	WorkspaceErrSpaceIDRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidSpaceID,
		Message: "空间 ID 不能为空",
	}
	// WorkspaceErrSpaceNotFound 对应场景：space not found
	WorkspaceErrSpaceNotFound = ErrorTemplate{
		Status:  http.StatusNotFound,
		Code:    CodeSpaceNotFound,
		Message: "空间不存在",
	}
	// WorkspaceErrLoginRequired 对应场景：login required
	WorkspaceErrLoginRequired = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "需要登录",
	}
	// WorkspaceErrInsufficientSpacePermission 对应场景：insufficient space permission
	WorkspaceErrInsufficientSpacePermission = ErrorTemplate{
		Status:  http.StatusForbidden,
		Code:    CodeForbidden,
		Message: "空间权限不足",
	}
	// WorkspaceErrCreateNodeRequest 对应场景：invalid create node request
	WorkspaceErrCreateNodeRequest = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "创建节点请求无效",
	}
	// WorkspaceErrNodeType 对应场景：invalid node type
	WorkspaceErrNodeType = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "节点类型无效",
	}
	// WorkspaceErrParentNodeNotFound 对应场景：parent node not found
	WorkspaceErrParentNodeNotFound = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "父节点不存在",
	}
	// WorkspaceErrParentNodeNotTargetSpace 对应场景：parent node not in target space
	WorkspaceErrParentNodeNotTargetSpace = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "父节点不在目标空间内",
	}
	// WorkspaceErrNodeIDRequired 对应场景：node id is required
	WorkspaceErrNodeIDRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidNodeID,
		Message: "节点 ID 不能为空",
	}
	// WorkspaceErrUpdateNodeRequest 对应场景：invalid update node request
	WorkspaceErrUpdateNodeRequest = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "更新节点请求无效",
	}
	// WorkspaceErrMoveNodeRequest 对应场景：invalid move node request
	WorkspaceErrMoveNodeRequest = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "移动节点请求无效",
	}
	// WorkspaceErrNodeNotFound 对应场景：node not found
	WorkspaceErrNodeNotFound = ErrorTemplate{
		Status:  http.StatusNotFound,
		Code:    CodeNodeNotFound,
		Message: "节点不存在",
	}
	// WorkspaceErrNodeCannotItsOwnParent 对应场景：node cannot be its own parent
	WorkspaceErrNodeCannotItsOwnParent = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "节点不能设置为自身的父节点",
	}
	// WorkspaceErrNodeMoveCycleDetected 对应场景：node cannot move to its own descendant
	WorkspaceErrNodeMoveCycleDetected = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidNodeMove,
		Message: "节点不能移动到其子孙节点下",
	}
	// WorkspaceErrDocumentIDRequired 对应场景：document id is required
	WorkspaceErrDocumentIDRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidDocumentID,
		Message: "文档 ID 不能为空",
	}
	// WorkspaceErrDocumentFormatInvalid 对应场景：invalid document format
	WorkspaceErrDocumentFormatInvalid = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "文档格式无效",
	}
	// WorkspaceErrOnlyOfficeDisabled 对应场景：ONLYOFFICE disabled when creating office document
	WorkspaceErrOnlyOfficeDisabled = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "ONLYOFFICE 未启用，当前不可创建 Office 文档",
	}
	// WorkspaceErrSaveDocumentRequest 对应场景：invalid save document request
	WorkspaceErrSaveDocumentRequest = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "保存文档请求无效",
	}
	// WorkspaceErrLocalizeRemoteImagesRequest 对应场景：invalid localize remote images request
	WorkspaceErrLocalizeRemoteImagesRequest = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "转储外链图片请求无效",
	}
	// WorkspaceErrBaseversionRequired 对应场景：baseVersion is required
	WorkspaceErrBaseversionRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "baseVersion 参数不能为空",
	}
	// WorkspaceErrMarkdownOnlyOperation 对应场景：operation only supports markdown document
	WorkspaceErrMarkdownOnlyOperation = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "当前文档格式不支持该操作",
	}
	// WorkspaceErrDocumentNotFound 对应场景：document not found
	WorkspaceErrDocumentNotFound = ErrorTemplate{
		Status:  http.StatusNotFound,
		Code:    CodeDocumentNotFound,
		Message: "文档不存在",
	}
	// WorkspaceErrInsufficientDocumentPermission 对应场景：insufficient document permission
	WorkspaceErrInsufficientDocumentPermission = ErrorTemplate{
		Status:  http.StatusForbidden,
		Code:    CodeForbidden,
		Message: "文档权限不足",
	}
	// WorkspaceErrAuthorizationTokenRequired 对应场景：authorization token is required
	WorkspaceErrAuthorizationTokenRequired = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "缺少授权令牌",
	}
	// WorkspaceErrAccessToken 对应场景：invalid access token
	WorkspaceErrAccessToken = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "访问令牌无效",
	}
	// WorkspaceErrDocumentIdentifierInvalid 对应场景：invalid document identifier
	WorkspaceErrDocumentIdentifierInvalid = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidDocumentIdentifier,
		Message: "文档标识无效",
	}
	// WorkspaceErrDocumentIdentifierReserved 对应场景：reserved document identifier
	WorkspaceErrDocumentIdentifierReserved = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeDocumentIdentifierReserved,
		Message: "文档标识为保留词",
	}
	// WorkspaceErrDocumentIdentifierConflict 对应场景：duplicate document identifier in same space
	WorkspaceErrDocumentIdentifierConflict = ErrorTemplate{
		Status:  http.StatusConflict,
		Code:    CodeDocumentIdentifierConflict,
		Message: "文档标识已存在",
	}
)
