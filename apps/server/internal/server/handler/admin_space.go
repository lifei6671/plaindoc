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
	"github.com/lifei6671/plaindoc/apps/server/internal/logit"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/middleware"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
)

type adminSpaceHandler struct {
	adminSpaceService *service.AdminSpaceService
}

type adminSpaceResponse struct {
	SpaceID           string              `json:"spaceId"`
	Name              string              `json:"name"`
	Description       string              `json:"description"`
	CategoryID        string              `json:"categoryId"`
	Category          string              `json:"category"`
	CategoryIsDefault bool                `json:"categoryIsDefault"`
	OwnerUserID       string              `json:"ownerUserId"`
	OwnerName         string              `json:"ownerName"`
	OwnerEmail        string              `json:"ownerEmail"`
	Visibility        models.Visibility   `json:"visibility"`
	Cover             *adminSpaceCoverDTO `json:"cover,omitempty"`
	Status            models.EntityStatus `json:"status"`
	BannedReason      string              `json:"bannedReason"`
	BannedAt          *time.Time          `json:"bannedAt"`
	DeletedAt         *time.Time          `json:"deletedAt"`
	CreatedAt         time.Time           `json:"createdAt"`
	UpdatedAt         time.Time           `json:"updatedAt"`
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
	SpaceID      string `json:"spaceId"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	CategoryID   string `json:"categoryId"`
	Visibility   string `json:"visibility"`
	CoverAssetID string `json:"coverAssetId"`
}

// updateAdminSpaceMetadataRequest 支持空间基础信息与封面字段的局部更新。
type updateAdminSpaceMetadataRequest struct {
	Name         *string `json:"name"`
	Description  *string `json:"description"`
	CategoryID   *string `json:"categoryId"`
	Visibility   *string `json:"visibility"`
	CoverAssetID *string `json:"coverAssetId"`
}

type createAdminSpaceCategoryRequest struct {
	Name string `json:"name"`
}

type renameAdminSpaceCategoryRequest struct {
	Name string `json:"name"`
}

type deleteAdminSpaceCategoryResponse struct {
	CategoryID             string `json:"categoryId"`
	ReassignedToCategoryID string `json:"reassignedToCategoryId"`
	ReassignedToName       string `json:"reassignedToName"`
	MovedSpaceCount        int64  `json:"movedSpaceCount"`
}

type adminSpaceCategoryResponse struct {
	CategoryID string    `json:"categoryId"`
	Name       string    `json:"name"`
	IsDefault  bool      `json:"isDefault"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type adminSpaceCategoriesResponse struct {
	Items []adminSpaceCategoryResponse `json:"items"`
}

type updateAdminSpaceStatusRequest struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type transferAdminSpaceRequest struct {
	TargetUserID string `json:"targetUserId"`
	TargetEmail  string `json:"targetEmail"`
}

type upsertAdminSpaceMemberRequest struct {
	TargetUserID string `json:"targetUserId"`
	TargetEmail  string `json:"targetEmail"`
	Role         string `json:"role"`
}

type updateAdminSpaceMemberRoleRequest struct {
	Role string `json:"role"`
}

type adminSpaceMemberResponse struct {
	UserID    string      `json:"userId"`
	Email     string      `json:"email"`
	Name      string      `json:"name"`
	Role      models.Role `json:"role"`
	IsOwner   bool        `json:"isOwner"`
	CreatedAt time.Time   `json:"createdAt"`
	UpdatedAt time.Time   `json:"updatedAt"`
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
		logit.SetRequestAttrs(c.Request.Context(), logit.Any("errmsg", err))
		response.AdminSpaceErrAdminActorMissing.Write(c)
		return
	}

	var req createAdminSpaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logit.SetRequestAttrs(c.Request.Context(), logit.Any("errmsg", err))
		response.AdminSpaceErrRequestBody.Write(c)
		return
	}
	logit.SetRequestAttrs(c.Request.Context(),
		logit.Any("spaceId", req.SpaceID),
		logit.Any("name", req.Name),
		logit.Any("categoryId", req.CategoryID),
		logit.Any("visibility", req.Visibility),
	)

	payload, err := h.adminSpaceService.CreateSpace(c.Request.Context(), service.CreateAdminSpaceInput{
		ActorUserID:  actorUserID,
		RequestID:    response.RequestIDFromContext(c),
		SpaceID:      req.SpaceID,
		Name:         req.Name,
		Description:  req.Description,
		CategoryID:   req.CategoryID,
		Visibility:   models.Visibility(strings.ToLower(strings.TrimSpace(req.Visibility))),
		CoverAssetID: strings.TrimSpace(req.CoverAssetID),
	})
	if err != nil {
		logit.SetRequestAttrs(c.Request.Context(), logit.Any("errmsg", err))
		response.FromError(c, err)
		return
	}

	response.JSON(c, http.StatusCreated, mapAdminSpaceResponse(payload))
}

