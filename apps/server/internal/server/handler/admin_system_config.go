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

type testAdminEmailSendRequest struct {
	Value   any    `json:"value"`
	ToEmail string `json:"toEmail"`
}

type dataRetentionCleanupPolicyResponse struct {
	Enabled                        bool     `json:"enabled"`
	ScheduleMinutes                int      `json:"scheduleMinutes"`
	CleanupBatchSize               int      `json:"cleanupBatchSize"`
	CleanupTables                  []string `json:"cleanupTables"`
	AuditLogRetentionDays          int      `json:"auditLogRetentionDays"`
	AuthCaptchaRetentionHours      int      `json:"authCaptchaRetentionHours"`
	AuthRiskStateRetentionDays     int      `json:"authRiskStateRetentionDays"`
	UserSessionRetentionDays       int      `json:"userSessionRetentionDays"`
	DocumentRevisionRetentionCount int      `json:"documentRevisionRetentionCount"`
}

type runDataRetentionCleanupResponse struct {
	Policy                       dataRetentionCleanupPolicyResponse `json:"policy"`
	StartedAt                    time.Time                          `json:"startedAt"`
	FinishedAt                   time.Time                          `json:"finishedAt"`
	DeletedAuditLogs             int64                              `json:"deletedAuditLogs"`
	DeletedAuthCaptchaChallenges int64                              `json:"deletedAuthCaptchaChallenges"`
	DeletedAuthRiskStates        int64                              `json:"deletedAuthRiskStates"`
	DeletedUserSessions          int64                              `json:"deletedUserSessions"`
	DeletedDocumentAttachments   int64                              `json:"deletedDocumentAttachments"`
	DeletedAttachmentBlobs       int64                              `json:"deletedAttachmentBlobs"`
	DeletedDocumentImageAssets   int64                              `json:"deletedDocumentImageAssets"`
	DeletedDocumentRevisions     int64                              `json:"deletedDocumentRevisions"`
	TotalDeleted                 int64                              `json:"totalDeleted"`
}

type runSearchIndexRebuildResponse struct {
	Provider         string `json:"provider"`
	IndexedDocuments int    `json:"indexedDocuments"`
}

type searchIndexStatusResponse struct {
	Enabled                     bool       `json:"enabled"`
	ActiveProvider              string     `json:"activeProvider"`
	EffectiveProvider           string     `json:"effectiveProvider"`
	FallbackPolicy              string     `json:"fallbackPolicy"`
	ActiveAnalyzer              string     `json:"activeAnalyzer"`
	RebuildInProgress           bool       `json:"rebuildInProgress"`
	ProviderHealthy             bool       `json:"providerHealthy"`
	ProviderMessage             string     `json:"providerMessage"`
	SupportsDocCount            bool       `json:"supportsDocCount"`
	IndexedDocuments            int64      `json:"indexedDocuments"`
	LastRebuildAt               *time.Time `json:"lastRebuildAt"`
	LastRebuildSource           string     `json:"lastRebuildSource"`
	LastRebuildIndexedDocuments int        `json:"lastRebuildIndexedDocuments"`
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
		setRequestErrmsg(c, err, "解析管理员身份失败")
		response.AdminSystemConfigErrAdminActorMissing.Write(c)
		return
	}

	configKey := strings.TrimSpace(c.Param("key"))
	if configKey == "" {
		setRequestErrmsg(c, nil, "配置项不存在")
		response.AdminSystemConfigErrConfigKeyRequired.Write(c)
		return
	}

	var req upsertAdminSystemConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		setRequestErrmsg(c, err, "解析请求体失败")
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
		setRequestErrmsg(c, err, "创建/更新系统配置失败")
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
		setRequestErrmsg(c, err, "解析管理员身份失败")
		response.AdminSystemConfigErrAdminActorMissing.Write(c)
		return
	}

	var req testAdminLDAPConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		setRequestErrmsg(c, err, "解析请求体失败")
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
		setRequestErrmsg(c, err, "测试 LDAP 连接失败")
		response.FromError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, map[string]bool{"ok": true})
}

// TestEmailSend 测试 email 配置发送能力（不落库）。
func (h *adminSystemConfigHandler) TestEmailSend(c *gin.Context) {
	if h == nil || h.adminSystemConfigService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		setRequestErrmsg(c, err, "解析管理员身份失败")
		response.AdminSystemConfigErrAdminActorMissing.Write(c)
		return
	}

	var req testAdminEmailSendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		setRequestErrmsg(c, err, "解析请求体失败")
		response.AdminSystemConfigErrRequestBody.Write(c)
		return
	}

	if err := h.adminSystemConfigService.TestEmailSend(
		c.Request.Context(),
		service.TestAdminSystemConfigEmailSendInput{
			ActorUserID: actorUserID,
			RequestID:   response.RequestIDFromContext(c),
			Value:       req.Value,
			ToEmail:     strings.TrimSpace(req.ToEmail),
		},
	); err != nil {
		setRequestErrmsg(c, err, "测试邮件发送失败")
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
		setRequestErrmsg(c, err, "解析管理员身份失败")
		response.AdminSystemConfigErrAdminActorMissing.Write(c)
		return
	}

	configKey := strings.TrimSpace(c.Param("key"))
	if configKey == "" {
		setRequestErrmsg(c, nil, "配置项不存在")
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
		setRequestErrmsg(c, err, "运行数据保留清理失败")
		response.FromError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, mapRunDataRetentionCleanupResponse(result))
}

