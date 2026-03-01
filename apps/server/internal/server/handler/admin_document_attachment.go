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

type adminDocumentAttachmentHandler struct {
	adminDocumentAttachmentService *service.AdminDocumentAttachmentService
}

type adminDocumentAttachmentResponse struct {
	AttachmentID     string              `json:"attachmentId"`
	DocumentID       string              `json:"documentId"`
	DocumentTitle    string              `json:"documentTitle"`
	DocumentStatus   models.EntityStatus `json:"documentStatus"`
	SpaceID          string              `json:"spaceId"`
	SpaceName        string              `json:"spaceName"`
	SpaceOwnerUserID string              `json:"spaceOwnerUserId"`
	SpaceOwnerName   string              `json:"spaceOwnerName"`
	SpaceOwnerEmail  string              `json:"spaceOwnerEmail"`
	FileName         string              `json:"fileName"`
	MimeType         string              `json:"mimeType"`
	SizeBytes        int64               `json:"sizeBytes"`
	StorageProvider  string              `json:"storageProvider"`
	PreviewKind      string              `json:"previewKind"`
	Status           models.EntityStatus `json:"status"`
	CreatedByUserID  *string             `json:"createdByUserId"`
	CreatedByName    string              `json:"createdByName"`
	CreatedByEmail   string              `json:"createdByEmail"`
	DeletedAt        *time.Time          `json:"deletedAt"`
	CreatedAt        time.Time           `json:"createdAt"`
	UpdatedAt        time.Time           `json:"updatedAt"`
}

type adminDocumentAttachmentPageResponse struct {
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
	Total    int64 `json:"total"`
}

type adminDocumentAttachmentListResponse struct {
	Items      []adminDocumentAttachmentResponse   `json:"items"`
	Pagination adminDocumentAttachmentPageResponse `json:"pagination"`
}

type adminDocumentAttachmentDeleteReferenceResponse struct {
	AttachmentID  string `json:"attachmentId"`
	DocumentID    string `json:"documentId"`
	DocumentTitle string `json:"documentTitle"`
	SpaceID       string `json:"spaceId"`
	SpaceName     string `json:"spaceName"`
	FileName      string `json:"fileName"`
}

type adminDocumentAttachmentDeleteResponse struct {
	AttachmentID            string                                           `json:"attachmentId"`
	DocumentID              string                                           `json:"documentId"`
	SpaceID                 string                                           `json:"spaceId"`
	PhysicalDeleteRequested bool                                             `json:"physicalDeleteRequested"`
	PhysicalDeleteExecuted  bool                                             `json:"physicalDeleteExecuted"`
	SoftDeleted             bool                                             `json:"softDeleted"`
	HardDeleted             bool                                             `json:"hardDeleted"`
	SharedReferenceCount    int64                                            `json:"sharedReferenceCount"`
	SharedReferences        []adminDocumentAttachmentDeleteReferenceResponse `json:"sharedReferences"`
	ConfirmationRequired    bool                                             `json:"confirmationRequired"`
	ConfirmationReason      string                                           `json:"confirmationReason"`
	PhysicalDeleteError     string                                           `json:"physicalDeleteError"`
}

// NewAdminDocumentAttachmentHandler 创建后台文档附件管理处理器。
func NewAdminDocumentAttachmentHandler(
	adminDocumentAttachmentService *service.AdminDocumentAttachmentService,
) *adminDocumentAttachmentHandler {
	return &adminDocumentAttachmentHandler{
		adminDocumentAttachmentService: adminDocumentAttachmentService,
	}
}

// ListAttachments 返回后台文档附件列表，支持关键词、空间、文档、状态、存储提供商与分页查询。
func (h *adminDocumentAttachmentHandler) ListAttachments(c *gin.Context) {
	if h == nil || h.adminDocumentAttachmentService == nil {
		setRequestErrmsgText(c, "初始化失败: admin document attachment service is nil")
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		setRequestErrmsg(c, err, "解析管理员身份失败")
		response.AdminDocumentAttachmentErrAdminActorMissing.Write(c)
		return
	}

	page, err := parseAdminDocumentQueryInt(c.Query("page"))
	if err != nil {
		setRequestErrmsg(c, err, "解析页码参数失败")
		response.AdminDocumentAttachmentErrPagePositiveInteger.Write(c)
		return
	}
	pageSize, err := parseAdminDocumentQueryInt(c.Query("pageSize"))
	if err != nil {
		setRequestErrmsg(c, err, "解析每页数量参数失败")
		response.AdminDocumentAttachmentErrPageSizePositiveInteger.Write(c)
		return
	}

	payload, err := h.adminDocumentAttachmentService.ListAttachments(
		c.Request.Context(),
		service.ListAdminDocumentAttachmentsInput{
			ActorUserID:           actorUserID,
			Keyword:               strings.TrimSpace(c.Query("keyword")),
			SpaceID:               strings.TrimSpace(c.Query("spaceId")),
			DocumentID:            strings.TrimSpace(c.Query("documentId")),
			StatusFilter:          service.AdminDocumentAttachmentStatusFilter(c.Query("status")),
			StorageProviderFilter: service.AdminDocumentAttachmentStorageProviderFilter(c.Query("storageProvider")),
			Page:                  page,
			PageSize:              pageSize,
		},
	)
	if err != nil {
		setRequestErrmsg(c, err, "查询后台文档附件列表失败")
		response.FromError(c, err)
		return
	}

	items := make([]adminDocumentAttachmentResponse, 0, len(payload.Items))
	for _, item := range payload.Items {
		items = append(items, mapAdminDocumentAttachmentResponse(item))
	}

	response.JSON(c, http.StatusOK, adminDocumentAttachmentListResponse{
		Items: items,
		Pagination: adminDocumentAttachmentPageResponse{
			Page:     payload.Page,
			PageSize: payload.PageSize,
			Total:    payload.Total,
		},
	})
}