// CreateCoverAsset 创建空间封面资产。
func (h *adminSpaceHandler) CreateCoverAsset(c *gin.Context) {
	if h == nil || h.adminSpaceService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		logit.SetRequestAttrs(c.Request.Context(), logit.Any("errmsg", err))
		response.AdminSpaceErrAdminActorMissing.Write(c)
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
			logit.SetRequestAttrs(c.Request.Context(), logit.Any("errmsg", err))
			response.AdminSpaceErrFileRequired.Write(c)
			return
		}

		fileName = strings.TrimSpace(fileHeader.Filename)
		fileContentType = strings.TrimSpace(fileHeader.Header.Get("Content-Type"))
		content, err := readAdminUploadedFileBytes(fileHeader)
		if err != nil {
			logit.SetRequestAttrs(c.Request.Context(), logit.Any("errmsg", err))
			response.AdminSpaceErrCannotReadUploadedFile.Write(c)
			return
		}
		fileBytes = content
	}
	logit.SetRequestAttrs(c.Request.Context(),
		logit.Any("source", source),
		logit.Any("spaceName", spaceName),
		logit.Any("fileName", fileName),
		logit.Any("fileContentType", fileContentType),
		logit.Any("clientWidth", c.PostForm("clientWidth")),
		logit.Any("clientHeight", c.PostForm("clientHeight")),
		logit.Any("clientMimeType", c.PostForm("clientMimeType")),
		logit.Any("clientProcessed", c.PostForm("clientProcessed")),
	)

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
		response.FromError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, mapAdminSpaceCoverDTO(payload))
}

// ListSpaces 返回后台空间列表，支持关键词、状态、可见性与分页筛选。
func (h *adminSpaceHandler) ListSpaces(c *gin.Context) {
	if h == nil || h.adminSpaceService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.AdminSpaceErrAdminActorMissing.Write(c)
		return
	}

	page, err := parseAdminSpaceQueryInt(c.Query("page"))
	if err != nil {
		response.AdminSpaceErrPagePositiveInteger.Write(c)
		return
	}
	pageSize, err := parseAdminSpaceQueryInt(c.Query("pageSize"))
	if err != nil {
		response.AdminSpaceErrPageSizePositiveInteger.Write(c)
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
		response.FromError(c, err)
		return
	}

	items := make([]adminSpaceResponse, 0, len(payload.Items))
	for _, item := range payload.Items {
		items = append(items, mapAdminSpaceResponse(item))
	}

	response.JSON(c, http.StatusOK, adminSpaceListResponse{
		Items: items,
		Pagination: adminSpacePaginationResult{
			Page:     payload.Page,
			PageSize: payload.PageSize,
			Total:    payload.Total,
		},
	})
}

// ListCategories 返回后台空间分类配置。
func (h *adminSpaceHandler) ListCategories(c *gin.Context) {
	if h == nil || h.adminSpaceService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.AdminSpaceErrAdminActorMissing.Write(c)
		return
	}

	payload, err := h.adminSpaceService.ListCategories(c.Request.Context(), actorUserID)
	if err != nil {
		response.FromError(c, err)
		return
	}

	items := make([]adminSpaceCategoryResponse, 0, len(payload))
	for _, item := range payload {
		items = append(items, mapAdminSpaceCategoryResponse(item))
	}

	response.JSON(c, http.StatusOK, adminSpaceCategoriesResponse{
		Items: items,
	})
}

