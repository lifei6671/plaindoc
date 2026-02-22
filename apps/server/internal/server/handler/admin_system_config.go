package handler

import (
	"errors"
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
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "admin actor is missing")
		return
	}

	items, err := h.adminSystemConfigService.ListConfigs(c.Request.Context(), actorUserID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAdminForbidden):
			response.Error(c, http.StatusForbidden, "FORBIDDEN", "platform admin role is required")
		default:
			response.InternalError(c)
		}
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
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "admin actor is missing")
		return
	}

	configKey := strings.TrimSpace(c.Param("key"))
	if configKey == "" {
		response.Error(c, http.StatusBadRequest, "INVALID_CONFIG_KEY", "config key is required")
		return
	}

	var req upsertAdminSystemConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
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
		switch {
		case errors.Is(err, service.ErrAdminForbidden):
			response.Error(c, http.StatusForbidden, "FORBIDDEN", "platform admin role is required")
		case errors.Is(err, service.ErrAdminSystemConfigInvalidKey):
			response.Error(c, http.StatusBadRequest, "INVALID_CONFIG_KEY", "config key is invalid")
		case errors.Is(err, service.ErrAdminSystemConfigInvalidValue):
			response.Error(c, http.StatusBadRequest, "INVALID_CONFIG_VALUE", "config value is invalid")
		case errors.Is(err, service.ErrAdminSystemConfigExpectedVersion):
			response.Error(c, http.StatusBadRequest, "INVALID_EXPECTED_VERSION", "expectedVersion must be positive integer")
		case errors.Is(err, service.ErrAdminSystemConfigVersionConflict):
			response.Error(c, http.StatusConflict, "CONFIG_VERSION_CONFLICT", "config version conflict")
		default:
			response.InternalError(c)
		}
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
