package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/logit"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/middleware"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
)

type adminDocumentImageAssetHandler struct {
	adminDocumentImageAssetService *service.AdminDocumentImageAssetService
}

type adminDocumentImageAssetResponse struct {
	ImageAssetID     string              `json:"imageAssetId"`
	DocumentID       string              `json:"documentId"`
	DocumentRouteKey string              `json:"documentRouteKey"`
	DocumentTitle    string              `json:"documentTitle"`
	DocumentStatus   models.EntityStatus `json:"documentStatus"`
	SpaceID          string              `json:"spaceId"`
	SpaceName        string              `json:"spaceName"`
	SpaceOwnerUserID string              `json:"spaceOwnerUserId"`
	SpaceOwnerName   string              `json:"spaceOwnerName"`
	SpaceOwnerEmail  string              `json:"spaceOwnerEmail"`
	StorageProvider  string              `json:"storageProvider"`
	ObjectKey        string              `json:"objectKey"`
	ObjectURL        string              `json:"objectUrl"`
	Status           string              `json:"status"`
	LastReferencedAt time.Time           `json:"lastReferencedAt"`
	PendingCleanupAt *time.Time          `json:"pendingCleanupAt"`
	DeletedAt        *time.Time          `json:"deletedAt"`
	CreatedAt        time.Time           `json:"createdAt"`
	UpdatedAt        time.Time           `json:"updatedAt"`
}

type adminDocumentImageAssetPageResponse struct {
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
	Total    int64 `json:"total"`
}

type adminDocumentImageAssetListResponse struct {
	Items      []adminDocumentImageAssetResponse   `json:"items"`
	Pagination adminDocumentImageAssetPageResponse `json:"pagination"`
}

type adminDocumentImageAssetDeleteReferenceResponse struct {
	ImageAssetID  string `json:"imageAssetId"`
	DocumentID    string `json:"documentId"`
	DocumentTitle string `json:"documentTitle"`
	SpaceID       string `json:"spaceId"`
	SpaceName     string `json:"spaceName"`
}

type adminDocumentImageAssetDeleteResponse struct {
	ImageAssetID            string                                           `json:"imageAssetId"`
	DocumentID              string                                           `json:"documentId"`
	SpaceID                 string                                           `json:"spaceId"`
	PhysicalDeleteRequested bool                                             `json:"physicalDeleteRequested"`
	PhysicalDeleteExecuted  bool                                             `json:"physicalDeleteExecuted"`
	SoftDeleted             bool                                             `json:"softDeleted"`
	HardDeleted             bool                                             `json:"hardDeleted"`
	SharedReferenceCount    int64                                            `json:"sharedReferenceCount"`
	SharedReferences        []adminDocumentImageAssetDeleteReferenceResponse `json:"sharedReferences"`
	ConfirmationRequired    bool                                             `json:"confirmationRequired"`
	ConfirmationReason      string                                           `json:"confirmationReason"`
	PhysicalDeleteError     string                                           `json:"physicalDeleteError"`
}

// NewAdminDocumentImageAssetHandler 创建后台文档图片资源管理处理器。
func NewAdminDocumentImageAssetHandler(
	adminDocumentImageAssetService *service.AdminDocumentImageAssetService,
) *adminDocumentImageAssetHandler {
	return &adminDocumentImageAssetHandler{
		adminDocumentImageAssetService: adminDocumentImageAssetService,
	}
}

// ListImageAssets 返回后台文档图片资源列表。
func (h *adminDocumentImageAssetHandler) ListImageAssets(c *gin.Context) {
	if h == nil || h.adminDocumentImageAssetService == nil {
		setRequestErrmsgText(c, "初始化失败: admin document image asset service is nil")
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		setRequestErrmsg(c, err, "解析管理员身份失败")
		response.AdminDocumentImageAssetErrAdminActorMissing.Write(c)
		return
	}

	page, err := parseAdminDocumentQueryInt(c.Query("page"))
	if err != nil {
		setRequestErrmsg(c, err, "解析页码参数失败")
		response.AdminDocumentImageAssetErrPagePositiveInteger.Write(c)
		return
	}
	pageSize, err := parseAdminDocumentQueryInt(c.Query("pageSize"))
	if err != nil {
		setRequestErrmsg(c, err, "解析每页数量参数失败")
		response.AdminDocumentImageAssetErrPageSizePositiveInteger.Write(c)
		return
	}
	logit.SetRequestAttrs(c.Request.Context(),
		logit.Any("actorUserID", actorUserID),
		logit.Any("keyword", c.Query("keyword")),
		logit.Any("spaceId", c.Query("spaceId")),
		logit.Any("documentId", c.Query("documentId")),
		logit.Any("status", c.Query("status")),
		logit.Any("storageProvider", c.Query("storageProvider")),
		logit.Any("page", page),
		logit.Any("pageSize", pageSize),
	)

	payload, err := h.adminDocumentImageAssetService.ListImageAssets(
		c.Request.Context(),
		service.ListAdminDocumentImageAssetsInput{
			ActorUserID:           actorUserID,
			Keyword:               strings.TrimSpace(c.Query("keyword")),
			SpaceID:               strings.TrimSpace(c.Query("spaceId")),
			DocumentID:            strings.TrimSpace(c.Query("documentId")),
			StatusFilter:          service.AdminDocumentImageAssetStatusFilter(c.Query("status")),
			StorageProviderFilter: service.AdminDocumentImageAssetStorageProviderFilter(c.Query("storageProvider")),
			Page:                  page,
			PageSize:              pageSize,
		},
	)
	if err != nil {
		setRequestErrmsg(c, err, "查询后台文档图片资源列表失败")
		response.FromError(c, err)
		return
	}

	items := make([]adminDocumentImageAssetResponse, 0, len(payload.Items))
	for _, item := range payload.Items {
		items = append(items, mapAdminDocumentImageAssetResponse(item))
	}

	response.JSON(c, http.StatusOK, adminDocumentImageAssetListResponse{
		Items: items,
		Pagination: adminDocumentImageAssetPageResponse{
			Page:     payload.Page,
			PageSize: payload.PageSize,
			Total:    payload.Total,
		},
	})
}

