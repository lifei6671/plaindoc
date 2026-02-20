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

type adminThemeHandler struct {
	adminThemeService *service.AdminThemeService
}

type adminThemeResponse struct {
	ThemeID            string            `json:"themeId"`
	Name               string            `json:"name"`
	Description        string            `json:"description"`
	Variables          map[string]string `json:"variables"`
	SyntaxTheme        string            `json:"syntaxTheme"`
	CodeBlockStyle     map[string]any    `json:"codeBlockStyle"`
	CodeBlockCodeStyle map[string]any    `json:"codeBlockCodeStyle"`
	InlineCodeStyle    map[string]any    `json:"inlineCodeStyle"`
	CustomCSS          string            `json:"customCss"`
	Builtin            bool              `json:"builtin"`
	Enabled            bool              `json:"enabled"`
	CreatedAt          time.Time         `json:"createdAt"`
	UpdatedAt          time.Time         `json:"updatedAt"`
}

type createAdminThemeRequest struct {
	ThemeID            string            `json:"themeId"`
	Name               string            `json:"name"`
	Description        string            `json:"description"`
	Variables          map[string]string `json:"variables"`
	SyntaxTheme        string            `json:"syntaxTheme"`
	CodeBlockStyle     map[string]any    `json:"codeBlockStyle"`
	CodeBlockCodeStyle map[string]any    `json:"codeBlockCodeStyle"`
	InlineCodeStyle    map[string]any    `json:"inlineCodeStyle"`
	CustomCSS          string            `json:"customCss"`
	Enabled            *bool             `json:"enabled"`
}

type updateAdminThemeRequest struct {
	Name               *string            `json:"name"`
	Description        *string            `json:"description"`
	Variables          *map[string]string `json:"variables"`
	SyntaxTheme        *string            `json:"syntaxTheme"`
	CodeBlockStyle     *map[string]any    `json:"codeBlockStyle"`
	CodeBlockCodeStyle *map[string]any    `json:"codeBlockCodeStyle"`
	InlineCodeStyle    *map[string]any    `json:"inlineCodeStyle"`
	CustomCSS          *string            `json:"customCss"`
	Enabled            *bool              `json:"enabled"`
}

// NewAdminThemeHandler 创建后台主题管理处理器。
func NewAdminThemeHandler(adminThemeService *service.AdminThemeService) *adminThemeHandler {
	return &adminThemeHandler{adminThemeService: adminThemeService}
}

// ListThemes 返回后台主题列表。
func (h *adminThemeHandler) ListThemes(c *gin.Context) {
	if h == nil || h.adminThemeService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "admin actor is missing")
		return
	}

	items, err := h.adminThemeService.ListThemes(c.Request.Context(), actorUserID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAdminForbidden):
			response.Error(c, http.StatusForbidden, "FORBIDDEN", "admin role is required")
		default:
			response.InternalError(c)
		}
		return
	}

	payload := make([]adminThemeResponse, 0, len(items))
	for _, item := range items {
		payload = append(payload, mapAdminThemeResponse(item))
	}
	c.JSON(http.StatusOK, payload)
}

// CreateTheme 创建自定义主题。
func (h *adminThemeHandler) CreateTheme(c *gin.Context) {
	if h == nil || h.adminThemeService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "admin actor is missing")
		return
	}

	var req createAdminThemeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	item, err := h.adminThemeService.CreateTheme(c.Request.Context(), service.CreateAdminThemeInput{
		ActorUserID:        actorUserID,
		RequestID:          response.RequestIDFromContext(c),
		ThemeID:            req.ThemeID,
		Name:               req.Name,
		Description:        req.Description,
		Variables:          req.Variables,
		SyntaxTheme:        req.SyntaxTheme,
		CodeBlockStyle:     req.CodeBlockStyle,
		CodeBlockCodeStyle: req.CodeBlockCodeStyle,
		InlineCodeStyle:    req.InlineCodeStyle,
		CustomCSS:          req.CustomCSS,
		Enabled:            enabled,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAdminForbidden):
			response.Error(c, http.StatusForbidden, "FORBIDDEN", "admin role is required")
		case errors.Is(err, service.ErrAdminThemeInvalidThemeID):
			response.Error(c, http.StatusBadRequest, "INVALID_THEME_ID", "theme id is invalid")
		case errors.Is(err, service.ErrAdminThemeInvalidName):
			response.Error(c, http.StatusBadRequest, "INVALID_NAME", "theme name is invalid")
		case errors.Is(err, service.ErrAdminThemeInvalidSyntax):
			response.Error(c, http.StatusBadRequest, "INVALID_SYNTAX_THEME", "syntax theme must be one-light or one-dark")
		case errors.Is(err, service.ErrAdminThemeAlreadyExists):
			response.Error(c, http.StatusConflict, "THEME_ALREADY_EXISTS", "theme id already exists")
		default:
			response.InternalError(c)
		}
		return
	}

	c.JSON(http.StatusCreated, mapAdminThemeResponse(item))
}

