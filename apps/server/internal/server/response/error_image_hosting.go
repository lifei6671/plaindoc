package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

var (
	imageHostingErrInvalidSpaceID = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidSpaceID,
		Message: "spaceId 参数不能为空",
	}
	imageHostingErrUploadForbidden = ErrorTemplate{
		Status:  http.StatusForbidden,
		Code:    CodeForbidden,
		Message: "上传图片的空间权限不足",
	}
	imageHostingErrProviderDisabled = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeImageHostingProviderDisabled,
		Message: "默认图床提供方不是本地存储",
	}
	imageHostingErrUploadFileRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidUploadFile,
		Message: "文件不能为空",
	}
	imageHostingErrUploadFileEmpty = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidUploadFile,
		Message: "文件为空",
	}
	imageHostingErrUploadFileTooLarge = ErrorTemplate{
		Status:  http.StatusRequestEntityTooLarge,
		Code:    CodeFileTooLarge,
		Message: "文件超过 20MB 限制",
	}
	imageHostingErrUploadFileUnreadable = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidUploadFile,
		Message: "无法读取上传文件",
	}
	imageHostingErrUploadFileNotImage = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidUploadFile,
		Message: "仅允许图片文件",
	}
	imageHostingErrFileNotFound = ErrorTemplate{
		Status:  http.StatusNotFound,
		Code:    CodeFileNotFound,
		Message: "文件不存在",
	}
	imageHostingErrInvalidFilePath = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidFilePath,
		Message: "文件路径无效",
	}
	imageHostingErrTokenRequired = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "缺少授权令牌",
	}
	imageHostingErrTokenInvalid = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "访问令牌无效",
	}
)

func ImageHostingInvalidSpaceID(c *gin.Context) {
	imageHostingErrInvalidSpaceID.Write(c)
}

func ImageHostingUploadForbidden(c *gin.Context) {
	imageHostingErrUploadForbidden.Write(c)
}

func ImageHostingProviderDisabled(c *gin.Context) {
	imageHostingErrProviderDisabled.Write(c)
}

func ImageHostingUploadFileRequired(c *gin.Context) {
	imageHostingErrUploadFileRequired.Write(c)
}

func ImageHostingUploadFileEmpty(c *gin.Context) {
	imageHostingErrUploadFileEmpty.Write(c)
}

func ImageHostingUploadFileTooLarge(c *gin.Context) {
	imageHostingErrUploadFileTooLarge.Write(c)
}

func ImageHostingUploadFileUnreadable(c *gin.Context) {
	imageHostingErrUploadFileUnreadable.Write(c)
}

func ImageHostingUploadFileNotImage(c *gin.Context) {
	imageHostingErrUploadFileNotImage.Write(c)
}

func ImageHostingFileNotFound(c *gin.Context) {
	imageHostingErrFileNotFound.Write(c)
}

func ImageHostingInvalidFilePath(c *gin.Context) {
	imageHostingErrInvalidFilePath.Write(c)
}

func ImageHostingTokenRequired(c *gin.Context) {
	imageHostingErrTokenRequired.Write(c)
}

func ImageHostingTokenInvalid(c *gin.Context) {
	imageHostingErrTokenInvalid.Write(c)
}
