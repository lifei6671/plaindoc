package response

import "net/http"

// 当前文件收敛 `admin_document_image_asset.go` 的错误响应模板。
var (
	// AdminDocumentImageAssetErrAdminActorMissing 对应场景：admin actor is missing
	AdminDocumentImageAssetErrAdminActorMissing = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "缺少管理员身份",
	}
	// AdminDocumentImageAssetErrPagePositiveInteger 对应场景：page must be a positive integer
	AdminDocumentImageAssetErrPagePositiveInteger = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidPage,
		Message: "页码参数（page）必须是正整数",
	}
	// AdminDocumentImageAssetErrPageSizePositiveInteger 对应场景：pageSize must be a positive integer
	AdminDocumentImageAssetErrPageSizePositiveInteger = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidPageSize,
		Message: "每页数量参数（pageSize）必须是正整数",
	}
	// AdminDocumentImageAssetErrImageAssetIDRequired 对应场景：image asset id is required
	AdminDocumentImageAssetErrImageAssetIDRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidRequest,
		Message: "图片资源 ID 不能为空",
	}
)
