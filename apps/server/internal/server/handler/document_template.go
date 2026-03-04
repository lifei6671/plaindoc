package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
)

type documentTemplateHandler struct {
	documentTemplateService *service.DocumentTemplateService
}

type documentTemplateSummaryResponse struct {
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

type documentTemplateListResponse struct {
	Items      []documentTemplateSummaryResponse `json:"items"`
	Pagination struct {
		Page     int   `json:"page"`
		PageSize int   `json:"pageSize"`
		Total    int64 `json:"total"`
	} `json:"pagination"`
}

type documentTemplateDetailResponse struct {
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

// NewDocumentTemplateHandler 创建文档模板处理器。
func NewDocumentTemplateHandler(documentTemplateService *service.DocumentTemplateService) *documentTemplateHandler {
	return &documentTemplateHandler{documentTemplateService: documentTemplateService}
}

// ListTemplates 返回已启用模板列表。
func (h *documentTemplateHandler) ListTemplates(c *gin.Context) {
	if h == nil || h.documentTemplateService == nil {
		response.InternalError(c)
		return
	}

	page, err := parseDocumentTemplateQueryInt(c.Query("page"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidPage, "page 参数不合法")
		return
	}
	pageSize, err := parseDocumentTemplateQueryInt(c.Query("pageSize"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidPageSize, "pageSize 参数不合法")
		return
	}

	result, serviceErr := h.documentTemplateService.ListEnabledTemplates(c.Request.Context(), service.ListDocumentTemplatesInput{
		SceneKey: c.Query("sceneKey"),
		Keyword:  c.Query("keyword"),
		Page:     page,
		PageSize: pageSize,
	})
	if serviceErr != nil {
		switch {
		case errors.Is(serviceErr, service.ErrDocumentTemplateInvalidPage):
			response.Error(c, http.StatusBadRequest, response.CodeInvalidPage, "page 参数不合法")
		case errors.Is(serviceErr, service.ErrDocumentTemplateInvalidSize):
			response.Error(c, http.StatusBadRequest, response.CodeInvalidPageSize, "pageSize 参数不合法")
		case errors.Is(serviceErr, service.ErrDocumentTemplateInvalidScene):
			response.Error(c, http.StatusBadRequest, response.CodeInvalidRequest, "sceneKey 参数不合法")
		case errors.Is(serviceErr, service.ErrDocumentTemplateInvalidSearch):
			response.Error(c, http.StatusBadRequest, response.CodeInvalidRequest, "keyword 参数不合法")
		default:
			response.InternalError(c)
		}
		return
	}

	payload := documentTemplateListResponse{
		Items: make([]documentTemplateSummaryResponse, 0, len(result.Items)),
	}
	payload.Pagination.Page = result.Page
	payload.Pagination.PageSize = result.PageSize
	payload.Pagination.Total = result.Total
	for _, item := range result.Items {
		payload.Items = append(payload.Items, documentTemplateSummaryResponse{
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

	response.JSON(c, http.StatusOK, payload)
}

// GetTemplate 返回已启用模板详情。
func (h *documentTemplateHandler) GetTemplate(c *gin.Context) {
	if h == nil || h.documentTemplateService == nil {
		response.InternalError(c)
		return
	}

	templateID := strings.TrimSpace(c.Param("templateId"))
	if templateID == "" {
		response.Error(c, http.StatusBadRequest, response.CodeInvalidTemplateID, "templateId 参数不能为空")
		return
	}

	item, err := h.documentTemplateService.GetEnabledTemplateByID(c.Request.Context(), templateID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrDocumentTemplateInvalidKey):
			response.Error(c, http.StatusBadRequest, response.CodeInvalidTemplateID, "templateId 参数不合法")
		case errors.Is(err, service.ErrDocumentTemplateNotFound):
			response.Error(c, http.StatusNotFound, response.CodeTemplateNotFound, "模板不存在")
		default:
			response.InternalError(c)
		}
		return
	}

	response.JSON(c, http.StatusOK, documentTemplateDetailResponse{
		TemplateID:      item.TemplateID,
		SceneKey:        item.SceneKey,
		SceneName:       item.SceneName,
		Name:            item.Name,
		Description:     item.Description,
		DefaultTitle:    item.DefaultTitle,
		ContentMD:       item.ContentMD,
		Sort:            item.Sort,
		Builtin:         item.Builtin,
		Enabled:         item.Enabled,
		CreatedByUserID: item.CreatedByUserID,
		UpdatedByUserID: item.UpdatedByUserID,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
	})
}

func parseDocumentTemplateQueryInt(raw string) (int, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, nil
	}
	parsedValue, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	if parsedValue < 0 {
		return 0, errors.New("negative int")
	}
	return parsedValue, nil
}
