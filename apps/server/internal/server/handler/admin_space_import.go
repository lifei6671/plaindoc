package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/logit"
	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/errcode"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
)

type adminSpaceImportHandler struct {
	importService *service.AdminSpaceImportService
}

var (
	adminSpaceImportMultipartMemory int64 = 32 << 20
	adminSpaceImportMaxRequestBytes       = service.MaxAdminSpaceImportUploadBytes + adminSpaceImportMultipartMemory
)

type commitAdminSpaceImportRequest struct {
	SpaceName  string `json:"spaceName"`
	SpaceID    string `json:"spaceId"`
	CategoryID string `json:"categoryId"`
	Visibility string `json:"visibility"`
}

type adminSpaceImportInspectResponse struct {
	ImportID       string                                  `json:"importId"`
	PackageVersion int                                     `json:"packageVersion"`
	PackageType    string                                  `json:"packageType"`
	ExportedAt     string                                  `json:"exportedAt"`
	Importable     bool                                    `json:"importable"`
	Space          service.AdminSpaceImportPreviewSpace    `json:"space"`
	Summary        service.AdminSpaceExportManifestSummary `json:"summary"`
	Warnings       []string                                `json:"warnings"`
}

type commitAdminSpaceImportResponse struct {
	JobID     string `json:"jobId"`
	StreamURL string `json:"streamUrl"`
}

// NewAdminSpaceImportHandler 创建后台空间导入处理器。
func NewAdminSpaceImportHandler(importService *service.AdminSpaceImportService) *adminSpaceImportHandler {
	return &adminSpaceImportHandler{importService: importService}
}

// Inspect 上传并解析空间导入包。
func (h *adminSpaceImportHandler) Inspect(c *gin.Context) {
	if h == nil || h.importService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, ok := adminActorUserID(c, response.AdminSpaceErrAdminActorMissing)
	if !ok {
		return
	}

	multipartMemory := adminSpaceImportMultipartMemory
	if multipartMemory <= 0 {
		multipartMemory = 32 << 20
	}
	maxRequestBytes := adminSpaceImportMaxRequestBytes
	if maxRequestBytes <= 0 {
		maxRequestBytes = service.MaxAdminSpaceImportUploadBytes + multipartMemory
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBytes)
	if err := c.Request.ParseMultipartForm(multipartMemory); err != nil {
		logit.SetRequestAttrs(c.Request.Context(), logit.Any("errmsg", err))
		writeAdminSpaceImportError(c, errcode.ErrAdminSpaceImportZipInvalid)
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		writeAdminSpaceImportError(c, errcode.ErrAdminSpaceImportFileRequired)
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		logit.SetRequestAttrs(c.Request.Context(), logit.Any("errmsg", err))
		response.AdminSpaceErrCannotReadUploadedFile.Write(c)
		return
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			logit.SetRequestAttrs(c.Request.Context(), logit.Any("errmsg", closeErr))
		}
	}()

	result, inspectErr := h.importService.Inspect(c.Request.Context(), service.InspectAdminSpaceImportInput{
		ActorUserID: actorUserID,
		FileName:    fileHeader.Filename,
		ContentType: fileHeader.Header.Get("Content-Type"),
		SizeBytes:   fileHeader.Size,
		Reader:      file,
	})
	if inspectErr != nil {
		writeAdminSpaceImportError(c, inspectErr)
		return
	}

	response.JSON(c, http.StatusOK, adminSpaceImportInspectResponse{
		ImportID:       result.ImportID,
		PackageVersion: result.PackageVersion,
		PackageType:    result.PackageType,
		ExportedAt:     result.ExportedAt,
		Importable:     result.Importable,
		Space:          result.Space,
		Summary:        result.Summary,
		Warnings:       result.Warnings,
	})
}

