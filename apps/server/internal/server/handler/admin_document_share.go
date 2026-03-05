package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/middleware"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
)

type adminDocumentShareHandler struct {
	adminDocumentShareService *service.AdminDocumentShareService
}

type adminDocumentShareResponse struct {
	ShareID          string                   `json:"shareId"`
	DocumentID       string                   `json:"documentId"`
	DocumentRouteKey string                   `json:"documentRouteKey"`
	DocumentTitle    string                   `json:"documentTitle"`
	SpaceID          string                   `json:"spaceId"`
	SpaceName        string                   `json:"spaceName"`
	SpaceOwnerUserID string                   `json:"spaceOwnerUserId"`
	SpaceOwnerName   string                   `json:"spaceOwnerName"`
	SpaceOwnerEmail  string                   `json:"spaceOwnerEmail"`
	Mode             models.DocumentShareMode `json:"mode"`
	PasswordHint     string                   `json:"passwordHint"`
	HasPassword      bool                     `json:"hasPassword"`
	ExpiresAt        *string                  `json:"expiresAt,omitempty"`
	AccessVersion    int                      `json:"accessVersion"`
	CreatedByUserID  *string                  `json:"createdByUserId,omitempty"`
	CreatedByName    string                   `json:"createdByName"`
	CreatedByEmail   string                   `json:"createdByEmail"`
	UpdatedByUserID  *string                  `json:"updatedByUserId,omitempty"`
	CreatedAt        string                   `json:"createdAt"`
	UpdatedAt        string                   `json:"updatedAt"`
	IsExpired        bool                     `json:"isExpired"`
}

type adminDocumentShareListResponse struct {
	Items      []adminDocumentShareResponse `json:"items"`
	Pagination adminDocumentPageResponse    `json:"pagination"`
}

// NewAdminDocumentShareHandler 创建后台分享中心处理器。
func NewAdminDocumentShareHandler(
	adminDocumentShareService *service.AdminDocumentShareService,
) *adminDocumentShareHandler {
	return &adminDocumentShareHandler{
		adminDocumentShareService: adminDocumentShareService,
	}
}

// ListShares 返回后台分享中心列表。
func (h *adminDocumentShareHandler) ListShares(c *gin.Context) {
	if h == nil || h.adminDocumentShareService == nil {
		response.InternalError(c)
		return
	}
	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		setRequestErrmsg(c, err, "解析管理员身份失败")
		response.AdminDocumentShareErrAdminActorMissing.Write(c)
		return
	}

	page, err := parseAdminDocumentQueryInt(c.Query("page"))
	if err != nil {
		response.AdminDocumentShareErrPagePositiveInteger.Write(c)
		return
	}
	pageSize, err := parseAdminDocumentQueryInt(c.Query("pageSize"))
	if err != nil {
		response.AdminDocumentShareErrPageSizePositiveInteger.Write(c)
		return
	}

	view, err := parseAdminDocumentShareView(c.Query("view"))
	if err != nil {
		response.AdminDocumentShareErrRequestBody.Write(c)
		return
	}
	mode, err := parseAdminDocumentShareMode(c.Query("mode"))
	if err != nil {
		response.DocumentShareErrInvalidMode.Write(c)
		return
	}

	payload, err := h.adminDocumentShareService.ListShares(c.Request.Context(), service.ListAdminDocumentSharesInput{
		ActorUserID: actorUserID,
		View:        view,
		Keyword:     strings.TrimSpace(c.Query("keyword")),
		SpaceID:     strings.TrimSpace(c.Query("spaceId")),
		Mode:        mode,
		Expired:     strings.TrimSpace(c.Query("expired")),
		Page:        page,
		PageSize:    pageSize,
	})
	if err != nil {
		setRequestErrmsg(c, err, "查询后台分享列表失败")
		if !writeMappedAdminDocumentShareError(c, err) {
			response.InternalError(c)
		}
		return
	}

	items := make([]adminDocumentShareResponse, 0, len(payload.Items))
	for _, item := range payload.Items {
		items = append(items, mapAdminDocumentShareResponse(item))
	}
	response.JSON(c, http.StatusOK, adminDocumentShareListResponse{
		Items: items,
		Pagination: adminDocumentPageResponse{
			Page:     payload.Page,
			PageSize: payload.PageSize,
			Total:    payload.Total,
		},
	})
}

// UpdateShare 更新后台分享配置。
func (h *adminDocumentShareHandler) UpdateShare(c *gin.Context) {
	if h == nil || h.adminDocumentShareService == nil {
		response.InternalError(c)
		return
	}
	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		setRequestErrmsg(c, err, "解析管理员身份失败")
		response.AdminDocumentShareErrAdminActorMissing.Write(c)
		return
	}

	shareID := strings.TrimSpace(c.Param("shareId"))
	if shareID == "" {
		response.AdminDocumentShareErrShareIDRequired.Write(c)
		return
	}

	var rawPayload map[string]json.RawMessage
	if err := c.ShouldBindJSON(&rawPayload); err != nil {
		setRequestErrmsg(c, err, "解析分享更新请求失败")
		response.AdminDocumentShareErrRequestBody.Write(c)
		return
	}

	input, err := buildUpdateAdminDocumentShareInput(rawPayload)
	if err != nil {
		setRequestErrmsg(c, err, "解析分享更新请求失败")
		response.AdminDocumentShareErrRequestBody.Write(c)
		return
	}
	input.ShareID = shareID
	input.ActorUserID = actorUserID

	updated, err := h.adminDocumentShareService.UpdateShare(c.Request.Context(), input)
	if err != nil {
		setRequestErrmsg(c, err, "更新后台分享失败")
		if !writeMappedAdminDocumentShareError(c, err) {
			response.InternalError(c)
		}
		return
	}
	response.JSON(c, http.StatusOK, mapWorkspaceDocumentShareResponse(updated))
}

