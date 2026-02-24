package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/middleware"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
)

type adminSystemConfigHandler struct {
	adminSystemConfigService *service.AdminSystemConfigService
}

type adminSystemConfigResponse struct {
	ConfigKey       string         `json:"configKey"`
	Value           map[string]any `json:"value"`
	Version         int            `json:"version"`
	UpdatedByUserID *string        `json:"updatedByUserId"`
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
}

type upsertAdminSystemConfigRequest struct {
	Value           any  `json:"value"`
	ExpectedVersion *int `json:"expectedVersion"`
}

// NewAdminSystemConfigHandler 创建后台系统配置处理器。
func NewAdminSystemConfigHandler(
	adminSystemConfigService *service.AdminSystemConfigService,
) *adminSystemConfigHandler {
	return &adminSystemConfigHandler{adminSystemConfigService: adminSystemConfigService}
}

// ListConfigs 返回系统配置列表。
func (h *adminSystemConfigHandler) ListConfigs(c *gin.Context) {
	if h == nil || h.adminSystemConfigService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.AdminSystemConfigErrAdminActorMissing.Write(c)
		return
	}

	items, err := h.adminSystemConfigService.ListConfigs(c.Request.Context(), actorUserID)
	if err != nil {
		response.FromError(c, err)
		return
	}

	payload := make([]adminSystemConfigResponse, 0, len(items))
	for _, item := range items {
		payload = append(payload, mapAdminSystemConfigResponse(item))
	}
	response.JSON(c, http.StatusOK, payload)
}

// UpsertConfig 创建或更新系统配置。
func (h *adminSystemConfigHandler) UpsertConfig(c *gin.Context) {
	if h == nil || h.adminSystemConfigService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.AdminSystemConfigErrAdminActorMissing.Write(c)
		return
	}

	configKey := strings.TrimSpace(c.Param("key"))
	if configKey == "" {
		response.AdminSystemConfigErrConfigKeyRequired.Write(c)
		return
	}

	var req upsertAdminSystemConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AdminSystemConfigErrRequestBody.Write(c)
		return
	}

	item, err := h.adminSystemConfigService.UpsertConfig(c.Request.Context(), service.UpsertAdminSystemConfigInput{
		ActorUserID:     actorUserID,
		RequestID:       response.RequestIDFromContext(c),
		ConfigKey:       configKey,
		Value:           req.Value,
		ExpectedVersion: req.ExpectedVersion,
	})
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, mapAdminSystemConfigResponse(item))
}

func mapAdminSystemConfigResponse(value service.AdminSystemConfigRecord) adminSystemConfigResponse {
	return adminSystemConfigResponse{
		ConfigKey:       value.ConfigKey,
		Value:           value.Value,
		Version:         value.Version,
		UpdatedByUserID: value.UpdatedByUserID,
		CreatedAt:       value.CreatedAt,
		UpdatedAt:       value.UpdatedAt,
	}
}
