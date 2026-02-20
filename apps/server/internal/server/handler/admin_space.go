package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/middleware"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
)

type adminSpaceHandler struct {
	adminSpaceService *service.AdminSpaceService
}

type adminSpaceResponse struct {
	SpaceID      string              `json:"spaceId"`
	Name         string              `json:"name"`
	OwnerUserID  string              `json:"ownerUserId"`
	OwnerName    string              `json:"ownerName"`
	OwnerEmail   string              `json:"ownerEmail"`
	Visibility   models.Visibility   `json:"visibility"`
	Status       models.EntityStatus `json:"status"`
	BannedReason string              `json:"bannedReason"`
	BannedAt     *time.Time          `json:"bannedAt"`
	DeletedAt    *time.Time          `json:"deletedAt"`
	CreatedAt    time.Time           `json:"createdAt"`
	UpdatedAt    time.Time           `json:"updatedAt"`
}

type adminSpaceListResponse struct {
	Items      []adminSpaceResponse       `json:"items"`
	Pagination adminSpacePaginationResult `json:"pagination"`
}

type adminSpacePaginationResult struct {
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
	Total    int64 `json:"total"`
}

type updateAdminSpaceMetadataRequest struct {
	Name       *string `json:"name"`
	Visibility *string `json:"visibility"`
}

type updateAdminSpaceStatusRequest struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

// NewAdminSpaceHandler 创建后台空间管理处理器。
func NewAdminSpaceHandler(adminSpaceService *service.AdminSpaceService) *adminSpaceHandler {
	return &adminSpaceHandler{adminSpaceService: adminSpaceService}
}

// ListSpaces 返回后台空间列表，支持关键词、状态、可见性与分页筛选。
func (h *adminSpaceHandler) ListSpaces(c *gin.Context) {
	if h == nil || h.adminSpaceService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "admin actor is missing")
		return
	}

	page, err := parseAdminSpaceQueryInt(c.Query("page"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_PAGE", "page must be a positive integer")
		return
	}
	pageSize, err := parseAdminSpaceQueryInt(c.Query("pageSize"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_PAGE_SIZE", "pageSize must be a positive integer")
		return
	}

	payload, err := h.adminSpaceService.ListSpaces(c.Request.Context(), service.ListAdminSpacesInput{
		ActorUserID:      actorUserID,
		Keyword:          strings.TrimSpace(c.Query("keyword")),
		StatusFilter:     service.AdminSpaceStatusFilter(c.Query("status")),
		VisibilityFilter: service.AdminSpaceVisibilityFilter(c.Query("visibility")),
		Page:             page,
		PageSize:         pageSize,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAdminForbidden):
			response.Error(c, http.StatusForbidden, "FORBIDDEN", "admin role is required")
		case errors.Is(err, service.ErrAdminSpaceInvalidStatusFilter):
			response.Error(c, http.StatusBadRequest, "INVALID_STATUS", "status filter is invalid")
		case errors.Is(err, service.ErrAdminSpaceInvalidVisibilityFilter):
			response.Error(c, http.StatusBadRequest, "INVALID_VISIBILITY", "visibility filter is invalid")
		default:
			response.InternalError(c)
		}
		return
	}

	items := make([]adminSpaceResponse, 0, len(payload.Items))
	for _, item := range payload.Items {
		items = append(items, mapAdminSpaceResponse(item))
	}

	c.JSON(http.StatusOK, adminSpaceListResponse{
		Items: items,
		Pagination: adminSpacePaginationResult{
			Page:     payload.Page,
			PageSize: payload.PageSize,
			Total:    payload.Total,
		},
	})
}

// UpdateMetadata 更新空间名称、可见性等元数据。
func (h *adminSpaceHandler) UpdateMetadata(c *gin.Context) {
	if h == nil || h.adminSpaceService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "admin actor is missing")
		return
	}

	spaceID := strings.TrimSpace(c.Param("spaceId"))
	if spaceID == "" {
		response.Error(c, http.StatusBadRequest, "INVALID_SPACE_ID", "space id is required")
		return
	}

	var req updateAdminSpaceMetadataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	var visibility *models.Visibility
	if req.Visibility != nil {
		normalizedVisibility := models.Visibility(strings.ToLower(strings.TrimSpace(*req.Visibility)))
		visibility = &normalizedVisibility
	}

	payload, err := h.adminSpaceService.UpdateMetadata(c.Request.Context(), service.UpdateAdminSpaceMetadataInput{
		ActorUserID: actorUserID,
		RequestID:   response.RequestIDFromContext(c),
		SpaceID:     spaceID,
		Name:        req.Name,
		Visibility:  visibility,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAdminForbidden):
			response.Error(c, http.StatusForbidden, "FORBIDDEN", "insufficient space admin permission")
		case errors.Is(err, service.ErrAdminSpaceNotFound):
			response.Error(c, http.StatusNotFound, "SPACE_NOT_FOUND", "space not found")
		case errors.Is(err, service.ErrAdminSpaceNoMetadataChange):
			response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "metadata change is required")
		case errors.Is(err, service.ErrAdminSpaceInvalidName):
			response.Error(c, http.StatusBadRequest, "INVALID_NAME", "space name is invalid")
		case errors.Is(err, service.ErrAdminSpaceInvalidVisibility):
			response.Error(c, http.StatusBadRequest, "INVALID_VISIBILITY", "space visibility is invalid")
		case errors.Is(err, service.ErrAdminSpaceAlreadyDeleted):
			response.Error(c, http.StatusBadRequest, "SPACE_DELETED", "space has been deleted")
		default:
			response.InternalError(c)
		}
		return
	}

	c.JSON(http.StatusOK, mapAdminSpaceResponse(payload))
}

