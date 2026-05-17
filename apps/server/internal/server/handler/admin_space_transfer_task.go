package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/logit"
	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/errcode"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
)

type adminSpaceTransferTaskHandler struct {
	taskService *service.AdminSpaceTransferTaskService
}

type listAdminSpaceTransferTasksResponse struct {
	Tasks []service.AdminSpaceTransferTask `json:"tasks"`
}

type getAdminSpaceTransferTaskResponse struct {
	Task service.AdminSpaceTransferTask `json:"task"`
}

type issueAdminSpaceTransferStreamURLResponse struct {
	StreamURL string `json:"streamUrl"`
}

type issueAdminSpaceTransferDownloadURLResponse struct {
	DownloadURL string `json:"downloadUrl"`
}

// NewAdminSpaceTransferTaskHandler 创建全局空间导入/导出任务处理器。
func NewAdminSpaceTransferTaskHandler(taskService *service.AdminSpaceTransferTaskService) *adminSpaceTransferTaskHandler {
	return &adminSpaceTransferTaskHandler{taskService: taskService}
}

// ListMyTasks 返回当前登录用户可见的导入/导出任务。
func (h *adminSpaceTransferTaskHandler) ListMyTasks(c *gin.Context) {
	if h == nil || h.taskService == nil {
		response.InternalError(c)
		return
	}
	actorUserID, ok := adminActorUserID(c, response.AdminSpaceErrAdminActorMissing)
	if !ok {
		return
	}
	tasks, err := h.taskService.ListMyTasks(c.Request.Context(), service.ListAdminSpaceTransferTasksInput{
		ActorUserID: actorUserID,
		Status:      c.Query("status"),
		Limit:       parseAdminSpaceTransferTaskLimit(c.Query("limit")),
	})
	if err != nil {
		writeAdminSpaceTransferTaskError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, listAdminSpaceTransferTasksResponse{Tasks: tasks})
}

// GetTask 返回当前登录用户可见的单个导入/导出任务。
func (h *adminSpaceTransferTaskHandler) GetTask(c *gin.Context) {
	if h == nil || h.taskService == nil {
		response.InternalError(c)
		return
	}
	actorUserID, ok := adminActorUserID(c, response.AdminSpaceErrAdminActorMissing)
	if !ok {
		return
	}
	task, err := h.taskService.GetMyTask(c.Request.Context(), service.GetAdminSpaceTransferTaskInput{
		ActorUserID: actorUserID,
		Kind:        c.Param("kind"),
		JobID:       c.Param("jobId"),
	})
	if err != nil {
		writeAdminSpaceTransferTaskError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, getAdminSpaceTransferTaskResponse{Task: task})
}

// IssueStreamURL 为当前登录用户的活跃任务重新签发 SSE 订阅 URL。
func (h *adminSpaceTransferTaskHandler) IssueStreamURL(c *gin.Context) {
	if h == nil || h.taskService == nil {
		response.InternalError(c)
		return
	}
	actorUserID, ok := adminActorUserID(c, response.AdminSpaceErrAdminActorMissing)
	if !ok {
		return
	}
	result, err := h.taskService.IssueStreamURL(c.Request.Context(), service.IssueAdminSpaceTransferStreamInput{
		ActorUserID: actorUserID,
		Kind:        c.Param("kind"),
		JobID:       c.Param("jobId"),
	})
	if err != nil {
		writeAdminSpaceTransferTaskError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, issueAdminSpaceTransferStreamURLResponse{StreamURL: result.StreamURL})
}

// IssueDownloadURL 为当前登录用户的已完成导出任务重新签发一次性下载 URL。
func (h *adminSpaceTransferTaskHandler) IssueDownloadURL(c *gin.Context) {
	if h == nil || h.taskService == nil {
		response.InternalError(c)
		return
	}
	actorUserID, ok := adminActorUserID(c, response.AdminSpaceErrAdminActorMissing)
	if !ok {
		return
	}
	result, err := h.taskService.IssueDownloadURL(c.Request.Context(), service.IssueAdminSpaceTransferDownloadInput{
		ActorUserID: actorUserID,
		Kind:        c.Param("kind"),
		JobID:       c.Param("jobId"),
	})
	if err != nil {
		writeAdminSpaceTransferTaskError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, issueAdminSpaceTransferDownloadURLResponse{DownloadURL: result.DownloadURL})
}

func parseAdminSpaceTransferTaskLimit(rawLimit string) int {
	limit, err := strconv.Atoi(rawLimit)
	if err != nil {
		return 0
	}
	return limit
}

func writeAdminSpaceTransferTaskError(c *gin.Context, err error) {
	if errors.Is(err, errcode.ErrAdminForbidden) {
		response.Error(c, http.StatusForbidden, response.CodeForbidden, "无权查看该任务")
		return
	}
	if response.WriteMappedError(c, err,
		response.ErrorTemplateMapping{Target: errcode.ErrAdminSpaceTransferTaskNotFound, Template: response.AdminSpaceTransferTaskErrNotFound},
		response.ErrorTemplateMapping{Target: errcode.ErrAdminSpaceTransferTaskKindUnsupported, Template: response.AdminSpaceTransferTaskErrKindUnsupported},
		response.ErrorTemplateMapping{Target: errcode.ErrAdminSpaceTransferTaskStreamURLUnavailable, Template: response.AdminSpaceTransferTaskErrStreamURLUnavailable},
		response.ErrorTemplateMapping{Target: errcode.ErrAdminSpaceTransferTaskDownloadUnavailable, Template: response.AdminSpaceTransferTaskErrDownloadUnavailable},
		response.ErrorTemplateMapping{Target: errcode.ErrAdminSpaceExportJobNotFound, Template: response.AdminSpaceTransferTaskErrNotFound},
		response.ErrorTemplateMapping{Target: errcode.ErrAdminSpaceImportStagingNotFound, Template: response.AdminSpaceTransferTaskErrNotFound},
		response.ErrorTemplateMapping{Target: errcode.ErrAdminSpaceExportJobTokenInvalid, Template: response.AdminSpaceTransferTaskErrStreamURLUnavailable},
		response.ErrorTemplateMapping{Target: errcode.ErrAdminSpaceImportJobTokenInvalid, Template: response.AdminSpaceTransferTaskErrStreamURLUnavailable},
		response.ErrorTemplateMapping{Target: errcode.ErrAdminSpaceExportFileExpired, Template: response.AdminSpaceExportErrFileExpired},
		response.ErrorTemplateMapping{Target: errcode.ErrAdminSpaceExportFileNotReady, Template: response.AdminSpaceExportErrFileNotReady},
		response.ErrorTemplateMapping{Target: errcode.ErrAdminSpaceExportDownloadForbidden, Template: response.AdminSpaceExportErrDownloadForbidden},
	) {
		return
	}
	logit.SetRequestAttrs(c.Request.Context(), logit.Any("errmsg", err))
	response.InternalError(c)
}
