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

type adminDocumentHandler struct {
	adminDocumentService *service.AdminDocumentService
}

type adminDocumentResponse struct {
	DocumentID       string              `json:"documentId"`
	NodeID           string              `json:"nodeId"`
	Title            string              `json:"title"`
	SpaceID          string              `json:"spaceId"`
	SpaceName        string              `json:"spaceName"`
	SpaceOwnerUserID string              `json:"spaceOwnerUserId"`
	SpaceOwnerName   string              `json:"spaceOwnerName"`
	SpaceOwnerEmail  string              `json:"spaceOwnerEmail"`
	Visibility       models.Visibility   `json:"visibility"`
	Status           models.EntityStatus `json:"status"`
	BannedReason     string              `json:"bannedReason"`
	BannedAt         *time.Time          `json:"bannedAt"`
	DeletedAt        *time.Time          `json:"deletedAt"`
	CreatedAt        time.Time           `json:"createdAt"`
	UpdatedAt        time.Time           `json:"updatedAt"`
}

type adminDocumentListResponse struct {
	Items      []adminDocumentResponse   `json:"items"`
	Pagination adminDocumentPageResponse `json:"pagination"`
}

type adminDocumentPageResponse struct {
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
	Total    int64 `json:"total"`
}

type updateAdminDocumentStatusRequest struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

// NewAdminDocumentHandler 创建后台文档管理处理器。
func NewAdminDocumentHandler(adminDocumentService *service.AdminDocumentService) *adminDocumentHandler {
	return &adminDocumentHandler{adminDocumentService: adminDocumentService}
}

// ListDocuments 返回后台文档列表，支持关键词、空间、状态、可见性与分页查询。
func (h *adminDocumentHandler) ListDocuments(c *gin.Context) {
	if h == nil || h.adminDocumentService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "admin actor is missing")
		return
	}

	page, err := parseAdminDocumentQueryInt(c.Query("page"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_PAGE", "page must be a positive integer")
		return
	}
	pageSize, err := parseAdminDocumentQueryInt(c.Query("pageSize"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_PAGE_SIZE", "pageSize must be a positive integer")
		return
	}

	payload, err := h.adminDocumentService.ListDocuments(c.Request.Context(), service.ListAdminDocumentsInput{
		ActorUserID:      actorUserID,
		Keyword:          strings.TrimSpace(c.Query("keyword")),
		SpaceID:          strings.TrimSpace(c.Query("spaceId")),
		StatusFilter:     service.AdminDocumentStatusFilter(c.Query("status")),
		VisibilityFilter: service.AdminDocumentVisibilityFilter(c.Query("visibility")),
		Page:             page,
		PageSize:         pageSize,
	})
	if err != nil {
		response.FromError(c, err)
		return
	}

	items := make([]adminDocumentResponse, 0, len(payload.Items))
	for _, item := range payload.Items {
		items = append(items, mapAdminDocumentResponse(item))
	}

	response.JSON(c, http.StatusOK, adminDocumentListResponse{
		Items: items,
		Pagination: adminDocumentPageResponse{
			Page:     payload.Page,
			PageSize: payload.PageSize,
			Total:    payload.Total,
		},
	})
}

// UpdateStatus 更新文档状态（active/banned）。
func (h *adminDocumentHandler) UpdateStatus(c *gin.Context) {
	if h == nil || h.adminDocumentService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "admin actor is missing")
		return
	}

	documentID := strings.TrimSpace(c.Param("documentId"))
	if documentID == "" {
		response.Error(c, http.StatusBadRequest, "INVALID_DOCUMENT_ID", "document id is required")
		return
	}

	var req updateAdminDocumentStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	payload, err := h.adminDocumentService.UpdateStatus(c.Request.Context(), service.UpdateAdminDocumentStatusInput{
		ActorUserID: actorUserID,
		RequestID:   response.RequestIDFromContext(c),
		DocumentID:  documentID,
		Status:      models.EntityStatus(strings.ToLower(strings.TrimSpace(req.Status))),
		Reason:      strings.TrimSpace(req.Reason),
	})
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, mapAdminDocumentResponse(payload))
}

// DeleteDocument 软删除文档。
func (h *adminDocumentHandler) DeleteDocument(c *gin.Context) {
	if h == nil || h.adminDocumentService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "admin actor is missing")
		return
	}

	documentID := strings.TrimSpace(c.Param("documentId"))
	if documentID == "" {
		response.Error(c, http.StatusBadRequest, "INVALID_DOCUMENT_ID", "document id is required")
		return
	}

	if err := h.adminDocumentService.DeleteDocument(
		c.Request.Context(),
		actorUserID,
		documentID,
		response.RequestIDFromContext(c),
	); err != nil {
		response.FromError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, struct{}{})
}

func parseAdminDocumentQueryInt(rawValue string) (int, error) {
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

func mapAdminDocumentResponse(value service.AdminDocumentRecord) adminDocumentResponse {
	return adminDocumentResponse{
		DocumentID:       value.DocumentID,
		NodeID:           value.NodeID,
		Title:            value.Title,
		SpaceID:          value.SpaceID,
		SpaceName:        value.SpaceName,
		SpaceOwnerUserID: value.SpaceOwnerUserID,
		SpaceOwnerName:   value.SpaceOwnerName,
		SpaceOwnerEmail:  value.SpaceOwnerEmail,
		Visibility:       value.Visibility,
		Status:           value.Status,
		BannedReason:     value.BannedReason,
		BannedAt:         value.BannedAt,
		DeletedAt:        value.DeletedAt,
		CreatedAt:        value.CreatedAt,
		UpdatedAt:        value.UpdatedAt,
	}
}
