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

type adminUserHandler struct {
	adminUserService *service.AdminUserService
}

type adminUserResponse struct {
	UserID       string              `json:"userId"`
	Email        string              `json:"email"`
	Name         string              `json:"name"`
	Status       models.EntityStatus `json:"status"`
	BannedReason string              `json:"bannedReason"`
	BannedAt     *time.Time          `json:"bannedAt"`
	DeletedAt    *time.Time          `json:"deletedAt"`
	CreatedAt    time.Time           `json:"createdAt"`
	UpdatedAt    time.Time           `json:"updatedAt"`
}

type adminUserListResponse struct {
	Items      []adminUserResponse        `json:"items"`
	Pagination adminUserPaginationPayload `json:"pagination"`
}

type adminUserPaginationPayload struct {
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
	Total    int64 `json:"total"`
}

type updateAdminUserStatusRequest struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

// NewAdminUserHandler 创建后台用户管理处理器。
func NewAdminUserHandler(adminUserService *service.AdminUserService) *adminUserHandler {
	return &adminUserHandler{adminUserService: adminUserService}
}

// ListUsers 返回后台用户列表，支持关键词、状态、分页查询。
func (h *adminUserHandler) ListUsers(c *gin.Context) {
	if h == nil || h.adminUserService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "admin actor is missing")
		return
	}

	page, err := parseQueryInt(c.Query("page"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_PAGE", "page must be a positive integer")
		return
	}
	pageSize, err := parseQueryInt(c.Query("pageSize"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_PAGE_SIZE", "pageSize must be a positive integer")
		return
	}

	payload, err := h.adminUserService.ListUsers(c.Request.Context(), service.ListAdminUsersInput{
		ActorUserID:  actorUserID,
		Keyword:      strings.TrimSpace(c.Query("keyword")),
		StatusFilter: service.AdminUserStatusFilter(c.Query("status")),
		Page:         page,
		PageSize:     pageSize,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAdminForbidden):
			response.Error(c, http.StatusForbidden, "FORBIDDEN", "platform admin role is required")
		case errors.Is(err, service.ErrAdminUserInvalidStatusFilter):
			response.Error(c, http.StatusBadRequest, "INVALID_STATUS", "status filter is invalid")
		default:
			response.InternalError(c)
		}
		return
	}

	items := make([]adminUserResponse, 0, len(payload.Items))
	for _, item := range payload.Items {
		items = append(items, mapAdminUserResponse(item))
	}

	c.JSON(http.StatusOK, adminUserListResponse{
		Items: items,
		Pagination: adminUserPaginationPayload{
			Page:     payload.Page,
			PageSize: payload.PageSize,
			Total:    payload.Total,
		},
	})
}

// UpdateStatus 更新后台目标用户状态（仅支持 active 与 banned）。
func (h *adminUserHandler) UpdateStatus(c *gin.Context) {
	if h == nil || h.adminUserService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "admin actor is missing")
		return
	}

	targetUserID := strings.TrimSpace(c.Param("userId"))
	if targetUserID == "" {
		response.Error(c, http.StatusBadRequest, "INVALID_USER_ID", "user id is required")
		return
	}

	var req updateAdminUserStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	payload, err := h.adminUserService.UpdateUserStatus(c.Request.Context(), service.UpdateAdminUserStatusInput{
		ActorUserID: actorUserID,
		RequestID:   response.RequestIDFromContext(c),
		UserID:      targetUserID,
		Status:      models.EntityStatus(strings.ToLower(strings.TrimSpace(req.Status))),
		Reason:      strings.TrimSpace(req.Reason),
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAdminForbidden):
			response.Error(c, http.StatusForbidden, "FORBIDDEN", "platform admin role is required")
		case errors.Is(err, service.ErrAdminUserNotFound):
			response.Error(c, http.StatusNotFound, "USER_NOT_FOUND", "user not found")
		case errors.Is(err, service.ErrAdminUserInvalidStatus):
			response.Error(c, http.StatusBadRequest, "INVALID_STATUS", "status must be active or banned")
		case errors.Is(err, service.ErrAdminUserBanReasonRequired):
			response.Error(c, http.StatusBadRequest, "INVALID_REASON", "ban reason is required")
		case errors.Is(err, service.ErrAdminUserSelfOperationBlocked):
			response.Error(c, http.StatusBadRequest, "SELF_OPERATION_FORBIDDEN", "self operation is not allowed")
		case errors.Is(err, service.ErrAdminUserAlreadyDeleted):
			response.Error(c, http.StatusBadRequest, "USER_DELETED", "user has been deleted")
		default:
			response.InternalError(c)
		}
		return
	}

	c.JSON(http.StatusOK, mapAdminUserResponse(payload))
}

// DeleteUser 软删除后台目标用户。
func (h *adminUserHandler) DeleteUser(c *gin.Context) {
	if h == nil || h.adminUserService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "admin actor is missing")
		return
	}

	targetUserID := strings.TrimSpace(c.Param("userId"))
	if targetUserID == "" {
		response.Error(c, http.StatusBadRequest, "INVALID_USER_ID", "user id is required")
		return
	}

	if err := h.adminUserService.DeleteUser(
		c.Request.Context(),
		actorUserID,
		targetUserID,
		response.RequestIDFromContext(c),
	); err != nil {
		switch {
		case errors.Is(err, service.ErrAdminForbidden):
			response.Error(c, http.StatusForbidden, "FORBIDDEN", "platform admin role is required")
		case errors.Is(err, service.ErrAdminUserNotFound):
			response.Error(c, http.StatusNotFound, "USER_NOT_FOUND", "user not found")
		case errors.Is(err, service.ErrAdminUserSelfOperationBlocked):
			response.Error(c, http.StatusBadRequest, "SELF_OPERATION_FORBIDDEN", "self operation is not allowed")
		case errors.Is(err, service.ErrAdminUserInvalidUserID):
			response.Error(c, http.StatusBadRequest, "INVALID_USER_ID", "user id is invalid")
		default:
			response.InternalError(c)
		}
		return
	}

	c.Status(http.StatusNoContent)
}

func parseQueryInt(rawValue string) (int, error) {
	trimmedValue := strings.TrimSpace(rawValue)
	if trimmedValue == "" {
		return 0, nil
	}
	parsedValue, err := strconv.Atoi(trimmedValue)
	if err != nil || parsedValue <= 0 {
		return 0, errors.New("invalid positive integer")
	}
	return parsedValue, nil
}

func mapAdminUserResponse(value service.AdminUserRecord) adminUserResponse {
	return adminUserResponse{
		UserID:       value.UserID,
		Email:        value.Email,
		Name:         value.Name,
		Status:       value.Status,
		BannedReason: value.BannedReason,
		BannedAt:     value.BannedAt,
		DeletedAt:    value.DeletedAt,
		CreatedAt:    value.CreatedAt,
		UpdatedAt:    value.UpdatedAt,
	}
}