// CreateCategory 新增后台空间分类。
func (h *adminSpaceHandler) CreateCategory(c *gin.Context) {
	if h == nil || h.adminSpaceService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.AdminSpaceErrAdminActorMissing.Write(c)
		return
	}

	var req createAdminSpaceCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AdminSpaceErrRequestBody.Write(c)
		return
	}

	payload, err := h.adminSpaceService.CreateCategory(c.Request.Context(), service.CreateAdminSpaceCategoryInput{
		ActorUserID: actorUserID,
		RequestID:   response.RequestIDFromContext(c),
		Name:        req.Name,
	})
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, mapAdminSpaceCategoryResponse(payload))
}

// RenameCategory 重命名后台空间分类。
func (h *adminSpaceHandler) RenameCategory(c *gin.Context) {
	if h == nil || h.adminSpaceService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.AdminSpaceErrAdminActorMissing.Write(c)
		return
	}

	categoryID := strings.TrimSpace(c.Param("categoryId"))
	if categoryID == "" {
		response.AdminSpaceErrSpaceCategoryID.Write(c)
		return
	}

	var req renameAdminSpaceCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AdminSpaceErrRequestBody.Write(c)
		return
	}

	payload, err := h.adminSpaceService.RenameCategory(c.Request.Context(), service.RenameAdminSpaceCategoryInput{
		ActorUserID: actorUserID,
		RequestID:   response.RequestIDFromContext(c),
		CategoryID:  categoryID,
		Name:        req.Name,
	})
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, mapAdminSpaceCategoryResponse(payload))
}

// DeleteCategory 删除后台空间分类并迁移关联空间到“未分类”。
func (h *adminSpaceHandler) DeleteCategory(c *gin.Context) {
	if h == nil || h.adminSpaceService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.AdminSpaceErrAdminActorMissing.Write(c)
		return
	}

	categoryID := strings.TrimSpace(c.Param("categoryId"))
	if categoryID == "" {
		response.AdminSpaceErrSpaceCategoryID.Write(c)
		return
	}

	payload, err := h.adminSpaceService.DeleteCategory(c.Request.Context(), service.DeleteAdminSpaceCategoryInput{
		ActorUserID: actorUserID,
		RequestID:   response.RequestIDFromContext(c),
		CategoryID:  categoryID,
	})
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, deleteAdminSpaceCategoryResponse{
		CategoryID:             payload.CategoryID,
		ReassignedToCategoryID: payload.ReassignedToCategoryID,
		ReassignedToName:       payload.ReassignedToName,
		MovedSpaceCount:        payload.MovedSpaceCount,
	})
}

// ListMembers 返回后台空间成员列表。
func (h *adminSpaceHandler) ListMembers(c *gin.Context) {
	if h == nil || h.adminSpaceService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.AdminSpaceErrAdminActorMissing.Write(c)
		return
	}

	spaceID := strings.TrimSpace(c.Param("spaceId"))
	if spaceID == "" {
		response.AdminSpaceErrSpaceIDRequired.Write(c)
		return
	}

	members, err := h.adminSpaceService.ListMembers(c.Request.Context(), service.ListAdminSpaceMembersInput{
		ActorUserID: actorUserID,
		SpaceID:     spaceID,
	})
	if err != nil {
		response.FromError(c, err)
		return
	}

	items := make([]adminSpaceMemberResponse, 0, len(members))
	for _, member := range members {
		items = append(items, mapAdminSpaceMemberResponse(member))
	}

	response.JSON(c, http.StatusOK, map[string]any{
		"items": items,
	})
}

