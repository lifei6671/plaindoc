package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/middleware"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
)

type adminDocumentTemplateSceneHandler struct {
	adminDocumentTemplateSceneService *service.AdminDocumentTemplateSceneService
}

type adminDocumentTemplateSceneResponse struct {
	SceneKey        string  `json:"sceneKey"`
	SceneName       string  `json:"sceneName"`
	Description     string  `json:"description"`
	Sort            int     `json:"sort"`
	Builtin         bool    `json:"builtin"`
	TemplateCount   int64   `json:"templateCount"`
	CreatedByUserID *string `json:"createdByUserId,omitempty"`
	UpdatedByUserID *string `json:"updatedByUserId,omitempty"`
	CreatedAt       string  `json:"createdAt"`
	UpdatedAt       string  `json:"updatedAt"`
}

type adminDocumentTemplateSceneSummaryResponse struct {
	SceneKey      string `json:"sceneKey"`
	SceneName     string `json:"sceneName"`
	Description   string `json:"description"`
	Sort          int    `json:"sort"`
	Builtin       bool   `json:"builtin"`
	TemplateCount int64  `json:"templateCount"`
	UpdatedAt     string `json:"updatedAt"`
}

type adminDocumentTemplateSceneListResponse struct {
	Items      []adminDocumentTemplateSceneSummaryResponse `json:"items"`
	Pagination struct {
		Page     int   `json:"page"`
		PageSize int   `json:"pageSize"`
		Total    int64 `json:"total"`
	} `json:"pagination"`
}

type createAdminDocumentTemplateSceneRequest struct {
	SceneKey    string `json:"sceneKey"`
	SceneName   string `json:"sceneName"`
	Description string `json:"description"`
	Sort        *int   `json:"sort"`
}

type updateAdminDocumentTemplateSceneRequest struct {
	SceneName   *string `json:"sceneName"`
	Description *string `json:"description"`
	Sort        *int    `json:"sort"`
}

// NewAdminDocumentTemplateSceneHandler 创建后台文档模板场景治理处理器。
func NewAdminDocumentTemplateSceneHandler(
	adminDocumentTemplateSceneService *service.AdminDocumentTemplateSceneService,
) *adminDocumentTemplateSceneHandler {
	return &adminDocumentTemplateSceneHandler{
		adminDocumentTemplateSceneService: adminDocumentTemplateSceneService,
	}
}

// ListScenes 返回后台文档模板场景列表。
func (h *adminDocumentTemplateSceneHandler) ListScenes(c *gin.Context) {
	if h == nil || h.adminDocumentTemplateSceneService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		setRequestErrmsg(c, err, "解析管理员身份失败")
		response.AdminErrAdminActorMissing.Write(c)
		return
	}

	page, err := parseAdminDocumentTemplateQueryInt(c.Query("page"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidPage, "page 参数不合法")
		return
	}
	pageSize, err := parseAdminDocumentTemplateQueryInt(c.Query("pageSize"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidPageSize, "pageSize 参数不合法")
		return
	}

	payload, err := h.adminDocumentTemplateSceneService.ListScenes(c.Request.Context(), service.AdminDocumentTemplateSceneListInput{
		ActorUserID: actorUserID,
		Keyword:     c.Query("keyword"),
		Page:        page,
		PageSize:    pageSize,
	})
	if err != nil {
		setRequestErrmsg(c, err, "查询文档模板场景列表失败")
		response.FromError(c, err)
		return
	}

	items := make([]adminDocumentTemplateSceneSummaryResponse, 0, len(payload.Items))
	for _, item := range payload.Items {
		items = append(items, adminDocumentTemplateSceneSummaryResponse{
			SceneKey:      item.SceneKey,
			SceneName:     item.SceneName,
			Description:   item.Description,
			Sort:          item.Sort,
			Builtin:       item.Builtin,
			TemplateCount: item.TemplateCount,
			UpdatedAt:     item.UpdatedAt,
		})
	}

	responsePayload := adminDocumentTemplateSceneListResponse{Items: items}
	responsePayload.Pagination.Page = payload.Page
	responsePayload.Pagination.PageSize = payload.PageSize
	responsePayload.Pagination.Total = payload.Total
	response.JSON(c, http.StatusOK, responsePayload)
}