// DeleteImageAsset 删除文档图片资源：
// - physicalDelete=false: 逻辑删除（仅标记为 deleted）
// - physicalDelete=true: 物理删除（删除记录；无活跃引用时删除物理文件）
func (h *adminDocumentImageAssetHandler) DeleteImageAsset(c *gin.Context) {
	if h == nil || h.adminDocumentImageAssetService == nil {
		setRequestErrmsgText(c, "初始化失败: admin document image asset service is nil")
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		setRequestErrmsg(c, err, "解析管理员身份失败")
		response.AdminDocumentImageAssetErrAdminActorMissing.Write(c)
		return
	}

	imageAssetID := strings.TrimSpace(c.Param("imageAssetId"))
	if imageAssetID == "" {
		setRequestErrmsgText(c, "图片资源 ID 不能为空")
		response.AdminDocumentImageAssetErrImageAssetIDRequired.Write(c)
		return
	}

	result, err := h.adminDocumentImageAssetService.DeleteImageAsset(c.Request.Context(), service.DeleteAdminDocumentImageAssetInput{
		ActorUserID:                actorUserID,
		ImageAssetID:               imageAssetID,
		PhysicalDelete:             parseBoolLikeValue(c.Query("physicalDelete")),
		ForcePhysicalDeleteOnShare: parseBoolLikeValue(c.Query("forcePhysicalDeleteOnShare")),
		RequestID:                  response.RequestIDFromContext(c),
	})
	if err != nil {
		setRequestErrmsg(c, err, "删除后台文档图片资源失败")
		response.FromError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, mapAdminDocumentImageAssetDeleteResponse(result))
}

func mapAdminDocumentImageAssetResponse(value service.AdminDocumentImageAssetRecord) adminDocumentImageAssetResponse {
	return adminDocumentImageAssetResponse{
		ImageAssetID:     value.ImageAssetID,
		DocumentID:       value.DocumentID,
		DocumentRouteKey: value.DocumentRouteKey,
		DocumentTitle:    value.DocumentTitle,
		DocumentStatus:   value.DocumentStatus,
		SpaceID:          value.SpaceID,
		SpaceName:        value.SpaceName,
		SpaceOwnerUserID: value.SpaceOwnerUserID,
		SpaceOwnerName:   value.SpaceOwnerName,
		SpaceOwnerEmail:  value.SpaceOwnerEmail,
		StorageProvider:  value.StorageProvider,
		ObjectKey:        value.ObjectKey,
		ObjectURL:        value.ObjectURL,
		Status:           value.Status,
		LastReferencedAt: value.LastReferencedAt,
		PendingCleanupAt: value.PendingCleanupAt,
		DeletedAt:        value.DeletedAt,
		CreatedAt:        value.CreatedAt,
		UpdatedAt:        value.UpdatedAt,
	}
}

func mapAdminDocumentImageAssetDeleteResponse(
	value service.DeleteAdminDocumentImageAssetResult,
) adminDocumentImageAssetDeleteResponse {
	items := make([]adminDocumentImageAssetDeleteReferenceResponse, 0, len(value.SharedReferences))
	for _, reference := range value.SharedReferences {
		items = append(items, adminDocumentImageAssetDeleteReferenceResponse{
			ImageAssetID:  strings.TrimSpace(reference.ImageAssetID),
			DocumentID:    strings.TrimSpace(reference.DocumentID),
			DocumentTitle: strings.TrimSpace(reference.DocumentTitle),
			SpaceID:       strings.TrimSpace(reference.SpaceID),
			SpaceName:     strings.TrimSpace(reference.SpaceName),
		})
	}

	return adminDocumentImageAssetDeleteResponse{
		ImageAssetID:            strings.TrimSpace(value.ImageAssetID),
		DocumentID:              strings.TrimSpace(value.DocumentID),
		SpaceID:                 strings.TrimSpace(value.SpaceID),
		PhysicalDeleteRequested: value.PhysicalDeleteRequested,
		PhysicalDeleteExecuted:  value.PhysicalDeleteExecuted,
		SoftDeleted:             value.SoftDeleted,
		HardDeleted:             value.HardDeleted,
		SharedReferenceCount:    value.SharedReferenceCount,
		SharedReferences:        items,
		ConfirmationRequired:    value.ConfirmationRequired,
		ConfirmationReason:      strings.TrimSpace(value.ConfirmationReason),
		PhysicalDeleteError:     strings.TrimSpace(value.PhysicalDeleteError),
	}
}
