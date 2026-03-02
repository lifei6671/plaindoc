package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/middleware"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
)

type adminSearchAnalyzerHandler struct {
	adminSearchAnalyzerService *service.AdminSearchAnalyzerService
}

type adminSearchAnalyzerResponse struct {
	Name               string `json:"name"`
	Enabled            bool   `json:"enabled"`
	Active             bool   `json:"active"`
	DictVersion        string `json:"dictVersion"`
	SupportsUserDict   bool   `json:"supportsUserDict"`
	SupportsHotReload  bool   `json:"supportsHotReload"`
	SupportsPhraseHint bool   `json:"supportsPhraseHint"`
	SupportsStopwords  bool   `json:"supportsStopwords"`
	SupportsSynonyms   bool   `json:"supportsSynonyms"`
}

type adminSearchAnalyzerDictEntryResponse struct {
	ID              int64     `json:"id"`
	Analyzer        string    `json:"analyzer"`
	Term            string    `json:"term"`
	Weight          *int      `json:"weight"`
	Tag             string    `json:"tag"`
	Status          string    `json:"status"`
	CreatedByUserID *string   `json:"createdByUserId"`
	UpdatedByUserID *string   `json:"updatedByUserId"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type adminSearchAnalyzerDictPageResponse struct {
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
	Total    int64 `json:"total"`
}

type adminSearchAnalyzerDictListResponse struct {
	Items      []adminSearchAnalyzerDictEntryResponse `json:"items"`
	Pagination adminSearchAnalyzerDictPageResponse    `json:"pagination"`
}

type createAdminSearchAnalyzerDictEntryRequest struct {
	Term   string `json:"term"`
	Weight *int   `json:"weight"`
	Tag    string `json:"tag"`
}

type updateAdminSearchAnalyzerDictEntryRequest struct {
	Term   string `json:"term"`
	Weight *int   `json:"weight"`
	Tag    string `json:"tag"`
}

type adminSearchAnalyzerReloadResponse struct {
	Analyzer       string    `json:"analyzer"`
	DictVersion    string    `json:"dictVersion"`
	SourceVersion  int       `json:"sourceVersion"`
	LoadedAt       time.Time `json:"loadedAt"`
	ActiveAnalyzer string    `json:"activeAnalyzer"`
}

type adminSearchAnalyzerAnalyzePreviewRequest struct {
	Text     string `json:"text"`
	Mode     string `json:"mode"`
	Language string `json:"language"`
	SpaceID  string `json:"spaceId"`
}

type adminSearchAnalyzerAnalyzePreviewResponse struct {
	Analyzer       string   `json:"analyzer"`
	Mode           string   `json:"mode"`
	Tokens         []string `json:"tokens"`
	NormalizedText string   `json:"normalizedText"`
	TokenCount     int      `json:"tokenCount"`
	DictVersion    string   `json:"dictVersion"`
}

// NewAdminSearchAnalyzerHandler 创建后台分词器治理处理器。
func NewAdminSearchAnalyzerHandler(
	adminSearchAnalyzerService *service.AdminSearchAnalyzerService,
) *adminSearchAnalyzerHandler {
	return &adminSearchAnalyzerHandler{
		adminSearchAnalyzerService: adminSearchAnalyzerService,
	}
}

// ListAnalyzers 返回后台分词器列表。
func (h *adminSearchAnalyzerHandler) ListAnalyzers(c *gin.Context) {
	if h == nil || h.adminSearchAnalyzerService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		setRequestErrmsg(c, err, "解析管理员身份失败")
		response.AdminSearchAnalyzerErrAdminActorMissing.Write(c)
		return
	}

	items, err := h.adminSearchAnalyzerService.ListAnalyzers(c.Request.Context(), actorUserID)
	if err != nil {
		setRequestErrmsg(c, err, "查询分词器列表失败")
		response.FromError(c, err)
		return
	}

	payload := make([]adminSearchAnalyzerResponse, 0, len(items))
	for _, item := range items {
		payload = append(payload, adminSearchAnalyzerResponse{
			Name:               item.Name,
			Enabled:            item.Enabled,
			Active:             item.Active,
			DictVersion:        item.DictVersion,
			SupportsUserDict:   item.SupportsUserDict,
			SupportsHotReload:  item.SupportsHotReload,
			SupportsPhraseHint: item.SupportsPhraseHint,
			SupportsStopwords:  item.SupportsStopwords,
			SupportsSynonyms:   item.SupportsSynonyms,
		})
	}
	response.JSON(c, http.StatusOK, payload)
}

// ListDictEntries 返回后台词典词条列表。
func (h *adminSearchAnalyzerHandler) ListDictEntries(c *gin.Context) {
	if h == nil || h.adminSearchAnalyzerService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		setRequestErrmsg(c, err, "解析管理员身份失败")
		response.AdminSearchAnalyzerErrAdminActorMissing.Write(c)
		return
	}

	analyzer := strings.TrimSpace(c.Param("analyzer"))
	if analyzer == "" {
		setRequestErrmsgText(c, "分词器名称不能为空")
		response.AdminSearchAnalyzerErrAnalyzerRequired.Write(c)
		return
	}

	page, err := parseAdminDocumentQueryInt(c.Query("page"))
	if err != nil {
		setRequestErrmsg(c, err, "解析页码参数失败")
		response.AdminSearchAnalyzerErrPagePositiveInteger.Write(c)
		return
	}
	pageSize, err := parseAdminDocumentQueryInt(c.Query("pageSize"))
	if err != nil {
		setRequestErrmsg(c, err, "解析每页数量参数失败")
		response.AdminSearchAnalyzerErrPageSizePositiveInteger.Write(c)
		return
	}

	payload, err := h.adminSearchAnalyzerService.ListDictEntries(c.Request.Context(), service.ListAdminSearchAnalyzerDictEntriesInput{
		ActorUserID:  actorUserID,
		Analyzer:     analyzer,
		StatusFilter: service.AdminSearchAnalyzerDictStatusFilter(c.Query("status")),
		Page:         page,
		PageSize:     pageSize,
	})
	if err != nil {
		setRequestErrmsg(c, err, "查询词典列表失败")
		response.FromError(c, err)
		return
	}

	items := make([]adminSearchAnalyzerDictEntryResponse, 0, len(payload.Items))
	for _, item := range payload.Items {
		items = append(items, mapAdminSearchAnalyzerDictEntryResponse(item))
	}
	response.JSON(c, http.StatusOK, adminSearchAnalyzerDictListResponse{
		Items: items,
		Pagination: adminSearchAnalyzerDictPageResponse{
			Page:     payload.Page,
			PageSize: payload.PageSize,
			Total:    payload.Total,
		},
	})
}

// CreateDictEntry 创建后台词典词条。
func (h *adminSearchAnalyzerHandler) CreateDictEntry(c *gin.Context) {
	if h == nil || h.adminSearchAnalyzerService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		setRequestErrmsg(c, err, "解析管理员身份失败")
		response.AdminSearchAnalyzerErrAdminActorMissing.Write(c)
		return
	}

	analyzer := strings.TrimSpace(c.Param("analyzer"))
	if analyzer == "" {
		setRequestErrmsgText(c, "分词器名称不能为空")
		response.AdminSearchAnalyzerErrAnalyzerRequired.Write(c)
		return
	}

	var req createAdminSearchAnalyzerDictEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		setRequestErrmsg(c, err, "解析请求体失败")
		response.AdminSearchAnalyzerErrRequestBody.Write(c)
		return
	}

	item, err := h.adminSearchAnalyzerService.CreateDictEntry(c.Request.Context(), service.UpsertAdminSearchAnalyzerDictEntryInput{
		ActorUserID: actorUserID,
		RequestID:   response.RequestIDFromContext(c),
		Analyzer:    analyzer,
		Term:        req.Term,
		Weight:      req.Weight,
		Tag:         req.Tag,
	})
	if err != nil {
		setRequestErrmsg(c, err, "创建词典词条失败")
		response.FromError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, mapAdminSearchAnalyzerDictEntryResponse(item))
}

// UpdateDictEntry 更新后台词典词条。
func (h *adminSearchAnalyzerHandler) UpdateDictEntry(c *gin.Context) {
	if h == nil || h.adminSearchAnalyzerService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		setRequestErrmsg(c, err, "解析管理员身份失败")
		response.AdminSearchAnalyzerErrAdminActorMissing.Write(c)
		return
	}

	analyzer := strings.TrimSpace(c.Param("analyzer"))
	if analyzer == "" {
		setRequestErrmsgText(c, "分词器名称不能为空")
		response.AdminSearchAnalyzerErrAnalyzerRequired.Write(c)
		return
	}

	entryID, err := parseAdminSearchAnalyzerEntryID(c.Param("entryId"))
	if err != nil {
		setRequestErrmsg(c, err, "解析词条 ID 失败")
		response.AdminSearchAnalyzerErrDictEntryIDRequired.Write(c)
		return
	}

	var req updateAdminSearchAnalyzerDictEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		setRequestErrmsg(c, err, "解析请求体失败")
		response.AdminSearchAnalyzerErrRequestBody.Write(c)
		return
	}

	item, err := h.adminSearchAnalyzerService.UpdateDictEntry(c.Request.Context(), service.UpdateAdminSearchAnalyzerDictEntryInput{
		ActorUserID: actorUserID,
		RequestID:   response.RequestIDFromContext(c),
		Analyzer:    analyzer,
		EntryID:     entryID,
		Term:        req.Term,
		Weight:      req.Weight,
		Tag:         req.Tag,
	})
	if err != nil {
		setRequestErrmsg(c, err, "更新词典词条失败")
		response.FromError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, mapAdminSearchAnalyzerDictEntryResponse(item))
}

// DeleteDictEntry 删除后台词典词条。
func (h *adminSearchAnalyzerHandler) DeleteDictEntry(c *gin.Context) {
	if h == nil || h.adminSearchAnalyzerService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		setRequestErrmsg(c, err, "解析管理员身份失败")
		response.AdminSearchAnalyzerErrAdminActorMissing.Write(c)
		return
	}

	analyzer := strings.TrimSpace(c.Param("analyzer"))
	if analyzer == "" {
		setRequestErrmsgText(c, "分词器名称不能为空")
		response.AdminSearchAnalyzerErrAnalyzerRequired.Write(c)
		return
	}

	entryID, err := parseAdminSearchAnalyzerEntryID(c.Param("entryId"))
	if err != nil {
		setRequestErrmsg(c, err, "解析词条 ID 失败")
		response.AdminSearchAnalyzerErrDictEntryIDRequired.Write(c)
		return
	}

	item, err := h.adminSearchAnalyzerService.DeleteDictEntry(c.Request.Context(), service.DeleteAdminSearchAnalyzerDictEntryInput{
		ActorUserID: actorUserID,
		RequestID:   response.RequestIDFromContext(c),
		Analyzer:    analyzer,
		EntryID:     entryID,
	})
	if err != nil {
		setRequestErrmsg(c, err, "删除词典词条失败")
		response.FromError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, mapAdminSearchAnalyzerDictEntryResponse(item))
}

// ReloadAnalyzer 触发后台分词器重载。
func (h *adminSearchAnalyzerHandler) ReloadAnalyzer(c *gin.Context) {
	if h == nil || h.adminSearchAnalyzerService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		setRequestErrmsg(c, err, "解析管理员身份失败")
		response.AdminSearchAnalyzerErrAdminActorMissing.Write(c)
		return
	}

	analyzer := strings.TrimSpace(c.Param("analyzer"))
	if analyzer == "" {
		setRequestErrmsgText(c, "分词器名称不能为空")
		response.AdminSearchAnalyzerErrAnalyzerRequired.Write(c)
		return
	}

	result, err := h.adminSearchAnalyzerService.ReloadAnalyzer(c.Request.Context(), service.ReloadAdminSearchAnalyzerInput{
		ActorUserID: actorUserID,
		RequestID:   response.RequestIDFromContext(c),
		Analyzer:    analyzer,
	})
	if err != nil {
		setRequestErrmsg(c, err, "重载分词器失败")
		response.FromError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, adminSearchAnalyzerReloadResponse{
		Analyzer:       result.Analyzer,
		DictVersion:    result.DictVersion,
		SourceVersion:  result.SourceVersion,
		LoadedAt:       result.LoadedAt,
		ActiveAnalyzer: result.ActiveAnalyzer,
	})
}

// AnalyzePreview 返回后台分词预览结果。
func (h *adminSearchAnalyzerHandler) AnalyzePreview(c *gin.Context) {
	if h == nil || h.adminSearchAnalyzerService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		setRequestErrmsg(c, err, "解析管理员身份失败")
		response.AdminSearchAnalyzerErrAdminActorMissing.Write(c)
		return
	}

	analyzer := strings.TrimSpace(c.Param("analyzer"))
	if analyzer == "" {
		setRequestErrmsgText(c, "分词器名称不能为空")
		response.AdminSearchAnalyzerErrAnalyzerRequired.Write(c)
		return
	}

	var req adminSearchAnalyzerAnalyzePreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		setRequestErrmsg(c, err, "解析请求体失败")
		response.AdminSearchAnalyzerErrRequestBody.Write(c)
		return
	}

	result, err := h.adminSearchAnalyzerService.AnalyzePreview(c.Request.Context(), service.AnalyzePreviewAdminSearchAnalyzerInput{
		ActorUserID: actorUserID,
		Analyzer:    analyzer,
		Mode:        req.Mode,
		Text:        req.Text,
		Language:    req.Language,
		SpaceID:     req.SpaceID,
	})
	if err != nil {
		setRequestErrmsg(c, err, "分词预览失败")
		response.FromError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, adminSearchAnalyzerAnalyzePreviewResponse{
		Analyzer:       result.Analyzer,
		Mode:           result.Mode,
		Tokens:         result.Tokens,
		NormalizedText: result.NormalizedText,
		TokenCount:     result.TokenCount,
		DictVersion:    result.DictVersion,
	})
}

func parseAdminSearchAnalyzerEntryID(rawValue string) (int64, error) {
	normalized := strings.TrimSpace(rawValue)
	if normalized == "" {
		return 0, strconv.ErrSyntax
	}
	id, err := strconv.ParseInt(normalized, 10, 64)
	if err != nil {
		return 0, err
	}
	if id <= 0 {
		return 0, strconv.ErrRange
	}
	return id, nil
}

func mapAdminSearchAnalyzerDictEntryResponse(
	value service.AdminSearchAnalyzerDictEntryRecord,
) adminSearchAnalyzerDictEntryResponse {
	return adminSearchAnalyzerDictEntryResponse{
		ID:              value.ID,
		Analyzer:        value.Analyzer,
		Term:            value.Term,
		Weight:          value.Weight,
		Tag:             value.Tag,
		Status:          value.Status,
		CreatedByUserID: value.CreatedByUserID,
		UpdatedByUserID: value.UpdatedByUserID,
		CreatedAt:       value.CreatedAt,
		UpdatedAt:       value.UpdatedAt,
	}
}
