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

type adminAuditHandler struct {
	adminAuditService *service.AdminAuditService
}

type adminAuditLogResponse struct {
	ID          int64          `json:"id"`
	ActorUserID *string        `json:"actorUserId"`
	ActorName   string         `json:"actorName"`
	ActorEmail  string         `json:"actorEmail"`
	Module      string         `json:"module"`
	Action      string         `json:"action"`
	TargetType  string         `json:"targetType"`
	TargetID    string         `json:"targetId"`
	Summary     string         `json:"summary"`
	Detail      map[string]any `json:"detail"`
	RequestID   string         `json:"requestId"`
	CreatedAt   time.Time      `json:"createdAt"`
}

type adminAuditListResponse struct {
	Items      []adminAuditLogResponse   `json:"items"`
	Pagination adminAuditPaginationBlock `json:"pagination"`
}

type adminAuditPaginationBlock struct {
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
	Total    int64 `json:"total"`
}

// NewAdminAuditHandler 创建后台审计查询处理器。
func NewAdminAuditHandler(adminAuditService *service.AdminAuditService) *adminAuditHandler {
	return &adminAuditHandler{adminAuditService: adminAuditService}
}

// ListAudits 返回后台审计日志列表，支持条件筛选与分页。
func (h *adminAuditHandler) ListAudits(c *gin.Context) {
	if h == nil || h.adminAuditService == nil {
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

	createdAtFrom, err := parseAdminAuditQueryTime(c.Query("from"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_FROM", "from must be RFC3339 datetime")
		return
	}
	createdAtTo, err := parseAdminAuditQueryTime(c.Query("to"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_TO", "to must be RFC3339 datetime")
		return
	}

	payload, err := h.adminAuditService.ListAudits(c.Request.Context(), service.ListAdminAuditsInput{
		ActorUserID:       actorUserID,
		Keyword:           strings.TrimSpace(c.Query("keyword")),
		ModuleFilter:      strings.TrimSpace(c.Query("module")),
		ActionFilter:      strings.TrimSpace(c.Query("action")),
		ActorUserIDFilter: strings.TrimSpace(c.Query("actorUserId")),
		TargetTypeFilter:  strings.TrimSpace(c.Query("targetType")),
		TargetIDFilter:    strings.TrimSpace(c.Query("targetId")),
		RequestIDFilter:   strings.TrimSpace(c.Query("requestId")),
		CreatedAtFrom:     createdAtFrom,
		CreatedAtTo:       createdAtTo,
		Page:              page,
		PageSize:          pageSize,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAdminForbidden):
			response.Error(c, http.StatusForbidden, "FORBIDDEN", "insufficient admin permission")
		case errors.Is(err, service.ErrAdminAuditInvalidModuleFilter):
			response.Error(c, http.StatusBadRequest, "INVALID_MODULE", "module filter is invalid")
		case errors.Is(err, service.ErrAdminAuditInvalidActionFilter):
			response.Error(c, http.StatusBadRequest, "INVALID_ACTION", "action filter is invalid")
		case errors.Is(err, service.ErrAdminAuditInvalidTimeRange):
			response.Error(c, http.StatusBadRequest, "INVALID_TIME_RANGE", "from must be before to")
		default:
			response.InternalError(c)
		}
		return
	}

	items := make([]adminAuditLogResponse, 0, len(payload.Items))
	for _, item := range payload.Items {
		items = append(items, adminAuditLogResponse{
			ID:          item.ID,
			ActorUserID: item.ActorUserID,
			ActorName:   item.ActorName,
			ActorEmail:  item.ActorEmail,
			Module:      item.Module,
			Action:      item.Action,
			TargetType:  item.TargetType,
			TargetID:    item.TargetID,
			Summary:     item.Summary,
			Detail:      item.Detail,
			RequestID:   item.RequestID,
			CreatedAt:   item.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, adminAuditListResponse{
		Items: items,
		Pagination: adminAuditPaginationBlock{
			Page:     payload.Page,
			PageSize: payload.PageSize,
			Total:    payload.Total,
		},
	})
}

func parseAdminAuditQueryTime(rawValue string) (*time.Time, error) {
	value := strings.TrimSpace(rawValue)
	if value == "" {
		return nil, nil
	}
	parsedAt, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil, err
	}
	utcAt := parsedAt.UTC()
	return &utcAt, nil
}