// GetScene 返回后台文档模板场景详情。
func (h *adminDocumentTemplateSceneHandler) GetScene(c *gin.Context) {
	if h == nil || h.adminDocumentTemplateSceneService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		setRequestErrmsg(c, err, "解析管理员身份失败")
		response.AdminErrAdminActorMissing.Write(c)
		return
	}

	sceneKey := strings.TrimSpace(c.Param("sceneKey"))
	if sceneKey == "" {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidRequest, "sceneKey 不能为空")
		return
	}

	item, err := h.adminDocumentTemplateSceneService.GetScene(c.Request.Context(), actorUserID, sceneKey)
	if err != nil {
		setRequestErrmsg(c, err, "查询文档模板场景详情失败")
		response.FromError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, mapAdminDocumentTemplateSceneResponse(item))
}

// CreateScene 创建后台文档模板场景。
func (h *adminDocumentTemplateSceneHandler) CreateScene(c *gin.Context) {
	if h == nil || h.adminDocumentTemplateSceneService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		setRequestErrmsg(c, err, "解析管理员身份失败")
		response.AdminErrAdminActorMissing.Write(c)
		return
	}

	var req createAdminDocumentTemplateSceneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		setRequestErrmsg(c, err, "解析请求体失败")
		response.Error(c, http.StatusBadRequest, response.CodeInvalidRequest, "请求体无效")
		return
	}

	sort := 0
	if req.Sort != nil {
		sort = *req.Sort
	}

	item, err := h.adminDocumentTemplateSceneService.CreateScene(c.Request.Context(), service.CreateAdminDocumentTemplateSceneInput{
		ActorUserID: actorUserID,
		RequestID:   response.RequestIDFromContext(c),
		SceneKey:    req.SceneKey,
		SceneName:   req.SceneName,
		Description: req.Description,
		Sort:        sort,
	})
	if err != nil {
		setRequestErrmsg(c, err, "创建文档模板场景失败")
		response.FromError(c, err)
		return
	}

	response.JSON(c, http.StatusCreated, mapAdminDocumentTemplateSceneResponse(item))
}

// UpdateScene 更新后台文档模板场景。
func (h *adminDocumentTemplateSceneHandler) UpdateScene(c *gin.Context) {
	if h == nil || h.adminDocumentTemplateSceneService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		setRequestErrmsg(c, err, "解析管理员身份失败")
		response.AdminErrAdminActorMissing.Write(c)
		return
	}

	sceneKey := strings.TrimSpace(c.Param("sceneKey"))
	if sceneKey == "" {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidRequest, "sceneKey 不能为空")
		return
	}

	var req updateAdminDocumentTemplateSceneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		setRequestErrmsg(c, err, "解析请求体失败")
		response.Error(c, http.StatusBadRequest, response.CodeInvalidRequest, "请求体无效")
		return
	}

	item, err := h.adminDocumentTemplateSceneService.UpdateScene(c.Request.Context(), service.UpdateAdminDocumentTemplateSceneInput{
		ActorUserID: actorUserID,
		RequestID:   response.RequestIDFromContext(c),
		SceneKey:    sceneKey,
		SceneName:   req.SceneName,
		Description: req.Description,
		Sort:        req.Sort,
	})
	if err != nil {
		setRequestErrmsg(c, err, "更新文档模板场景失败")
		response.FromError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, mapAdminDocumentTemplateSceneResponse(item))
}

// DeleteScene 删除后台文档模板场景。
func (h *adminDocumentTemplateSceneHandler) DeleteScene(c *gin.Context) {
	if h == nil || h.adminDocumentTemplateSceneService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		setRequestErrmsg(c, err, "解析管理员身份失败")
		response.AdminErrAdminActorMissing.Write(c)
		return
	}

	sceneKey := strings.TrimSpace(c.Param("sceneKey"))
	if sceneKey == "" {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidRequest, "sceneKey 不能为空")
		return
	}

	if err := h.adminDocumentTemplateSceneService.DeleteScene(c.Request.Context(), actorUserID, sceneKey, response.RequestIDFromContext(c)); err != nil {
		setRequestErrmsg(c, err, "删除文档模板场景失败")
		response.FromError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, gin.H{"deleted": true})
}

func mapAdminDocumentTemplateSceneResponse(value service.AdminDocumentTemplateSceneRecord) adminDocumentTemplateSceneResponse {
	return adminDocumentTemplateSceneResponse{
		SceneKey:        value.SceneKey,
		SceneName:       value.SceneName,
		Description:     value.Description,
		Sort:            value.Sort,
		Builtin:         value.Builtin,
		TemplateCount:   value.TemplateCount,
		CreatedByUserID: value.CreatedByUserID,
		UpdatedByUserID: value.UpdatedByUserID,
		CreatedAt:       value.CreatedAt,
		UpdatedAt:       value.UpdatedAt,
	}
}