// RunSearchIndexRebuild 手动触发一次全文索引重建。
func (h *adminSystemConfigHandler) RunSearchIndexRebuild(c *gin.Context) {
	if h == nil || h.adminSystemConfigService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		setRequestErrmsg(c, err, "解析管理员身份失败")
		response.AdminSystemConfigErrAdminActorMissing.Write(c)
		return
	}

	configKey := strings.TrimSpace(c.Param("key"))
	if configKey == "" {
		setRequestErrmsg(c, nil, "配置项不存在")
		response.AdminSystemConfigErrConfigKeyRequired.Write(c)
		return
	}

	result, err := h.adminSystemConfigService.RunSearchIndexRebuild(
		c.Request.Context(),
		service.RunSearchIndexRebuildInput{
			ActorUserID: actorUserID,
			RequestID:   response.RequestIDFromContext(c),
			ConfigKey:   configKey,
		},
	)
	if err != nil {
		setRequestErrmsg(c, err, "运行全文索引重建失败")
		response.FromError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, mapRunSearchIndexRebuildResponse(result))
}

// GetSearchIndexStatus 返回全文索引状态。
func (h *adminSystemConfigHandler) GetSearchIndexStatus(c *gin.Context) {
	if h == nil || h.adminSystemConfigService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		setRequestErrmsg(c, err, "解析管理员身份失败")
		response.AdminSystemConfigErrAdminActorMissing.Write(c)
		return
	}

	configKey := strings.TrimSpace(c.Param("key"))
	if configKey == "" {
		setRequestErrmsg(c, nil, "配置项不存在")
		response.AdminSystemConfigErrConfigKeyRequired.Write(c)
		return
	}

	statusResult, err := h.adminSystemConfigService.GetSearchIndexStatus(
		c.Request.Context(),
		service.GetSearchIndexStatusInput{
			ActorUserID: actorUserID,
			ConfigKey:   configKey,
		},
	)
	if err != nil {
		setRequestErrmsg(c, err, "查询全文索引状态失败")
		response.FromError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, mapSearchIndexStatusResponse(statusResult))
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
		value.DeletedDocumentAttachments +
		value.DeletedAttachmentBlobs +
		value.DeletedDocumentImageAssets +
		value.DeletedDocumentRevisions

	return runDataRetentionCleanupResponse{
		Policy: dataRetentionCleanupPolicyResponse{
			Enabled:                        value.Policy.Enabled,
			ScheduleMinutes:                value.Policy.ScheduleMinutes,
			CleanupBatchSize:               value.Policy.CleanupBatchSize,
			CleanupTables:                  append([]string(nil), value.Policy.CleanupTables...),
			AuditLogRetentionDays:          value.Policy.AuditLogRetentionDays,
			AuthCaptchaRetentionHours:      value.Policy.AuthCaptchaRetentionHours,
			AuthRiskStateRetentionDays:     value.Policy.AuthRiskStateRetentionDays,
			UserSessionRetentionDays:       value.Policy.UserSessionRetentionDays,
			DocumentRevisionRetentionCount: value.Policy.DocumentRevisionRetentionCount,
		},
		StartedAt:                    value.StartedAt,
		FinishedAt:                   value.FinishedAt,
		DeletedAuditLogs:             value.DeletedAuditLogs,
		DeletedAuthCaptchaChallenges: value.DeletedAuthCaptchaChallenges,
		DeletedAuthRiskStates:        value.DeletedAuthRiskStates,
		DeletedUserSessions:          value.DeletedUserSessions,
		DeletedDocumentAttachments:   value.DeletedDocumentAttachments,
		DeletedAttachmentBlobs:       value.DeletedAttachmentBlobs,
		DeletedDocumentImageAssets:   value.DeletedDocumentImageAssets,
		DeletedDocumentRevisions:     value.DeletedDocumentRevisions,
		TotalDeleted:                 totalDeleted,
	}
}

func mapRunSearchIndexRebuildResponse(
	value service.SearchIndexRebuildResult,
) runSearchIndexRebuildResponse {
	return runSearchIndexRebuildResponse{
		Provider:         string(value.Provider),
		IndexedDocuments: value.IndexedDocuments,
	}
}

func mapSearchIndexStatusResponse(
	value service.SearchIndexStatusResult,
) searchIndexStatusResponse {
	return searchIndexStatusResponse{
		Enabled:                     value.Enabled,
		ActiveProvider:              string(value.ActiveProvider),
		EffectiveProvider:           string(value.EffectiveProvider),
		FallbackPolicy:              string(value.FallbackPolicy),
		ActiveAnalyzer:              string(value.ActiveAnalyzer),
		RebuildInProgress:           value.RebuildInProgress,
		ProviderHealthy:             value.ProviderHealthy,
		ProviderMessage:             value.ProviderMessage,
		SupportsDocCount:            value.SupportsDocCount,
		IndexedDocuments:            value.IndexedDocuments,
		LastRebuildAt:               value.LastRebuildAt,
		LastRebuildSource:           value.LastRebuildSource,
		LastRebuildIndexedDocuments: value.LastRebuildIndexedDocuments,
	}
}
