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

type testAdminLDAPConnectionRequest struct {
	Value      any    `json:"value"`
	ProviderID string `json:"providerId"`
}

type dataRetentionCleanupPolicyResponse struct {
	Enabled                    bool     `json:"enabled"`
	ScheduleMinutes            int      `json:"scheduleMinutes"`
	CleanupBatchSize           int      `json:"cleanupBatchSize"`
	CleanupTables              []string `json:"cleanupTables"`
	AuditLogRetentionDays      int      `json:"auditLogRetentionDays"`
	AuthCaptchaRetentionHours  int      `json:"authCaptchaRetentionHours"`
	AuthRiskStateRetentionDays int      `json:"authRiskStateRetentionDays"`
	UserSessionRetentionDays   int      `json:"userSessionRetentionDays"`
}

type runDataRetentionCleanupResponse struct {
	Policy                       dataRetentionCleanupPolicyResponse `json:"policy"`
	StartedAt                    time.Time                          `json:"startedAt"`
	FinishedAt                   time.Time                          `json:"finishedAt"`
	DeletedAuditLogs             int64                              `json:"deletedAuditLogs"`
	DeletedAuthCaptchaChallenges int64                              `json:"deletedAuthCaptchaChallenges"`
	DeletedAuthRiskStates        int64                              `json:"deletedAuthRiskStates"`
	DeletedUserSessions          int64                              `json:"deletedUserSessions"`
	DeletedDocumentImageAssets   int64                              `json:"deletedDocumentImageAssets"`
	TotalDeleted                 int64                              `json:"totalDeleted"`
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

// TestLDAPConnection 测试 auth 配置中的 LDAP provider 连通性（不落库）。
func (h *adminSystemConfigHandler) TestLDAPConnection(c *gin.Context) {
	if h == nil || h.adminSystemConfigService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.AdminSystemConfigErrAdminActorMissing.Write(c)
		return
	}

	var req testAdminLDAPConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AdminSystemConfigErrRequestBody.Write(c)
		return
	}

	if err := h.adminSystemConfigService.TestLDAPConnection(
		c.Request.Context(),
		service.TestAdminSystemConfigLDAPConnectionInput{
			ActorUserID: actorUserID,
			Value:       req.Value,
			ProviderID:  strings.TrimSpace(req.ProviderID),
		},
	); err != nil {
		response.FromError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, map[string]bool{"ok": true})
}

// RunDataRetentionCleanup 手动触发一次 data-retention 清理。
func (h *adminSystemConfigHandler) RunDataRetentionCleanup(c *gin.Context) {
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

	result, err := h.adminSystemConfigService.RunDataRetentionCleanup(
		c.Request.Context(),
		service.RunDataRetentionCleanupInput{
			ActorUserID: actorUserID,
			RequestID:   response.RequestIDFromContext(c),
			ConfigKey:   configKey,
		},
	)
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, mapRunDataRetentionCleanupResponse(result))
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

func mapRunDataRetentionCleanupResponse(
	value service.DataRetentionCleanupResult,
) runDataRetentionCleanupResponse {
	totalDeleted := value.DeletedAuditLogs +
		value.DeletedAuthCaptchaChallenges +
		value.DeletedAuthRiskStates +
		value.DeletedUserSessions +
		value.DeletedDocumentImageAssets

	return runDataRetentionCleanupResponse{
		Policy: dataRetentionCleanupPolicyResponse{
			Enabled:                    value.Policy.Enabled,
			ScheduleMinutes:            value.Policy.ScheduleMinutes,
			CleanupBatchSize:           value.Policy.CleanupBatchSize,
			CleanupTables:              append([]string(nil), value.Policy.CleanupTables...),
			AuditLogRetentionDays:      value.Policy.AuditLogRetentionDays,
			AuthCaptchaRetentionHours:  value.Policy.AuthCaptchaRetentionHours,
			AuthRiskStateRetentionDays: value.Policy.AuthRiskStateRetentionDays,
			UserSessionRetentionDays:   value.Policy.UserSessionRetentionDays,
		},
		StartedAt:                    value.StartedAt,
		FinishedAt:                   value.FinishedAt,
		DeletedAuditLogs:             value.DeletedAuditLogs,
		DeletedAuthCaptchaChallenges: value.DeletedAuthCaptchaChallenges,
		DeletedAuthRiskStates:        value.DeletedAuthRiskStates,
		DeletedUserSessions:          value.DeletedUserSessions,
		DeletedDocumentImageAssets:   value.DeletedDocumentImageAssets,
		TotalDeleted:                 totalDeleted,
	}
}