// UpsertMember 新增空间成员（已存在则更新角色）。
func (h *adminSpaceHandler) UpsertMember(c *gin.Context) {
	if h == nil || h.adminSpaceService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.AdminSpaceErrAdminActorMissing.Write(c)
		return
	}

	spaceID := strings.TrimSpace(c.Param("spaceId"))
	if spaceID == "" {
		response.AdminSpaceErrSpaceIDRequired.Write(c)
		return
	}

	var req upsertAdminSpaceMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AdminSpaceErrRequestBody.Write(c)
		return
	}

	member, err := h.adminSpaceService.UpsertMember(c.Request.Context(), service.UpsertAdminSpaceMemberInput{
		ActorUserID:  actorUserID,
		RequestID:    response.RequestIDFromContext(c),
		SpaceID:      spaceID,
		TargetUserID: strings.TrimSpace(req.TargetUserID),
		TargetEmail:  strings.TrimSpace(req.TargetEmail),
		Role:         models.Role(strings.ToLower(strings.TrimSpace(req.Role))),
	})
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, mapAdminSpaceMemberResponse(member))
}

// UpdateMemberRole 更新空间成员角色。
func (h *adminSpaceHandler) UpdateMemberRole(c *gin.Context) {
	if h == nil || h.adminSpaceService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.AdminSpaceErrAdminActorMissing.Write(c)
		return
	}

	spaceID := strings.TrimSpace(c.Param("spaceId"))
	if spaceID == "" {
		response.AdminSpaceErrSpaceIDRequired.Write(c)
		return
	}
	memberUserID := strings.TrimSpace(c.Param("userId"))
	if memberUserID == "" {
		response.AdminSpaceErrMemberUserIDRequired.Write(c)
		return
	}

	var req updateAdminSpaceMemberRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AdminSpaceErrRequestBody.Write(c)
		return
	}

	member, err := h.adminSpaceService.UpdateMemberRole(c.Request.Context(), service.UpdateAdminSpaceMemberRoleInput{
		ActorUserID: actorUserID,
		RequestID:   response.RequestIDFromContext(c),
		SpaceID:     spaceID,
		UserID:      memberUserID,
		Role:        models.Role(strings.ToLower(strings.TrimSpace(req.Role))),
	})
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, mapAdminSpaceMemberResponse(member))
}

// DeleteMember 删除空间成员。
func (h *adminSpaceHandler) DeleteMember(c *gin.Context) {
	if h == nil || h.adminSpaceService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.AdminSpaceErrAdminActorMissing.Write(c)
		return
	}

	spaceID := strings.TrimSpace(c.Param("spaceId"))
	if spaceID == "" {
		response.AdminSpaceErrSpaceIDRequired.Write(c)
		return
	}
	memberUserID := strings.TrimSpace(c.Param("userId"))
	if memberUserID == "" {
		response.AdminSpaceErrMemberUserIDRequired.Write(c)
		return
	}

	if err := h.adminSpaceService.DeleteMember(c.Request.Context(), service.DeleteAdminSpaceMemberInput{
		ActorUserID: actorUserID,
		RequestID:   response.RequestIDFromContext(c),
		SpaceID:     spaceID,
		UserID:      memberUserID,
	}); err != nil {
		response.FromError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, struct{}{})
}

// UpdateMetadata 更新空间名称、可见性等元数据。
func (h *adminSpaceHandler) UpdateMetadata(c *gin.Context) {
	if h == nil || h.adminSpaceService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.AdminSpaceErrAdminActorMissing.Write(c)
		return
	}

	spaceID := strings.TrimSpace(c.Param("spaceId"))
	if spaceID == "" {
		response.AdminSpaceErrSpaceIDRequired.Write(c)
		return
	}

	var req updateAdminSpaceMetadataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AdminSpaceErrRequestBody.Write(c)
		return
	}

	var visibility *models.Visibility
	if req.Visibility != nil {
		normalizedVisibility := models.Visibility(strings.ToLower(strings.TrimSpace(*req.Visibility)))
		visibility = &normalizedVisibility
	}

	payload, err := h.adminSpaceService.UpdateMetadata(c.Request.Context(), service.UpdateAdminSpaceMetadataInput{
		ActorUserID:  actorUserID,
		RequestID:    response.RequestIDFromContext(c),
		SpaceID:      spaceID,
		Name:         req.Name,
		Description:  req.Description,
		CategoryID:   req.CategoryID,
		Visibility:   visibility,
		CoverAssetID: req.CoverAssetID,
	})
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, mapAdminSpaceResponse(payload))
}

