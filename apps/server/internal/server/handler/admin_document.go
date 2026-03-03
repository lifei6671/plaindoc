package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/logit"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/middleware"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
)

type adminDocumentHandler struct {
	adminDocumentService *service.AdminDocumentService
	searchIndexService   *service.SearchIndexService
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
func NewAdminDocumentHandler(
	adminDocumentService *service.AdminDocumentService,
	searchIndexServices ...*service.SearchIndexService,
) *adminDocumentHandler {
	var searchIndexService *service.SearchIndexService
	if len(searchIndexServices) > 0 {
		searchIndexService = searchIndexServices[0]
	}

	return &adminDocumentHandler{
		adminDocumentService: adminDocumentService,
		searchIndexService:   searchIndexService,
	}
}

// ListDocuments 返回后台文档列表，支持关键词、空间、状态、可见性与分页查询。
func (h *adminDocumentHandler) ListDocuments(c *gin.Context) {
	if h == nil || h.adminDocumentService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.AdminDocumentErrAdminActorMissing.Write(c)
		return
	}

	page, err := parseAdminDocumentQueryInt(c.Query("page"))
	if err != nil {
		response.AdminDocumentErrPagePositiveInteger.Write(c)
		return
	}
	pageSize, err := parseAdminDocumentQueryInt(c.Query("pageSize"))
	if err != nil {
		response.AdminDocumentErrPageSizePositiveInteger.Write(c)
		return
	}
	logit.SetRequestAttrs(c.Request.Context(),
		logit.Any("actorUserID", actorUserID),
		logit.Any("keyword", c.Query("keyword")),
		logit.Any("spaceId", c.Query("spaceId")),
		logit.Any("status", c.Query("status")),
		logit.Any("visibility", c.Query("visibility")),
		logit.Any("page", page),
		logit.Any("pageSize", pageSize),
	)

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
		response.AdminDocumentErrAdminActorMissing.Write(c)
		return
	}

	documentID := strings.TrimSpace(c.Param("documentId"))
	if documentID == "" {
		response.AdminDocumentErrDocumentIDRequired.Write(c)
		return
	}

	var req updateAdminDocumentStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AdminDocumentErrRequestBody.Write(c)
		return
	}
	logit.SetRequestAttrs(c.Request.Context(),
		logit.Any("documentID", documentID),
		logit.Any("status", req.Status),
		logit.Any("reason", req.Reason),
	)

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

// DeleteDocument 删除文档并触发关联数据清理。
func (h *adminDocumentHandler) DeleteDocument(c *gin.Context) {
	if h == nil || h.adminDocumentService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.AdminDocumentErrAdminActorMissing.Write(c)
		return
	}

	documentID := strings.TrimSpace(c.Param("documentId"))
	if documentID == "" {
		response.AdminDocumentErrDocumentIDRequired.Write(c)
		return
	}
	logit.SetRequestAttrs(c.Request.Context(),
		logit.Any("documentID", documentID),
		logit.Any("actorUserID", actorUserID),
	)

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
