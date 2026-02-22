package handler

import (
	"errors"
	"io"
	"mime/multipart"
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
	Description  string              `json:"description"`
	OwnerUserID  string              `json:"ownerUserId"`
	OwnerName    string              `json:"ownerName"`
	OwnerEmail   string              `json:"ownerEmail"`
	Visibility   models.Visibility   `json:"visibility"`
	Cover        *adminSpaceCoverDTO `json:"cover,omitempty"`
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

type adminSpaceCoverDTO struct {
	AssetID    string `json:"assetId,omitempty"`
	Key        string `json:"key"`
	URL        string `json:"url"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	MimeType   string `json:"mimeType"`
	SizeBytes  int64  `json:"sizeBytes,omitempty"`
	Normalized bool   `json:"normalized"`
	Source     string `json:"source"`
}

type createAdminSpaceRequest struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	Visibility   string `json:"visibility"`
	CoverAssetID string `json:"coverAssetId"`
}

// updateAdminSpaceMetadataRequest 支持空间基础信息与封面字段的局部更新。
type updateAdminSpaceMetadataRequest struct {
	Name         *string `json:"name"`
	Description  *string `json:"description"`
	Visibility   *string `json:"visibility"`
	CoverAssetID *string `json:"coverAssetId"`
}

type updateAdminSpaceStatusRequest struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type transferAdminSpaceRequest struct {
	TargetUserID string `json:"targetUserId"`
	TargetEmail  string `json:"targetEmail"`
}

// NewAdminSpaceHandler 创建后台空间管理处理器。
func NewAdminSpaceHandler(adminSpaceService *service.AdminSpaceService) *adminSpaceHandler {
	return &adminSpaceHandler{adminSpaceService: adminSpaceService}
}

// CreateSpace 创建后台空间。
func (h *adminSpaceHandler) CreateSpace(c *gin.Context) {
	if h == nil || h.adminSpaceService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "admin actor is missing")
		return
	}

	var req createAdminSpaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	payload, err := h.adminSpaceService.CreateSpace(c.Request.Context(), service.CreateAdminSpaceInput{
		ActorUserID:  actorUserID,
		RequestID:    response.RequestIDFromContext(c),
		Name:         req.Name,
		Description:  req.Description,
		Visibility:   models.Visibility(strings.ToLower(strings.TrimSpace(req.Visibility))),
		CoverAssetID: strings.TrimSpace(req.CoverAssetID),
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAdminForbidden):
			response.Error(c, http.StatusForbidden, "FORBIDDEN", "admin role is required")
		case errors.Is(err, service.ErrAdminSpaceInvalidName):
			response.Error(c, http.StatusBadRequest, "INVALID_NAME", "space name is invalid")
		case errors.Is(err, service.ErrAdminSpaceInvalidDescription):
			response.Error(c, http.StatusBadRequest, "INVALID_DESCRIPTION", "space description is invalid")
		case errors.Is(err, service.ErrAdminSpaceInvalidVisibility):
			response.Error(c, http.StatusBadRequest, "INVALID_VISIBILITY", "space visibility is invalid")
		case errors.Is(err, service.ErrAdminSpaceCoverAssetNotFound):
			response.Error(c, http.StatusNotFound, "COVER_ASSET_NOT_FOUND", "cover asset not found")
		default:
			response.InternalError(c)
		}
		return
	}

	c.JSON(http.StatusCreated, mapAdminSpaceResponse(payload))
}

// CreateCoverAsset 创建空间封面资产。
func (h *adminSpaceHandler) CreateCoverAsset(c *gin.Context) {
	if h == nil || h.adminSpaceService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "admin actor is missing")
		return
	}

	source := service.AdminSpaceCoverSource(strings.ToLower(strings.TrimSpace(c.PostForm("source"))))
	spaceName := strings.TrimSpace(c.PostForm("spaceName"))
	fileName := ""
	fileContentType := ""
	fileBytes := make([]byte, 0)

	if source == service.AdminSpaceCoverSourceUserUpload {
		fileHeader, err := c.FormFile("file")
		if err != nil || fileHeader == nil {
			response.Error(c, http.StatusBadRequest, "INVALID_UPLOAD_FILE", "file is required")
			return
		}

		fileName = strings.TrimSpace(fileHeader.Filename)
		fileContentType = strings.TrimSpace(fileHeader.Header.Get("Content-Type"))
		content, err := readAdminUploadedFileBytes(fileHeader)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "INVALID_UPLOAD_FILE", "cannot read uploaded file")
			return
		}
		fileBytes = content
	}

	clientWidth, _ := parseAdminSpaceQueryInt(c.PostForm("clientWidth"))
	clientHeight, _ := parseAdminSpaceQueryInt(c.PostForm("clientHeight"))
	clientMimeType := strings.TrimSpace(c.PostForm("clientMimeType"))
	clientProcessed := strings.EqualFold(strings.TrimSpace(c.PostForm("clientProcessed")), "true")

	payload, err := h.adminSpaceService.CreateCoverAsset(c.Request.Context(), service.CreateAdminSpaceCoverAssetInput{
		ActorUserID:      actorUserID,
		Source:           source,
		FileName:         fileName,
		FileContentType:  fileContentType,
		FileBytes:        fileBytes,
		SpaceName:        spaceName,
		ClientWidth:      clientWidth,
		ClientHeight:     clientHeight,
		ClientMimeType:   clientMimeType,
		ClientProcessed:  clientProcessed,
		PreferredQuality: 0,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAdminForbidden):
			response.Error(c, http.StatusForbidden, "FORBIDDEN", "admin role is required")
		case errors.Is(err, service.ErrAdminSpaceInvalidCoverSource):
			response.Error(c, http.StatusBadRequest, "INVALID_SOURCE", "source must be user_upload or system_generated")
		case errors.Is(err, service.ErrAdminSpaceCoverFileRequired):
			response.Error(c, http.StatusBadRequest, "INVALID_UPLOAD_FILE", "file is required")
		case errors.Is(err, service.ErrAdminSpaceCoverSpaceNameRequired):
			response.Error(c, http.StatusBadRequest, "INVALID_SPACE_NAME", "space name is required for system generated cover")
		case errors.Is(err, service.ErrAdminSpaceCoverImageInvalid):
			response.Error(c, http.StatusBadRequest, "INVALID_COVER_IMAGE", "cover image is invalid")
		case errors.Is(err, service.ErrAdminSpaceCoverImageTooLarge):
			response.Error(c, http.StatusRequestEntityTooLarge, "COVER_IMAGE_TOO_LARGE", "cover image exceeds 10MB limit")
		case errors.Is(err, service.ErrAdminSpaceCoverImageTooManyPixels):
			response.Error(c, http.StatusBadRequest, "COVER_IMAGE_TOO_LARGE", "cover image dimensions are too large")
		case errors.Is(err, service.ErrAdminSpaceFontUnavailable):
			response.Error(c, http.StatusServiceUnavailable, "COVER_FONT_UNAVAILABLE", "system cover font is unavailable")
		default:
			response.InternalError(c)
		}
		return
	}

	c.JSON(http.StatusOK, mapAdminSpaceCoverDTO(payload))
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
		Description: req.Description,
		Visibility:  visibility,
		CoverAssetID: req.CoverAssetID,
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
		case errors.Is(err, service.ErrAdminSpaceInvalidDescription):
			response.Error(c, http.StatusBadRequest, "INVALID_DESCRIPTION", "space description is invalid")
		case errors.Is(err, service.ErrAdminSpaceInvalidVisibility):
			response.Error(c, http.StatusBadRequest, "INVALID_VISIBILITY", "space visibility is invalid")
		case errors.Is(err, service.ErrAdminSpaceCoverAssetNotFound):
			response.Error(c, http.StatusNotFound, "COVER_ASSET_NOT_FOUND", "cover asset not found")
		case errors.Is(err, service.ErrAdminSpaceAlreadyDeleted):
			response.Error(c, http.StatusBadRequest, "SPACE_DELETED", "space has been deleted")
		default:
			response.InternalError(c)
		}
		return
	}

	c.JSON(http.StatusOK, mapAdminSpaceResponse(payload))
}

// TransferOwnership 转让空间归属。
func (h *adminSpaceHandler) TransferOwnership(c *gin.Context) {
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

	var req transferAdminSpaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	payload, err := h.adminSpaceService.TransferOwnership(c.Request.Context(), service.TransferAdminSpaceOwnershipInput{
		ActorUserID:  actorUserID,
		RequestID:    response.RequestIDFromContext(c),
		SpaceID:      spaceID,
		TargetUserID: strings.TrimSpace(req.TargetUserID),
		TargetEmail:  strings.TrimSpace(req.TargetEmail),
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAdminForbidden):
			response.Error(c, http.StatusForbidden, "FORBIDDEN", "insufficient space admin permission")
		case errors.Is(err, service.ErrAdminSpaceNotFound):
			response.Error(c, http.StatusNotFound, "SPACE_NOT_FOUND", "space not found")
		case errors.Is(err, service.ErrAdminSpaceAlreadyDeleted):
			response.Error(c, http.StatusBadRequest, "SPACE_DELETED", "space has been deleted")
		case errors.Is(err, service.ErrAdminSpaceTransferTargetRequired):
			response.Error(c, http.StatusBadRequest, "TRANSFER_TARGET_REQUIRED", "transfer target is required")
		case errors.Is(err, service.ErrAdminSpaceTransferTargetNotFound):
			response.Error(c, http.StatusNotFound, "TRANSFER_TARGET_NOT_FOUND", "transfer target not found")
		case errors.Is(err, service.ErrAdminSpaceTransferTargetNotMember):
			response.Error(c, http.StatusBadRequest, "TRANSFER_TARGET_NOT_MEMBER", "transfer target is not a space member")
		case errors.Is(err, service.ErrAdminSpaceTransferToSelf):
			response.Error(c, http.StatusBadRequest, "TRANSFER_TARGET_SELF", "transfer target is current owner")
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
		Description:  value.Description,
		OwnerUserID:  value.OwnerUserID,
		OwnerName:    value.OwnerName,
		OwnerEmail:   value.OwnerEmail,
		Visibility:   value.Visibility,
		Cover:        mapAdminSpaceCoverDTOFromPointer(value.Cover),
		Status:       value.Status,
		BannedReason: value.BannedReason,
		BannedAt:     value.BannedAt,
		DeletedAt:    value.DeletedAt,
		CreatedAt:    value.CreatedAt,
		UpdatedAt:    value.UpdatedAt,
	}
}

func mapAdminSpaceCoverDTO(value service.AdminSpaceCoverAsset) adminSpaceCoverDTO {
	return adminSpaceCoverDTO{
		AssetID:    value.AssetID,
		Key:        value.Key,
		URL:        value.URL,
		Width:      value.Width,
		Height:     value.Height,
		MimeType:   value.MimeType,
		SizeBytes:  value.SizeBytes,
		Normalized: value.Normalized,
		Source:     string(value.Source),
	}
}

func mapAdminSpaceCoverDTOFromPointer(value *service.AdminSpaceCoverAsset) *adminSpaceCoverDTO {
	if value == nil {
		return nil
	}
	payload := mapAdminSpaceCoverDTO(*value)
	return &payload
}

func readAdminUploadedFileBytes(fileHeader *multipart.FileHeader) ([]byte, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}
