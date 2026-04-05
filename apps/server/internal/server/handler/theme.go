package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/rendercache"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
	"gorm.io/gorm"
)

type themeHandler struct {
	db *gorm.DB
}

type documentThemeHandler struct {
	db          *gorm.DB
	renderCache *rendercache.Cache
}

// NewThemeHandler 创建主题查询处理器。
func NewThemeHandler(db *gorm.DB) *themeHandler {
	return &themeHandler{db: db}
}

// NewDocumentThemeHandler 创建文档主题更新处理器。
func NewDocumentThemeHandler(db *gorm.DB, renderCache *rendercache.Cache) *documentThemeHandler {
	return &documentThemeHandler{db: db, renderCache: renderCache}
}

type themeResponse struct {
	ID                 string            `json:"id"`
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
}

type updateDocumentThemeRequest struct {
	ThemeID string `json:"themeId" binding:"required"`
}

type themeRow struct {
	ThemeID                string `gorm:"column:theme_id"`
	Name                   string `gorm:"column:name"`
	Description            string `gorm:"column:description"`
	VariablesJSON          string `gorm:"column:variables_json"`
	SyntaxTheme            string `gorm:"column:syntax_theme"`
	CodeBlockStyleJSON     string `gorm:"column:code_block_style_json"`
	CodeBlockCodeStyleJSON string `gorm:"column:code_block_code_style_json"`
	InlineCodeStyleJSON    string `gorm:"column:inline_code_style_json"`
	CustomCSS              string `gorm:"column:custom_css"`
	IsBuiltin              bool   `gorm:"column:is_builtin"`
	IsEnabled              bool   `gorm:"column:is_enabled"`
}

type documentRow struct {
	DocumentID string `gorm:"column:document_id"`
	NodeID     string `gorm:"column:node_id"`
	ThemeID    string `gorm:"column:theme_id"`
	Title      string `gorm:"column:title"`
	ContentMD  string `gorm:"column:content_md"`
	Version    int    `gorm:"column:version"`
}

type documentResponse struct {
	ID        string `json:"id"`
	NodeID    string `json:"nodeId"`
	ThemeID   string `json:"themeId"`
	Title     string `json:"title"`
	Content   string `json:"contentMd"`
	Version   int    `json:"version"`
	UpdatedAt string `json:"updatedAt"`
}

// List 返回可用主题列表，作为前端主题菜单数据源。
func (h *themeHandler) List(c *gin.Context) {
	if h == nil || h.db == nil {
		response.InternalError(c)
		return
	}

	var themes []themeRow
	if err := h.db.WithContext(c.Request.Context()).
		Table("themes").
		Select("theme_id", "name", "description", "variables_json", "syntax_theme", "code_block_style_json", "code_block_code_style_json", "inline_code_style_json", "custom_css", "is_builtin", "is_enabled").
		Where("is_enabled = ?", true).
		Order("id DESC").
		Find(&themes).Error; err != nil {
		response.InternalError(c)
		return
	}

	payload := make([]themeResponse, 0, len(themes))
	for _, theme := range themes {
		item, err := toThemeResponse(theme)
		if err != nil {
			response.InternalError(c)
			return
		}
		payload = append(payload, item)
	}

	response.JSON(c, http.StatusOK, payload)
}

// UpdateTheme 按文档 ID 绑定主题 ID，并返回最新文档快照。
func (h *documentThemeHandler) UpdateTheme(c *gin.Context) {
	if h == nil || h.db == nil {
		response.InternalError(c)
		return
	}

	documentID := c.Param("docId")
	if documentID == "" {
		response.ThemeErrDocumentIDRequired.Write(c)
		return
	}

	var req updateDocumentThemeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ThemeErrThemeIDRequired.Write(c)
		return
	}

	if req.ThemeID == "" {
		response.ThemeErrInvalidThemeIdThemeIDRequired.Write(c)
		return
	}

	var themeCount int64
	if err := h.db.WithContext(c.Request.Context()).
		Table("themes").
		Where("theme_id = ? AND is_enabled = ?", req.ThemeID, true).
		Count(&themeCount).Error; err != nil {
		response.InternalError(c)
		return
	}
	if themeCount == 0 {
		response.ThemeErrThemeNotFound.Write(c)
		return
	}

	updateResult := h.db.WithContext(c.Request.Context()).
		Table("documents").
		Where("document_id = ?", documentID).
		Updates(map[string]any{
			"theme_id":   req.ThemeID,
			"updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
		})
	if updateResult.Error != nil {
		response.InternalError(c)
		return
	}
	if updateResult.RowsAffected == 0 {
		response.ThemeErrDocumentNotFound.Write(c)
		return
	}

	var document documentRow
	if err := h.db.WithContext(c.Request.Context()).
		Table("documents").
		Select("document_id", "node_id", "theme_id", "title", "content_md", "version").
		Where("document_id = ?", documentID).
		First(&document).Error; err != nil {
		response.InternalError(c)
		return
	}

	if h != nil && h.renderCache != nil {
		// 主题切换会影响阅读页样式，需主动失效渲染缓存。
		h.renderCache.PurgeDoc(document.DocumentID)
	}

	response.JSON(c, http.StatusOK, toDocumentResponse(document, time.Now().UTC().Format(time.RFC3339Nano)))
}

func toThemeResponse(theme themeRow) (themeResponse, error) {
	var rawVariables map[string]any
	if err := json.Unmarshal([]byte(theme.VariablesJSON), &rawVariables); err != nil {
		return themeResponse{}, err
	}
	variables := make(map[string]string, len(rawVariables))
	for key, value := range rawVariables {
		variables[key] = fmt.Sprint(value)
	}

	codeBlockStyle, err := decodeStyleJSON(theme.CodeBlockStyleJSON)
	if err != nil {
		return themeResponse{}, err
	}
	codeBlockCodeStyle, err := decodeStyleJSON(theme.CodeBlockCodeStyleJSON)
	if err != nil {
		return themeResponse{}, err
	}
	inlineCodeStyle, err := decodeStyleJSON(theme.InlineCodeStyleJSON)
	if err != nil {
		return themeResponse{}, err
	}

	return themeResponse{
		ID:                 theme.ThemeID,
		Name:               theme.Name,
		Description:        theme.Description,
		Variables:          variables,
		SyntaxTheme:        theme.SyntaxTheme,
		CodeBlockStyle:     codeBlockStyle,
		CodeBlockCodeStyle: codeBlockCodeStyle,
		InlineCodeStyle:    inlineCodeStyle,
		CustomCSS:          theme.CustomCSS,
		Builtin:            theme.IsBuiltin,
		Enabled:            theme.IsEnabled,
	}, nil
}

func decodeStyleJSON(raw string) (map[string]any, error) {
	return service.DecodeThemeStyleJSON(raw)
}

func toDocumentResponse(document documentRow, updatedAt string) documentResponse {
	return documentResponse{
		ID:        document.DocumentID,
		NodeID:    document.NodeID,
		ThemeID:   document.ThemeID,
		Title:     document.Title,
		Content:   document.ContentMD,
		Version:   document.Version,
		UpdatedAt: updatedAt,
	}
}
