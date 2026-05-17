package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/logit"
	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/errcode"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/middleware"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
)

type adminSpaceExportHandler struct {
	exportService *service.AdminSpaceExportService
}

type startAdminSpaceExportRequest struct {
	Format               service.AdminSpaceExportFormat `json:"format"`
	IncludeAttachments   bool                           `json:"includeAttachments"`
	IncludeOfficeSources bool                           `json:"includeOfficeSources"`
}

type startAdminSpaceExportResponse struct {
	JobID     string `json:"jobId"`
	StreamURL string `json:"streamUrl"`
}

// NewAdminSpaceExportHandler 创建后台空间导出处理器。
func NewAdminSpaceExportHandler(exportService *service.AdminSpaceExportService) *adminSpaceExportHandler {
	return &adminSpaceExportHandler{exportService: exportService}
}

// StartExport 创建空间导出任务。
func (h *adminSpaceExportHandler) StartExport(c *gin.Context) {
	if h == nil || h.exportService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, ok := adminActorUserID(c, response.AdminSpaceErrAdminActorMissing)
	if !ok {
		return
	}

	var req startAdminSpaceExportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logit.SetRequestAttrs(c.Request.Context(), logit.Any("errmsg", err))
		response.AdminSpaceExportErrRequestBody.Write(c)
		return
	}

	result, err := h.exportService.StartExport(c.Request.Context(), service.StartAdminSpaceExportInput{
		ActorUserID:          actorUserID,
		SpaceID:              c.Param("spaceId"),
		Format:               req.Format,
		IncludeAttachments:   req.IncludeAttachments,
		IncludeOfficeSources: req.IncludeOfficeSources,
	})
	if err != nil {
		writeAdminSpaceExportError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, startAdminSpaceExportResponse{
		JobID:     result.JobID,
		StreamURL: result.StreamURL,
	})
}

// StreamEvents 是导出任务 SSE 入口；任务事件 Store 在 Phase 2 补齐。
func (h *adminSpaceExportHandler) StreamEvents(c *gin.Context) {
	if h == nil || h.exportService == nil {
		response.InternalError(c)
		return
	}
	actorUserID, ok := adminActorUserID(c, response.AdminSpaceErrAdminActorMissing)
	if !ok {
		return
	}
	initial, events, unsubscribe, err := h.exportService.Subscribe(
		c.Request.Context(),
		c.Param("jobId"),
		actorUserID,
		c.Query("token"),
	)
	if err != nil {
		writeAdminSpaceExportError(c, err)
		return
	}
	defer unsubscribe()
	writeAdminSpaceTransferSSE(c, initial, events)
}

// Download 是导出文件下载入口；真实文件生成和 token 校验在后续阶段补齐。
func (h *adminSpaceExportHandler) Download(c *gin.Context) {
	if h == nil || h.exportService == nil {
		response.InternalError(c)
		return
	}
	actorUserID, ok := adminActorUserID(c, response.AdminSpaceErrAdminActorMissing)
	if !ok {
		return
	}
	download, err := h.exportService.ConsumeDownloadToken(c.Param("jobId"), actorUserID, c.Query("token"))
	if err != nil {
		writeAdminSpaceExportError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	c.FileAttachment(download.FilePath, download.FileName)
}

func writeAdminSpaceExportError(c *gin.Context, err error) {
	if errors.Is(err, errcode.ErrAdminForbidden) {
		response.Error(c, http.StatusForbidden, response.CodeForbidden, "无权导出该空间")
		return
	}
	if response.WriteMappedError(c, err,
		response.ErrorTemplateMapping{Target: errcode.ErrAdminSpaceExportSpaceIDRequired, Template: response.AdminSpaceExportErrSpaceIDRequired},
		response.ErrorTemplateMapping{Target: errcode.ErrAdminSpaceExportRequestBody, Template: response.AdminSpaceExportErrRequestBody},
		response.ErrorTemplateMapping{Target: errcode.ErrAdminSpaceExportFormatUnsupported, Template: response.AdminSpaceExportErrFormatUnsupported},
		response.ErrorTemplateMapping{Target: errcode.ErrAdminSpaceExportJobNotFound, Template: response.AdminSpaceExportErrJobNotFound},
		response.ErrorTemplateMapping{Target: errcode.ErrAdminSpaceExportJobTokenInvalid, Template: response.AdminSpaceExportErrJobTokenInvalid},
		response.ErrorTemplateMapping{Target: errcode.ErrAdminSpaceExportJobRunningLimit, Template: response.AdminSpaceExportErrJobRunningLimit},
		response.ErrorTemplateMapping{Target: errcode.ErrAdminSpaceExportFileNotReady, Template: response.AdminSpaceExportErrFileNotReady},
		response.ErrorTemplateMapping{Target: errcode.ErrAdminSpaceExportFileExpired, Template: response.AdminSpaceExportErrFileExpired},
		response.ErrorTemplateMapping{Target: errcode.ErrAdminSpaceExportDownloadForbidden, Template: response.AdminSpaceExportErrDownloadForbidden},
	) {
		return
	}
	logit.SetRequestAttrs(c.Request.Context(), logit.Any("errmsg", err))
	response.InternalError(c)
}

func adminActorUserID(c *gin.Context, template response.ErrorTemplate) (string, bool) {
	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		logit.SetRequestAttrs(c.Request.Context(), logit.Any("errmsg", err))
		template.Write(c)
		return "", false
	}
	return actorUserID, true
}