// TransferOwnership 转让空间归属。
func (h *adminSpaceHandler) TransferOwnership(c *gin.Context) {
	if h == nil || h.adminSpaceService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.AdminSpaceErrAdminActorMissing.Write(c)
		return
	}

	spaceID := strings.TrimSpace(c.Param("spaceId"))
	if spaceID == "" {
		response.AdminSpaceErrSpaceIDRequired.Write(c)
		return
	}

	var req transferAdminSpaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AdminSpaceErrRequestBody.Write(c)
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
		response.FromError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, mapAdminSpaceResponse(payload))
}

// UpdateStatus 更新空间状态（active/banned）。
func (h *adminSpaceHandler) UpdateStatus(c *gin.Context) {
	if h == nil || h.adminSpaceService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.AdminSpaceErrAdminActorMissing.Write(c)
		return
	}

	spaceID := strings.TrimSpace(c.Param("spaceId"))
	if spaceID == "" {
		response.AdminSpaceErrSpaceIDRequired.Write(c)
		return
	}

	var req updateAdminSpaceStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AdminSpaceErrRequestBody.Write(c)
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
		response.FromError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, mapAdminSpaceResponse(payload))
}

// DeleteSpace 删除空间及关联资源。
func (h *adminSpaceHandler) DeleteSpace(c *gin.Context) {
	if h == nil || h.adminSpaceService == nil {
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		response.AdminSpaceErrAdminActorMissing.Write(c)
		return
	}

	spaceID := strings.TrimSpace(c.Param("spaceId"))
	if spaceID == "" {
		response.AdminSpaceErrSpaceIDRequired.Write(c)
		return
	}

	if err := h.adminSpaceService.DeleteSpace(
		c.Request.Context(),
		actorUserID,
		spaceID,
		response.RequestIDFromContext(c),
	); err != nil {
		response.FromError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, struct{}{})
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
		SpaceID:           value.SpaceID,
		Name:              value.Name,
		Description:       value.Description,
		CategoryID:        value.CategoryID,
		Category:          value.Category,
		CategoryIsDefault: value.CategoryIsDefault,
		OwnerUserID:       value.OwnerUserID,
		OwnerName:         value.OwnerName,
		OwnerEmail:        value.OwnerEmail,
		Visibility:        value.Visibility,
		Cover:             mapAdminSpaceCoverDTOFromPointer(value.Cover),
		Status:            value.Status,
		BannedReason:      value.BannedReason,
		BannedAt:          value.BannedAt,
		DeletedAt:         value.DeletedAt,
		CreatedAt:         value.CreatedAt,
		UpdatedAt:         value.UpdatedAt,
	}
}

func mapAdminSpaceCategoryResponse(value service.AdminSpaceCategoryRecord) adminSpaceCategoryResponse {
	return adminSpaceCategoryResponse{
		CategoryID: value.CategoryID,
		Name:       value.Name,
		IsDefault:  value.IsDefault,
		CreatedAt:  value.CreatedAt,
		UpdatedAt:  value.UpdatedAt,
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

func mapAdminSpaceMemberResponse(value service.AdminSpaceMemberRecord) adminSpaceMemberResponse {
	return adminSpaceMemberResponse{
		UserID:    value.UserID,
		Email:     value.Email,
		Name:      value.Name,
		Role:      value.Role,
		IsOwner:   value.IsOwner,
		CreatedAt: value.CreatedAt,
		UpdatedAt: value.UpdatedAt,
	}
}

func readAdminUploadedFileBytes(fileHeader *multipart.FileHeader) ([]byte, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}
