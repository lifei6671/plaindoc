package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

type workspaceDocumentShareHandler struct {
	workspace            *workspaceHandler
	documentShareService *service.DocumentShareService
}

type upsertWorkspaceDocumentShareRequest struct {
	Enabled      bool    `json:"enabled"`
	Mode         string  `json:"mode"`
	Password     string  `json:"password"`
	PasswordHint string  `json:"passwordHint"`
	ExpiresAt    *string `json:"expiresAt"`
}

type workspaceDocumentShareResponse struct {
	Enabled       bool                     `json:"enabled"`
	ShareID       string                   `json:"shareId"`
	DocumentID    string                   `json:"documentId"`
	SpaceID       string                   `json:"spaceId"`
	Mode          models.DocumentShareMode `json:"mode"`
	PasswordHint  string                   `json:"passwordHint"`
	HasPassword   bool                     `json:"hasPassword"`
	ExpiresAt     *string                  `json:"expiresAt,omitempty"`
	DisabledAt    *string                  `json:"disabledAt,omitempty"`
	AccessVersion int                      `json:"accessVersion"`
	CreatedAt     string                   `json:"createdAt"`
	UpdatedAt     string                   `json:"updatedAt"`
}

// NewWorkspaceDocumentShareHandler 创建编辑器文档分享配置处理器。
func NewWorkspaceDocumentShareHandler(
	workspace *workspaceHandler,
	documentShareService *service.DocumentShareService,
) *workspaceDocumentShareHandler {
	return &workspaceDocumentShareHandler{
		workspace:            workspace,
		documentShareService: documentShareService,
	}
}

// GetDocumentShare 返回文档当前分享配置。
func (h *workspaceDocumentShareHandler) GetDocumentShare(c *gin.Context) {
	actorUserID, ok := h.requireActorUserID(c)
	if !ok {
		return
	}
	if h == nil || h.workspace == nil || h.documentShareService == nil {
		response.InternalError(c)
		return
	}

	documentID := strings.TrimSpace(c.Param("docId"))
	if documentID == "" {
		response.DocumentShareErrDocumentIDRequired.Write(c)
		return
	}

	spaceID, err := h.ensureDocumentWritable(c, documentID, actorUserID)
	if err != nil {
		h.writeWorkspacePermissionError(c, err)
		return
	}

	config, err := h.documentShareService.GetConfigByDocumentID(c.Request.Context(), documentID)
	if err != nil {
		setRequestErrmsg(c, err, "读取文档分享配置失败")
		if !writeMappedDocumentShareError(c, err) {
			response.InternalError(c)
		}
		return
	}
	if strings.TrimSpace(config.SpaceID) == "" {
		config.SpaceID = spaceID
	}
	response.JSON(c, http.StatusOK, mapWorkspaceDocumentShareResponse(config))
}

// UpsertDocumentShare 创建或更新文档分享配置。
func (h *workspaceDocumentShareHandler) UpsertDocumentShare(c *gin.Context) {
	actorUserID, ok := h.requireActorUserID(c)
	if !ok {
		return
	}
	if h == nil || h.workspace == nil || h.documentShareService == nil {
		response.InternalError(c)
		return
	}

	documentID := strings.TrimSpace(c.Param("docId"))
	if documentID == "" {
		response.DocumentShareErrDocumentIDRequired.Write(c)
		return
	}

	var req upsertWorkspaceDocumentShareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		setRequestErrmsg(c, err, "解析文档分享配置请求失败")
		response.DocumentShareErrInvalidMode.Write(c)
		return
	}

	spaceID, err := h.ensureDocumentWritable(c, documentID, actorUserID)
	if err != nil {
		h.writeWorkspacePermissionError(c, err)
		return
	}

	if !req.Enabled {
		if err := h.documentShareService.DisableByDocumentID(c.Request.Context(), documentID, actorUserID); err != nil {
			setRequestErrmsg(c, err, "取消文档分享失败")
			if !writeMappedDocumentShareError(c, err) {
				response.InternalError(c)
			}
			return
		}
		config, getErr := h.documentShareService.GetConfigByDocumentID(c.Request.Context(), documentID)
		if getErr != nil {
			setRequestErrmsg(c, getErr, "读取文档分享配置失败")
			if !writeMappedDocumentShareError(c, getErr) {
				response.InternalError(c)
			}
			return
		}
		if strings.TrimSpace(config.SpaceID) == "" {
			config.SpaceID = spaceID
		}
		response.JSON(c, http.StatusOK, mapWorkspaceDocumentShareResponse(config))
		return
	}

	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode == "" {
		mode = string(models.DocumentShareModePublic)
	}
	expiresAt, err := parseDocumentShareOptionalTime(req.ExpiresAt)
	if err != nil {
		response.DocumentShareErrInvalidMode.Write(c)
		return
	}

	config, err := h.documentShareService.UpsertByDocumentID(c.Request.Context(), service.UpsertDocumentShareInput{
		DocumentID:   documentID,
		SpaceID:      spaceID,
		ActorUserID:  actorUserID,
		Mode:         models.DocumentShareMode(mode),
		Password:     req.Password,
		PasswordHint: req.PasswordHint,
		ExpiresAt:    expiresAt,
	})
	if err != nil {
		setRequestErrmsg(c, err, "更新文档分享配置失败")
		if !writeMappedDocumentShareError(c, err) {
			response.InternalError(c)
		}
		return
	}
	response.JSON(c, http.StatusOK, mapWorkspaceDocumentShareResponse(config))
}