// Commit 创建空间导入任务。
func (h *adminSpaceImportHandler) Commit(c *gin.Context) {
	if h == nil || h.importService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, ok := adminActorUserID(c, response.AdminSpaceErrAdminActorMissing)
	if !ok {
		return
	}

	var req commitAdminSpaceImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logit.SetRequestAttrs(c.Request.Context(), logit.Any("errmsg", err))
		response.AdminSpaceImportErrPackageNotImportable.Write(c)
		return
	}

	result, err := h.importService.Commit(c.Request.Context(), service.CommitAdminSpaceImportInput{
		ActorUserID: actorUserID,
		ImportID:    c.Param("importId"),
		SpaceName:   req.SpaceName,
		SpaceID:     req.SpaceID,
		CategoryID:  req.CategoryID,
		Visibility:  req.Visibility,
	})
	if err != nil {
		writeAdminSpaceImportError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, commitAdminSpaceImportResponse{
		JobID:     result.JobID,
		StreamURL: result.StreamURL,
	})
}

// StreamEvents 是导入任务 SSE 入口；任务事件 Store 在 Phase 2 补齐。
func (h *adminSpaceImportHandler) StreamEvents(c *gin.Context) {
	if h == nil || h.importService == nil {
		response.InternalError(c)
		return
	}
	actorUserID, ok := adminActorUserID(c, response.AdminSpaceErrAdminActorMissing)
	if !ok {
		return
	}
	initial, events, unsubscribe, err := h.importService.Subscribe(
		c.Request.Context(),
		c.Param("jobId"),
		actorUserID,
		c.Query("token"),
	)
	if err != nil {
		writeAdminSpaceImportError(c, err)
		return
	}
	defer unsubscribe()
	writeAdminSpaceTransferSSE(c, initial, events)
}

func writeAdminSpaceImportError(c *gin.Context, err error) {
	if errors.Is(err, errcode.ErrAdminForbidden) {
		response.Error(c, http.StatusForbidden, response.CodeForbidden, "无权导入空间")
		return
	}
	if response.WriteMappedError(c, err,
		response.ErrorTemplateMapping{Target: errcode.ErrAdminSpaceImportFileRequired, Template: response.AdminSpaceImportErrFileRequired},
		response.ErrorTemplateMapping{Target: errcode.ErrAdminSpaceImportZipInvalid, Template: response.AdminSpaceImportErrZipInvalid},
		response.ErrorTemplateMapping{Target: errcode.ErrAdminSpaceImportManifestMissing, Template: response.AdminSpaceImportErrManifestMissing},
		response.ErrorTemplateMapping{Target: errcode.ErrAdminSpaceImportTreeMissing, Template: response.AdminSpaceImportErrTreeMissing},
		response.ErrorTemplateMapping{Target: errcode.ErrAdminSpaceImportPackageUnsupported, Template: response.AdminSpaceImportErrPackageUnsupported},
		response.ErrorTemplateMapping{Target: errcode.ErrAdminSpaceImportPackageNotImportable, Template: response.AdminSpaceImportErrPackageNotImportable},
		response.ErrorTemplateMapping{Target: errcode.ErrAdminSpaceImportStagingNotFound, Template: response.AdminSpaceImportErrStagingNotFound},
		response.ErrorTemplateMapping{Target: errcode.ErrAdminSpaceImportStagingExpired, Template: response.AdminSpaceImportErrStagingExpired},
		response.ErrorTemplateMapping{Target: errcode.ErrAdminSpaceImportJobRunningLimit, Template: response.AdminSpaceImportErrJobRunningLimit},
		response.ErrorTemplateMapping{Target: errcode.ErrAdminSpaceImportJobTokenInvalid, Template: response.AdminSpaceImportErrJobTokenInvalid},
		response.ErrorTemplateMapping{Target: errcode.ErrAdminSpaceImportCommitForbidden, Template: response.AdminSpaceImportErrCommitForbidden},
	) {
		return
	}
	logit.SetRequestAttrs(c.Request.Context(), logit.Any("errmsg", err))
	response.InternalError(c)
}