// UpdateTheme 更新主题内容与启停状态。
func (h *adminThemeHandler) UpdateTheme(c *gin.Context) {
	if h == nil || h.adminThemeService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "admin actor is missing")
		return
	}

	themeID := strings.TrimSpace(c.Param("themeId"))
	if themeID == "" {
		response.Error(c, http.StatusBadRequest, "INVALID_THEME_ID", "theme id is required")
		return
	}

	var req updateAdminThemeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	item, err := h.adminThemeService.UpdateTheme(c.Request.Context(), service.UpdateAdminThemeInput{
		ActorUserID:        actorUserID,
		RequestID:          response.RequestIDFromContext(c),
		ThemeID:            themeID,
		Name:               req.Name,
		Description:        req.Description,
		Variables:          req.Variables,
		SyntaxTheme:        req.SyntaxTheme,
		CodeBlockStyle:     req.CodeBlockStyle,
		CodeBlockCodeStyle: req.CodeBlockCodeStyle,
		InlineCodeStyle:    req.InlineCodeStyle,
		CustomCSS:          req.CustomCSS,
		Enabled:            req.Enabled,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAdminForbidden):
			response.Error(c, http.StatusForbidden, "FORBIDDEN", "admin role is required")
		case errors.Is(err, service.ErrAdminThemeInvalidThemeID):
			response.Error(c, http.StatusBadRequest, "INVALID_THEME_ID", "theme id is invalid")
		case errors.Is(err, service.ErrAdminThemeInvalidName):
			response.Error(c, http.StatusBadRequest, "INVALID_NAME", "theme name is invalid")
		case errors.Is(err, service.ErrAdminThemeInvalidSyntax):
			response.Error(c, http.StatusBadRequest, "INVALID_SYNTAX_THEME", "syntax theme must be one-light or one-dark")
		case errors.Is(err, service.ErrAdminThemeNoChanges):
			response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "theme update changes are required")
		case errors.Is(err, service.ErrAdminThemeNotFound):
			response.Error(c, http.StatusNotFound, "THEME_NOT_FOUND", "theme not found")
		case errors.Is(err, service.ErrAdminThemeBuiltinImmutable):
			response.Error(c, http.StatusBadRequest, "THEME_BUILTIN_IMMUTABLE", "builtin theme can not be modified")
		default:
			response.InternalError(c)
		}
		return
	}

	c.JSON(http.StatusOK, mapAdminThemeResponse(item))
}

// DeleteTheme 删除自定义主题。
func (h *adminThemeHandler) DeleteTheme(c *gin.Context) {
	if h == nil || h.adminThemeService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "admin actor is missing")
		return
	}

	themeID := strings.TrimSpace(c.Param("themeId"))
	if themeID == "" {
		response.Error(c, http.StatusBadRequest, "INVALID_THEME_ID", "theme id is required")
		return
	}

	if err := h.adminThemeService.DeleteTheme(
		c.Request.Context(),
		actorUserID,
		themeID,
		response.RequestIDFromContext(c),
	); err != nil {
		switch {
		case errors.Is(err, service.ErrAdminForbidden):
			response.Error(c, http.StatusForbidden, "FORBIDDEN", "admin role is required")
		case errors.Is(err, service.ErrAdminThemeInvalidThemeID):
			response.Error(c, http.StatusBadRequest, "INVALID_THEME_ID", "theme id is invalid")
		case errors.Is(err, service.ErrAdminThemeNotFound):
			response.Error(c, http.StatusNotFound, "THEME_NOT_FOUND", "theme not found")
		case errors.Is(err, service.ErrAdminThemeBuiltinImmutable):
			response.Error(c, http.StatusBadRequest, "THEME_BUILTIN_IMMUTABLE", "builtin theme can not be deleted")
		case errors.Is(err, service.ErrAdminThemeInUse):
			response.Error(c, http.StatusConflict, "THEME_IN_USE", "theme is referenced by documents")
		default:
			response.InternalError(c)
		}
		return
	}

	c.Status(http.StatusNoContent)
}

func mapAdminThemeResponse(value service.AdminThemeRecord) adminThemeResponse {
	return adminThemeResponse{
		ThemeID:            value.ThemeID,
		Name:               value.Name,
		Description:        value.Description,
		Variables:          value.Variables,
		SyntaxTheme:        value.SyntaxTheme,
		CodeBlockStyle:     value.CodeBlockStyle,
		CodeBlockCodeStyle: value.CodeBlockCodeStyle,
		InlineCodeStyle:    value.InlineCodeStyle,
		CustomCSS:          value.CustomCSS,
		Builtin:            value.Builtin,
		Enabled:            value.Enabled,
		CreatedAt:          value.CreatedAt,
		UpdatedAt:          value.UpdatedAt,
	}
}