// DisableShare 取消后台分享。
func (h *adminDocumentShareHandler) DisableShare(c *gin.Context) {
	if h == nil || h.adminDocumentShareService == nil {
		response.InternalError(c)
		return
	}
	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		setRequestErrmsg(c, err, "解析管理员身份失败")
		response.AdminDocumentShareErrAdminActorMissing.Write(c)
		return
	}

	shareID := strings.TrimSpace(c.Param("shareId"))
	if shareID == "" {
		response.AdminDocumentShareErrShareIDRequired.Write(c)
		return
	}
	if err := h.adminDocumentShareService.DisableShare(c.Request.Context(), shareID, actorUserID); err != nil {
		setRequestErrmsg(c, err, "取消后台分享失败")
		if !writeMappedAdminDocumentShareError(c, err) {
			response.InternalError(c)
		}
		return
	}
	response.JSON(c, http.StatusOK, struct{}{})
}

func parseAdminDocumentShareView(raw string) (repository.DocumentShareAdminView, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "", string(repository.DocumentShareAdminViewAll):
		return repository.DocumentShareAdminViewAll, nil
	case string(repository.DocumentShareAdminViewMine):
		return repository.DocumentShareAdminViewMine, nil
	default:
		return "", errors.New("invalid admin document share view")
	}
}

func parseAdminDocumentShareMode(raw string) (models.DocumentShareMode, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" || value == "all" {
		return "", nil
	}
	mode := models.DocumentShareMode(value)
	if !models.IsValidDocumentShareMode(mode) {
		return "", errors.New("invalid document share mode")
	}
	return mode, nil
}

func buildUpdateAdminDocumentShareInput(
	rawPayload map[string]json.RawMessage,
) (service.UpdateDocumentShareByIDInput, error) {
	input := service.UpdateDocumentShareByIDInput{}
	if rawPayload == nil {
		return input, nil
	}

	if rawMode, ok := rawPayload["mode"]; ok {
		var modeText string
		if err := json.Unmarshal(rawMode, &modeText); err != nil {
			return input, err
		}
		modeValue := models.DocumentShareMode(strings.ToLower(strings.TrimSpace(modeText)))
		input.Mode = &modeValue
	}

	if rawPassword, ok := rawPayload["password"]; ok {
		var passwordText *string
		if string(rawPassword) == "null" {
			empty := ""
			passwordText = &empty
		} else {
			var value string
			if err := json.Unmarshal(rawPassword, &value); err != nil {
				return input, err
			}
			passwordText = &value
		}
		input.Password = passwordText
	}

	if rawPasswordHint, ok := rawPayload["passwordHint"]; ok {
		var value string
		if string(rawPasswordHint) == "null" {
			value = ""
		} else if err := json.Unmarshal(rawPasswordHint, &value); err != nil {
			return input, err
		}
		input.PasswordHint = &value
	}

	if rawExpiresAt, ok := rawPayload["expiresAt"]; ok {
		input.ExpiresAtSet = true
		if string(rawExpiresAt) == "null" {
			input.ExpiresAt = nil
		} else {
			var expiresText string
			if err := json.Unmarshal(rawExpiresAt, &expiresText); err != nil {
				return input, err
			}
			expiresAt, err := parseDocumentShareOptionalTime(&expiresText)
			if err != nil {
				return input, err
			}
			input.ExpiresAt = expiresAt
		}
	}
	return input, nil
}

func writeMappedAdminDocumentShareError(c *gin.Context, err error) bool {
	if writeMappedDocumentShareError(c, err) {
		return true
	}
	if errors.Is(err, service.ErrDocumentShareAccessDenied) {
		response.Error(c, http.StatusForbidden, response.CodeForbidden, "无权管理该分享")
		return true
	}
	return false
}

func mapAdminDocumentShareResponse(value repository.AdminDocumentShareListRecord) adminDocumentShareResponse {
	mode := value.Share.Mode
	if !models.IsValidDocumentShareMode(mode) {
		mode = models.DocumentShareModePublic
	}
	return adminDocumentShareResponse{
		ShareID:          strings.TrimSpace(value.Share.ShareID),
		DocumentID:       strings.TrimSpace(value.Share.DocumentID),
		DocumentRouteKey: strings.TrimSpace(value.DocumentRouteKey),
		DocumentTitle:    strings.TrimSpace(value.DocumentTitle),
		SpaceID:          strings.TrimSpace(value.Share.SpaceID),
		SpaceName:        strings.TrimSpace(value.SpaceName),
		SpaceOwnerUserID: strings.TrimSpace(value.SpaceOwnerID),
		SpaceOwnerName:   strings.TrimSpace(value.SpaceOwnerName),
		SpaceOwnerEmail:  strings.TrimSpace(value.SpaceOwnerEmail),
		Mode:             mode,
		PasswordHint:     strings.TrimSpace(value.Share.PasswordHint),
		HasPassword:      value.Share.PasswordHash != nil && strings.TrimSpace(*value.Share.PasswordHash) != "",
		ExpiresAt:        formatDocumentShareOptionalTime(value.Share.ExpiresAt),
		AccessVersion:    value.Share.AccessVersion,
		CreatedByUserID:  value.Share.CreatedByUserID,
		CreatedByName:    strings.TrimSpace(value.CreatedByName),
		CreatedByEmail:   strings.TrimSpace(value.CreatedByEmail),
		UpdatedByUserID:  value.Share.UpdatedByUserID,
		CreatedAt:        value.Share.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:        value.Share.UpdatedAt.UTC().Format(time.RFC3339Nano),
		IsExpired:        value.IsExpired,
	}
}