// UpdateStatus 更新空间状态（active/banned）。
func (h *adminSpaceHandler) UpdateStatus(c *gin.Context) {
	if h == nil || h.adminSpaceService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "admin actor is missing")
		return
	}

	spaceID := strings.TrimSpace(c.Param("spaceId"))
	if spaceID == "" {
		response.Error(c, http.StatusBadRequest, "INVALID_SPACE_ID", "space id is required")
		return
	}

	var req updateAdminSpaceStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	payload, err := h.adminSpaceService.UpdateStatus(c.Request.Context(), service.UpdateAdminSpaceStatusInput{
		ActorUserID: actorUserID,
		RequestID:   response.RequestIDFromContext(c),
		SpaceID:     spaceID,
		Status:      models.EntityStatus(strings.ToLower(strings.TrimSpace(req.Status))),
		Reason:      strings.TrimSpace(req.Reason),
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAdminForbidden):
			response.Error(c, http.StatusForbidden, "FORBIDDEN", "insufficient space admin permission")
		case errors.Is(err, service.ErrAdminSpaceNotFound):
			response.Error(c, http.StatusNotFound, "SPACE_NOT_FOUND", "space not found")
		case errors.Is(err, service.ErrAdminSpaceInvalidStatus):
			response.Error(c, http.StatusBadRequest, "INVALID_STATUS", "status must be active or banned")
		case errors.Is(err, service.ErrAdminSpaceBanReasonRequired):
			response.Error(c, http.StatusBadRequest, "INVALID_REASON", "ban reason is required")
		case errors.Is(err, service.ErrAdminSpaceAlreadyDeleted):
			response.Error(c, http.StatusBadRequest, "SPACE_DELETED", "space has been deleted")
		default:
			response.InternalError(c)
		}
		return
	}

	c.JSON(http.StatusOK, mapAdminSpaceResponse(payload))
}

// DeleteSpace 软删除空间。
func (h *adminSpaceHandler) DeleteSpace(c *gin.Context) {
	if h == nil || h.adminSpaceService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "admin actor is missing")
		return
	}

	spaceID := strings.TrimSpace(c.Param("spaceId"))
	if spaceID == "" {
		response.Error(c, http.StatusBadRequest, "INVALID_SPACE_ID", "space id is required")
		return
	}

	if err := h.adminSpaceService.DeleteSpace(
		c.Request.Context(),
		actorUserID,
		spaceID,
		response.RequestIDFromContext(c),
	); err != nil {
		switch {
		case errors.Is(err, service.ErrAdminForbidden):
			response.Error(c, http.StatusForbidden, "FORBIDDEN", "insufficient space admin permission")
		case errors.Is(err, service.ErrAdminSpaceNotFound):
			response.Error(c, http.StatusNotFound, "SPACE_NOT_FOUND", "space not found")
		case errors.Is(err, service.ErrAdminSpaceInvalidSpaceID):
			response.Error(c, http.StatusBadRequest, "INVALID_SPACE_ID", "space id is invalid")
		default:
			response.InternalError(c)
		}
		return
	}

	c.Status(http.StatusNoContent)
}

func parseAdminSpaceQueryInt(rawValue string) (int, error) {
	value := strings.TrimSpace(rawValue)
	if value == "" {
		return 0, nil
	}
	parsedValue, err := strconv.Atoi(value)
	if err != nil || parsedValue <= 0 {
		return 0, errors.New("invalid positive integer")
	}
	return parsedValue, nil
}

func mapAdminSpaceResponse(value service.AdminSpaceRecord) adminSpaceResponse {
	return adminSpaceResponse{
		SpaceID:      value.SpaceID,
		Name:         value.Name,
		OwnerUserID:  value.OwnerUserID,
		OwnerName:    value.OwnerName,
		OwnerEmail:   value.OwnerEmail,
		Visibility:   value.Visibility,
		Status:       value.Status,
		BannedReason: value.BannedReason,
		BannedAt:     value.BannedAt,
		DeletedAt:    value.DeletedAt,
		CreatedAt:    value.CreatedAt,
		UpdatedAt:    value.UpdatedAt,
	}
}