// DeleteDocumentShare 取消文档分享。
func (h *workspaceDocumentShareHandler) DeleteDocumentShare(c *gin.Context) {
	actorUserID, ok := h.requireActorUserID(c)
	if !ok {
		return
	}
	if h == nil || h.workspace == nil || h.documentShareService == nil {
		response.InternalError(c)
		return
	}

	documentID := strings.TrimSpace(c.Param("docId"))
	if documentID == "" {
		response.DocumentShareErrDocumentIDRequired.Write(c)
		return
	}

	_, err := h.ensureDocumentWritable(c, documentID, actorUserID)
	if err != nil {
		h.writeWorkspacePermissionError(c, err)
		return
	}
	if err := h.documentShareService.DisableByDocumentID(c.Request.Context(), documentID, actorUserID); err != nil {
		setRequestErrmsg(c, err, "取消文档分享失败")
		if !writeMappedDocumentShareError(c, err) {
			response.InternalError(c)
		}
		return
	}
	response.JSON(c, http.StatusOK, struct{}{})
}

func (h *workspaceDocumentShareHandler) requireActorUserID(c *gin.Context) (string, bool) {
	if h == nil || h.workspace == nil {
		response.InternalError(c)
		return "", false
	}
	return h.workspace.requireActorUserID(c)
}

func (h *workspaceDocumentShareHandler) ensureDocumentWritable(
	c *gin.Context,
	documentID string,
	actorUserID string,
) (string, error) {
	if h == nil || h.workspace == nil || h.workspace.workspaceRepo == nil {
		return "", errors.New("workspace document share handler dependencies are nil")
	}
	record, err := h.workspace.workspaceRepo.GetDocumentByDocumentID(c.Request.Context(), documentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", service.ErrDocumentNotFound
		}
		return "", err
	}
	spaceID := strings.TrimSpace(record.SpaceID)
	if _, err := h.workspace.ensureSpaceWritable(c.Request.Context(), spaceID, actorUserID); err != nil {
		return "", err
	}
	return spaceID, nil
}

func (h *workspaceDocumentShareHandler) writeWorkspacePermissionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrDocumentNotFound):
		response.WorkspaceErrDocumentNotFound.Write(c)
	case errors.Is(err, service.ErrSpaceNotFound):
		response.WorkspaceErrSpaceNotFound.Write(c)
	case errors.Is(err, service.ErrSpaceAccessDenied):
		response.WorkspaceErrInsufficientSpacePermission.Write(c)
	default:
		setRequestErrmsg(c, err, "校验文档分享权限失败")
		response.InternalError(c)
	}
}

func writeMappedDocumentShareError(c *gin.Context, err error) bool {
	return response.WriteMappedError(
		c,
		err,
		response.ErrorTemplateMapping{Target: service.ErrDocumentShareNotFound, Template: response.DocumentShareErrShareNotFound},
		response.ErrorTemplateMapping{Target: service.ErrDocumentShareInvalidMode, Template: response.DocumentShareErrInvalidMode},
		response.ErrorTemplateMapping{Target: service.ErrDocumentSharePasswordRequired, Template: response.DocumentShareErrPasswordRequired},
		response.ErrorTemplateMapping{Target: service.ErrDocumentSharePasswordTooShort, Template: response.DocumentShareErrPasswordTooShort},
		response.ErrorTemplateMapping{Target: service.ErrDocumentSharePasswordInvalid, Template: response.DocumentShareErrPasswordInvalid},
		response.ErrorTemplateMapping{Target: service.ErrDocumentShareAccessDenied, Template: response.DocumentShareErrAccessDenied},
	)
}

func mapWorkspaceDocumentShareResponse(value service.DocumentShareConfig) workspaceDocumentShareResponse {
	return workspaceDocumentShareResponse{
		Enabled:       value.Enabled,
		ShareID:       strings.TrimSpace(value.ShareID),
		DocumentID:    strings.TrimSpace(value.DocumentID),
		SpaceID:       strings.TrimSpace(value.SpaceID),
		Mode:          value.Mode,
		PasswordHint:  strings.TrimSpace(value.PasswordHint),
		HasPassword:   value.HasPassword,
		ExpiresAt:     formatDocumentShareOptionalTime(value.ExpiresAt),
		DisabledAt:    formatDocumentShareOptionalTime(value.DisabledAt),
		AccessVersion: value.AccessVersion,
		CreatedAt:     value.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:     value.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func parseDocumentShareOptionalTime(raw *string) (*time.Time, error) {
	if raw == nil {
		return nil, nil
	}
	value := strings.TrimSpace(*raw)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, err
	}
	normalized := parsed.UTC()
	return &normalized, nil
}

func formatDocumentShareOptionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	text := value.UTC().Format(time.RFC3339Nano)
	return &text
}