func writeAdminSpaceTransferSSE(c *gin.Context, initial service.AdminSpaceTransferEvent, events <-chan service.AdminSpaceTransferEvent) {
	writer := c.Writer
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	writer.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := writer.(http.Flusher)
	if !ok {
		response.InternalError(c)
		return
	}
	if err := clearAdminSpaceTransferSSEWriteDeadline(writer); err != nil {
		// SSE 长连接如果无法解除写截止时间，仍允许客户端通过 EventSource 自动重连兜底；
		// 这里必须打日志，便于定位代理或 ResponseWriter 不支持导致的周期性断连。
		slog.WarnContext(c.Request.Context(), "清理空间传输 SSE 写截止时间失败", "error", err)
	}

	writeAdminSpaceTransferSSEEvent(writer, flusher, initial)
	if isTerminalAdminSpaceTransferEvent(initial) {
		return
	}

	heartbeat := time.NewTicker(adminSpaceTransferSSEHeartbeatInterval)
	defer heartbeat.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-heartbeat.C:
			_, _ = fmt.Fprint(writer, ": heartbeat\n\n")
			flusher.Flush()
		case event, ok := <-events:
			if !ok {
				return
			}
			writeAdminSpaceTransferSSEEvent(writer, flusher, event)
			if isTerminalAdminSpaceTransferEvent(event) {
				return
			}
		}
	}
}

// adminSpaceTransferSSEHeartbeatInterval 必须低于常见 10s 代理 idle timeout，
// 否则大文件导入/导出长时间无业务事件时，EventSource 会被代理切断并反复重连。
const adminSpaceTransferSSEHeartbeatInterval = 5 * time.Second

func clearAdminSpaceTransferSSEWriteDeadline(writer http.ResponseWriter) error {
	if writer == nil {
		return nil
	}
	// SSE 是长连接响应，不能继承 http.Server.WriteTimeout 的绝对写截止时间；
	// 失败时交给调用方记录日志，由 heartbeat 和客户端重连兜底。
	if err := http.NewResponseController(writer).SetWriteDeadline(time.Time{}); err != nil {
		return fmt.Errorf("清理 SSE 写截止时间: %w", err)
	}
	return nil
}

func writeAdminSpaceTransferSSEEvent(writer gin.ResponseWriter, flusher http.Flusher, event service.AdminSpaceTransferEvent) {
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(writer, "event: %s\n", event.Type)
	_, _ = fmt.Fprintf(writer, "data: %s\n\n", payload)
	flusher.Flush()
}

func isTerminalAdminSpaceTransferEvent(event service.AdminSpaceTransferEvent) bool {
	return event.Type == service.AdminSpaceTransferEventTypeCompleted || event.Type == service.AdminSpaceTransferEventTypeFailed
}
