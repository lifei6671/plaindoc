package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/middleware"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
)

type adminDocumentTemplateHandler struct {
	adminDocumentTemplateService *service.AdminDocumentTemplateService
}

type adminDocumentTemplateResponse struct {
	TemplateID      string  `json:"templateId"`
	SceneKey        string  `json:"sceneKey"`
	SceneName       string  `json:"sceneName"`
	Name            string  `json:"name"`
	Description     string  `json:"description"`
	DefaultTitle    string  `json:"defaultTitle"`
	ContentMD       string  `json:"contentMd"`
	Sort            int     `json:"sort"`
	Builtin         bool    `json:"builtin"`
	Enabled         bool    `json:"enabled"`
	CreatedByUserID *string `json:"createdByUserId,omitempty"`
	UpdatedByUserID *string `json:"updatedByUserId,omitempty"`
	CreatedAt       string  `json:"createdAt"`
	UpdatedAt       string  `json:"updatedAt"`
}

type adminDocumentTemplateSummaryResponse struct {
	TemplateID   string `json:"templateId"`
	SceneKey     string `json:"sceneKey"`
	SceneName    string `json:"sceneName"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	DefaultTitle string `json:"defaultTitle"`
	Sort         int    `json:"sort"`
	Builtin      bool   `json:"builtin"`
	Enabled      bool   `json:"enabled"`
	UpdatedAt    string `json:"updatedAt"`
}

type adminDocumentTemplateListResponse struct {
	Items      []adminDocumentTemplateSummaryResponse `json:"items"`
	Pagination struct {
		Page     int   `json:"page"`
		PageSize int   `json:"pageSize"`
		Total    int64 `json:"total"`
	} `json:"pagination"`
}

type createAdminDocumentTemplateRequest struct {
	TemplateID   string `json:"templateId"`
	SceneKey     string `json:"sceneKey"`
	SceneName    string `json:"sceneName"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	DefaultTitle string `json:"defaultTitle"`
	ContentMD    string `json:"contentMd"`
	Sort         *int   `json:"sort"`
	Enabled      *bool  `json:"enabled"`
}

type updateAdminDocumentTemplateRequest struct {
	SceneKey     *string `json:"sceneKey"`
	SceneName    *string `json:"sceneName"`
	Name         *string `json:"name"`
	Description  *string `json:"description"`
	DefaultTitle *string `json:"defaultTitle"`
	ContentMD    *string `json:"contentMd"`
	Sort         *int    `json:"sort"`
	Enabled      *bool   `json:"enabled"`
}

// NewAdminDocumentTemplateHandler 创建后台文档模板治理处理器。
func NewAdminDocumentTemplateHandler(
	adminDocumentTemplateService *service.AdminDocumentTemplateService,
) *adminDocumentTemplateHandler {
	return &adminDocumentTemplateHandler{
		adminDocumentTemplateService: adminDocumentTemplateService,
	}
}

// ListTemplates 返回后台文档模板列表。
func (h *adminDocumentTemplateHandler) ListTemplates(c *gin.Context) {
	if h == nil || h.adminDocumentTemplateService == nil {
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

	payload, err := h.adminDocumentTemplateService.ListTemplates(c.Request.Context(), service.AdminDocumentTemplateListInput{
		ActorUserID: actorUserID,
		SceneKey:    c.Query("sceneKey"),
		Keyword:     c.Query("keyword"),
		Page:        page,
		PageSize:    pageSize,
	})
	if err != nil {
		setRequestErrmsg(c, err, "查询文档模板列表失败")
		response.FromError(c, err)
		return
	}

	items := make([]adminDocumentTemplateSummaryResponse, 0, len(payload.Items))
	for _, item := range payload.Items {
		items = append(items, adminDocumentTemplateSummaryResponse{
			TemplateID:   item.TemplateID,
			SceneKey:     item.SceneKey,
			SceneName:    item.SceneName,
			Name:         item.Name,
			Description:  item.Description,
			DefaultTitle: item.DefaultTitle,
			Sort:         item.Sort,
			Builtin:      item.Builtin,
			Enabled:      item.Enabled,
			UpdatedAt:    item.UpdatedAt,
		})
	}

	responsePayload := adminDocumentTemplateListResponse{
		Items: items,
	}
	responsePayload.Pagination.Page = payload.Page
	responsePayload.Pagination.PageSize = payload.PageSize
	responsePayload.Pagination.Total = payload.Total
	response.JSON(c, http.StatusOK, responsePayload)
}

// GetTemplate 返回后台文档模板详情。
func (h *adminDocumentTemplateHandler) GetTemplate(c *gin.Context) {
	if h == nil || h.adminDocumentTemplateService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		setRequestErrmsg(c, err, "解析管理员身份失败")
		response.AdminErrAdminActorMissing.Write(c)
		return
	}

	templateID := strings.TrimSpace(c.Param("templateId"))
	if templateID == "" {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidTemplateID, "templateId 不能为空")
		return
	}

	item, err := h.adminDocumentTemplateService.GetTemplate(c.Request.Context(), actorUserID, templateID)
	if err != nil {
		setRequestErrmsg(c, err, "查询文档模板详情失败")
		response.FromError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, mapAdminDocumentTemplateResponse(item))
}