// DeleteAttachment 删除文档附件：
// - physicalDelete=false: 逻辑删除（保留记录与文件）
// - physicalDelete=true: 物理删除（删除文件并硬删除记录）
func (h *adminDocumentAttachmentHandler) DeleteAttachment(c *gin.Context) {
	if h == nil || h.adminDocumentAttachmentService == nil {
		setRequestErrmsgText(c, "初始化失败: admin document attachment service is nil")
		response.InternalError(c)
		return
	}

	actorUserID, err := middleware.AdminActorUserID(c)
	if err != nil {
		setRequestErrmsg(c, err, "解析管理员身份失败")
		response.AdminDocumentAttachmentErrAdminActorMissing.Write(c)
		return
	}

	attachmentID := strings.TrimSpace(c.Param("attachmentId"))
	if attachmentID == "" {
		setRequestErrmsgText(c, "附件 ID 不能为空")
		response.AdminDocumentAttachmentErrAttachmentIDRequired.Write(c)
		return
	}

	logit.SetRequestAttrs(c.Request.Context(),
		logit.Any("attachmentId", attachmentID),
		logit.Any("actorUserID", actorUserID),
	)

	result, err := h.adminDocumentAttachmentService.DeleteAttachment(c.Request.Context(), service.DeleteAdminDocumentAttachmentInput{
		ActorUserID:                actorUserID,
		AttachmentID:               attachmentID,
		PhysicalDelete:             parseBoolLikeValue(c.Query("physicalDelete")),
		ForcePhysicalDeleteOnShare: parseBoolLikeValue(c.Query("forcePhysicalDeleteOnShare")),
		RequestID:                  response.RequestIDFromContext(c),
	})
	if err != nil {
		setRequestErrmsg(c, err, "删除后台文档附件失败")
		response.FromError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, mapAdminDocumentAttachmentDeleteResponse(result))
}

func mapAdminDocumentAttachmentResponse(value service.AdminDocumentAttachmentRecord) adminDocumentAttachmentResponse {
	return adminDocumentAttachmentResponse{
		AttachmentID:     value.AttachmentID,
		DocumentID:       value.DocumentID,
		DocumentTitle:    value.DocumentTitle,
		DocumentStatus:   value.DocumentStatus,
		SpaceID:          value.SpaceID,
		SpaceName:        value.SpaceName,
		SpaceOwnerUserID: value.SpaceOwnerUserID,
		SpaceOwnerName:   value.SpaceOwnerName,
		SpaceOwnerEmail:  value.SpaceOwnerEmail,
		FileName:         value.FileName,
		MimeType:         value.MimeType,
		SizeBytes:        value.SizeBytes,
		StorageProvider:  value.StorageProvider,
		PreviewKind:      value.PreviewKind,
		Status:           value.Status,
		CreatedByUserID:  value.CreatedByUserID,
		CreatedByName:    value.CreatedByName,
		CreatedByEmail:   value.CreatedByEmail,
		DeletedAt:        value.DeletedAt,
		CreatedAt:        value.CreatedAt,
		UpdatedAt:        value.UpdatedAt,
	}
}

func mapAdminDocumentAttachmentDeleteResponse(
	value service.DeleteAdminDocumentAttachmentResult,
) adminDocumentAttachmentDeleteResponse {
	items := make([]adminDocumentAttachmentDeleteReferenceResponse, 0, len(value.SharedReferences))
	for _, reference := range value.SharedReferences {
		items = append(items, adminDocumentAttachmentDeleteReferenceResponse{
			AttachmentID:  strings.TrimSpace(reference.AttachmentID),
			DocumentID:    strings.TrimSpace(reference.DocumentID),
			DocumentTitle: strings.TrimSpace(reference.DocumentTitle),
			SpaceID:       strings.TrimSpace(reference.SpaceID),
			SpaceName:     strings.TrimSpace(reference.SpaceName),
			FileName:      strings.TrimSpace(reference.FileName),
		})
	}

	return adminDocumentAttachmentDeleteResponse{
		AttachmentID:            strings.TrimSpace(value.AttachmentID),
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
