package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

var (
	imageHostingErrInvalidSpaceID = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidSpaceID,
		Message: "spaceId is required",
	}
	imageHostingErrUploadForbidden = ErrorTemplate{
		Status:  http.StatusForbidden,
		Code:    CodeForbidden,
		Message: "insufficient space permission for image upload",
	}
	imageHostingErrProviderDisabled = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeImageHostingProviderDisabled,
		Message: "default provider is not local",
	}
	imageHostingErrUploadFileRequired = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidUploadFile,
		Message: "file is required",
	}
	imageHostingErrUploadFileEmpty = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidUploadFile,
		Message: "file is empty",
	}
	imageHostingErrUploadFileTooLarge = ErrorTemplate{
		Status:  http.StatusRequestEntityTooLarge,
		Code:    CodeFileTooLarge,
		Message: "file exceeds 20MB limit",
	}
	imageHostingErrUploadFileUnreadable = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidUploadFile,
		Message: "cannot read upload file",
	}
	imageHostingErrUploadFileNotImage = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidUploadFile,
		Message: "only image file is allowed",
	}
	imageHostingErrFileNotFound = ErrorTemplate{
		Status:  http.StatusNotFound,
		Code:    CodeFileNotFound,
		Message: "file not found",
	}
	imageHostingErrInvalidFilePath = ErrorTemplate{
		Status:  http.StatusBadRequest,
		Code:    CodeInvalidFilePath,
		Message: "invalid file path",
	}
	imageHostingErrTokenRequired = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "authorization token is required",
	}
	imageHostingErrTokenInvalid = ErrorTemplate{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: "invalid access token",
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