// CreateTemplate 创建后台文档模板。
func (h *adminDocumentTemplateHandler) CreateTemplate(c *gin.Context) {
	if h == nil || h.adminDocumentTemplateService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		setRequestErrmsg(c, err, "解析管理员身份失败")
		response.AdminErrAdminActorMissing.Write(c)
		return
	}

	var req createAdminDocumentTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		setRequestErrmsg(c, err, "解析请求体失败")
		response.Error(c, http.StatusBadRequest, response.CodeInvalidRequest, "请求体无效")
		return
	}

	sort := 0
	if req.Sort != nil {
		sort = *req.Sort
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	item, err := h.adminDocumentTemplateService.CreateTemplate(c.Request.Context(), service.CreateAdminDocumentTemplateInput{
		ActorUserID:  actorUserID,
		RequestID:    response.RequestIDFromContext(c),
		TemplateID:   req.TemplateID,
		SceneKey:     req.SceneKey,
		SceneName:    req.SceneName,
		Name:         req.Name,
		Description:  req.Description,
		DefaultTitle: req.DefaultTitle,
		ContentMD:    req.ContentMD,
		Sort:         sort,
		Enabled:      enabled,
	})
	if err != nil {
		setRequestErrmsg(c, err, "创建文档模板失败")
		response.FromError(c, err)
		return
	}

	response.JSON(c, http.StatusCreated, mapAdminDocumentTemplateResponse(item))
}

// UpdateTemplate 更新后台文档模板。
func (h *adminDocumentTemplateHandler) UpdateTemplate(c *gin.Context) {
	if h == nil || h.adminDocumentTemplateService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		setRequestErrmsg(c, err, "解析管理员身份失败")
		response.AdminErrAdminActorMissing.Write(c)
		return
	}

	templateID := strings.TrimSpace(c.Param("templateId"))
	if templateID == "" {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidTemplateID, "templateId 不能为空")
		return
	}

	var req updateAdminDocumentTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		setRequestErrmsg(c, err, "解析请求体失败")
		response.Error(c, http.StatusBadRequest, response.CodeInvalidRequest, "请求体无效")
		return
	}

	item, err := h.adminDocumentTemplateService.UpdateTemplate(c.Request.Context(), service.UpdateAdminDocumentTemplateInput{
		ActorUserID:  actorUserID,
		RequestID:    response.RequestIDFromContext(c),
		TemplateID:   templateID,
		SceneKey:     req.SceneKey,
		SceneName:    req.SceneName,
		Name:         req.Name,
		Description:  req.Description,
		DefaultTitle: req.DefaultTitle,
		ContentMD:    req.ContentMD,
		Sort:         req.Sort,
		Enabled:      req.Enabled,
	})
	if err != nil {
		setRequestErrmsg(c, err, "更新文档模板失败")
		response.FromError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, mapAdminDocumentTemplateResponse(item))
}

// DeleteTemplate 删除后台文档模板。
func (h *adminDocumentTemplateHandler) DeleteTemplate(c *gin.Context) {
	if h == nil || h.adminDocumentTemplateService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		setRequestErrmsg(c, err, "解析管理员身份失败")
		response.AdminErrAdminActorMissing.Write(c)
		return
	}

	templateID := strings.TrimSpace(c.Param("templateId"))
	if templateID == "" {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidTemplateID, "templateId 不能为空")
		return
	}

	if err := h.adminDocumentTemplateService.DeleteTemplate(
		c.Request.Context(),
		actorUserID,
		templateID,
		response.RequestIDFromContext(c),
	); err != nil {
		setRequestErrmsg(c, err, "删除文档模板失败")
		response.FromError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, struct{}{})
}

func mapAdminDocumentTemplateResponse(value service.AdminDocumentTemplateRecord) adminDocumentTemplateResponse {
	return adminDocumentTemplateResponse{
		TemplateID:      value.TemplateID,
		SceneKey:        value.SceneKey,
		SceneName:       value.SceneName,
		Name:            value.Name,
		Description:     value.Description,
		DefaultTitle:    value.DefaultTitle,
		ContentMD:       value.ContentMD,
		Sort:            value.Sort,
		Builtin:         value.Builtin,
		Enabled:         value.Enabled,
		CreatedByUserID: value.CreatedByUserID,
		UpdatedByUserID: value.UpdatedByUserID,
		CreatedAt:       value.CreatedAt,
		UpdatedAt:       value.UpdatedAt,
	}
}

func parseAdminDocumentTemplateQueryInt(raw string) (int, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, nil
	}
	parsedValue, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	if parsedValue < 0 {
		return 0, strconv.ErrSyntax
	}
	return parsedValue, nil
}
