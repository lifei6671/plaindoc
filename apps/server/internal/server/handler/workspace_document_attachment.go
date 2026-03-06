package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/logit"
	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"
)

const (
	maxWorkspaceAttachmentSizeBytes = 100 << 20 // 100MB
)

const (
	documentAttachmentPreviewKindNone   = "none"
	documentAttachmentPreviewKindImage  = "image"
	documentAttachmentPreviewKindPDF    = "pdf"
	documentAttachmentPreviewKindOffice = "office"
	documentAttachmentPreviewKindText   = "text"
)

type workspaceDocumentAttachmentResponse struct {
	AttachmentID         string     `json:"attachmentId"`
	DocumentID           string     `json:"documentId"`
	SpaceID              string     `json:"spaceId"`
	FileName             string     `json:"fileName"`
	MimeType             string     `json:"mimeType"`
	SizeBytes            int64      `json:"sizeBytes"`
	StorageProvider      string     `json:"storageProvider"`
	PreviewKind          string     `json:"previewKind"`
	PreviewSupported     bool       `json:"previewSupported"`
	RequiresAuthDownload bool       `json:"requiresAuthDownload"`
	PublicDownloadURL    string     `json:"publicDownloadUrl,omitempty"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
	DeletedAt            *time.Time `json:"deletedAt,omitempty"`
}

type workspaceDocumentAttachmentListResponse struct {
	Items []workspaceDocumentAttachmentResponse `json:"items"`
}

type workspaceDocumentAttachmentAccessLinkResponse struct {
	URL          string `json:"url"`
	Purpose      string `json:"purpose"`
	PreviewKind  string `json:"previewKind"`
	RequiresAuth bool   `json:"requiresAuth"`
	ExpiresAt    string `json:"expiresAt,omitempty"`
}

type workspaceDocumentAttachmentPurpose string

const (
	workspaceDocumentAttachmentPurposeDownload workspaceDocumentAttachmentPurpose = "download"
	workspaceDocumentAttachmentPurposePreview  workspaceDocumentAttachmentPurpose = "preview"
)

// ListDocumentAttachments 返回文档附件列表。
func (h *workspaceHandler) ListDocumentAttachments(c *gin.Context) {
	if h == nil || h.documentAttachmentRepo == nil || h.visibilityService == nil {
		setRequestErrmsgText(c, "初始化失败: handler or dependencies is nil")
		response.InternalError(c)
		return
	}

	documentID := strings.TrimSpace(c.Param("docId"))
	if documentID == "" {
		response.WorkspaceErrDocumentIDRequired.Write(c)
		return
	}

	viewerUserID, err := h.resolveOptionalViewerUserID(c)
	if err != nil {
		setRequestErrmsg(c, err, "解析访问令牌失败")
		response.WorkspaceErrAccessToken.Write(c)
		return
	}
	if _, err := h.visibilityService.GetDocument(c.Request.Context(), documentID, viewerUserID); err != nil {
		setRequestErrmsg(c, err, "验证文档访问权限失败")
		writeWorkspaceDocumentAccessError(c, err)
		return
	}

	attachments, err := h.documentAttachmentRepo.ListByDocumentID(c.Request.Context(), documentID, false)
	if err != nil {
		setRequestErrmsg(c, err, "查询文档附件失败")
		response.InternalError(c)
		return
	}

	publicReadable := h.isDocumentPubliclyReadable(c.Request.Context(), documentID)
	config := service.DefaultImageHostingConfig()
	if h.imageHostingService != nil {
		if loadedConfig, configErr := h.imageHostingService.GetConfig(c.Request.Context()); configErr == nil {
			config = loadedConfig
		} else {
			setRequestErrmsg(c, configErr, "读取图床配置失败")
		}
	}

	items := make([]workspaceDocumentAttachmentResponse, 0, len(attachments))
	for _, attachment := range attachments {
		publicDownloadURL := ""
		if publicReadable {
			resolvedURL, resolvedURLErr := h.resolveAttachmentPublicDownloadURL(
				c.Request.Context(),
				attachment,
				config,
				service.DocumentAttachmentLinkPurposeDownload,
			)
			if resolvedURLErr != nil {
				setRequestErrmsg(c, resolvedURLErr, "生成附件公开链接失败")
			} else {
				publicDownloadURL = resolvedURL
			}
		}
		items = append(items, mapWorkspaceDocumentAttachmentResponse(
			attachment,
			publicDownloadURL,
			!publicReadable,
		))
	}

	response.JSON(c, http.StatusOK, workspaceDocumentAttachmentListResponse{
		Items: items,
	})
}

// UploadDocumentAttachment 上传文档附件。
func (h *workspaceHandler) UploadDocumentAttachment(c *gin.Context) {
	actorUserID, ok := h.requireActorUserID(c)
	if !ok {
		return
	}
	if h == nil || h.workspaceRepo == nil || h.documentAttachmentRepo == nil || h.imageHostingService == nil {
		setRequestErrmsgText(c, "初始化失败: handler or dependencies is nil")
		response.InternalError(c)
		return
	}

	documentID := strings.TrimSpace(c.Param("docId"))
	if documentID == "" {
		response.WorkspaceErrDocumentIDRequired.Write(c)
		return
	}

	currentRecord, err := h.workspaceRepo.GetDocumentByDocumentID(c.Request.Context(), documentID)
	if err != nil {
		setRequestErrmsg(c, err, "查询文档失败")
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.WorkspaceErrDocumentNotFound.Write(c)
			return
		}
		response.InternalError(c)
		return
	}

	spaceID := strings.TrimSpace(currentRecord.SpaceID)
	if _, err := h.ensureSpaceWritable(c.Request.Context(), spaceID, actorUserID); err != nil {
		setRequestErrmsg(c, err, "验证空间权限失败")
		switch {
		case errors.Is(err, service.ErrSpaceNotFound):
			response.WorkspaceErrSpaceNotFound.Write(c)
		case errors.Is(err, service.ErrSpaceAccessDenied):
			response.WorkspaceErrInsufficientSpacePermission.Write(c)
		default:
			setRequestErrmsg(c, err, "验证空间权限失败")
			response.InternalError(c)
		}
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil || fileHeader == nil {
		if err != nil {
			setRequestErrmsg(c, err, "读取上传文件失败")
		} else {
			setRequestErrmsgText(c, "上传文件为空")
		}
		response.DocumentAttachmentErrUploadFileRequired.Write(c)
		return
	}
	if fileHeader.Size <= 0 {
		response.DocumentAttachmentErrUploadFileEmpty.Write(c)
		return
	}
	if fileHeader.Size > maxWorkspaceAttachmentSizeBytes {
		response.DocumentAttachmentErrUploadFileTooLarge.Write(c)
		return
	}

	config, err := h.imageHostingService.GetConfig(c.Request.Context())
	if err != nil {
		setRequestErrmsg(c, err, "获取图片托管服务配置失败")
		response.InternalError(c)
		return
	}
	targetProvider := config.DefaultProvider
	if targetProvider == "" {
		targetProvider = service.ImageHostingProviderLocal
	}

	contentType, err := detectUploadedFileContentType(fileHeader)
	if err != nil {
		setRequestErrmsg(c, err, "检测上传文件内容类型失败")
		response.ImageHostingUploadFileUnreadable(c)
		return
	}

	const contentHashAlgo = "sha256"
	contentHash, err := computeUploadedFileSHA256(fileHeader)
	if err != nil {
		setRequestErrmsg(c, err, "计算上传文件哈希失败")
		response.ImageHostingUploadFileUnreadable(c)
		return
	}

	blob, err := h.documentAttachmentRepo.FindBlobByHash(
		c.Request.Context(),
		string(targetProvider),
		contentHashAlgo,
		contentHash,
		fileHeader.Size,
	)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		setRequestErrmsg(c, err, "按哈希查询文件实体失败")
		response.InternalError(c)
		return
	}

	savedTargetPath := ""
	createdBlobID := ""
	if errors.Is(err, gorm.ErrRecordNotFound) {
		objectKey, objectKeyErr := buildDocumentAttachmentObjectKey(
			fileHeader.Filename,
			contentType,
			spaceID,
			documentID,
			actorUserID,
			time.Now().UTC(),
			config.UploadPathTemplate(targetProvider),
		)
		if objectKeyErr != nil {
			setRequestErrmsg(c, objectKeyErr, "生成对象存储键失败")
			response.InternalError(c)
			return
		}

		uploadedObjectURL, uploadedLocalTargetPath, uploadErr := h.uploadDocumentAttachmentToProvider(
			c,
			fileHeader,
			contentType,
			objectKey,
			targetProvider,
			config,
		)
		if uploadErr != nil {
			setRequestErrmsg(c, uploadErr, "上传附件到对象存储失败")
			response.InternalError(c)
			return
		}
		savedTargetPath = uploadedLocalTargetPath

		now := time.Now().UTC()
		blobCandidate := &models.DocumentAttachmentBlob{
			BlobID:          strings.ToLower(ulid.Make().String()),
			StorageProvider: string(targetProvider),
			ObjectKey:       objectKey,
			ObjectURL:       strings.TrimSpace(uploadedObjectURL),
			MimeType:        strings.TrimSpace(contentType),
			SizeBytes:       fileHeader.Size,
			ContentHashAlgo: contentHashAlgo,
			ContentHash:     contentHash,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if strings.TrimSpace(blobCandidate.MimeType) == "" {
			blobCandidate.MimeType = "application/octet-stream"
		}

		createBlobErr := h.documentAttachmentRepo.CreateBlob(c.Request.Context(), blobCandidate)
		if createBlobErr != nil {
			if savedTargetPath != "" {
				if cleanupErr := os.Remove(savedTargetPath); cleanupErr != nil && !errors.Is(cleanupErr, os.ErrNotExist) {
					setRequestErrmsg(c, cleanupErr, "清理重复上传临时文件失败")
				}
			}
			savedTargetPath = ""

			if isLikelyUniqueConstraintError(createBlobErr) {
				existingBlob, lookupErr := h.documentAttachmentRepo.FindBlobByHash(
					c.Request.Context(),
					string(targetProvider),
					contentHashAlgo,
					contentHash,
					fileHeader.Size,
				)
				if lookupErr != nil {
					setRequestErrmsg(c, lookupErr, "回查去重文件实体失败")
					response.InternalError(c)
					return
				}
				blob = existingBlob
			} else {
				setRequestErrmsg(c, createBlobErr, "创建文件实体失败")
				response.InternalError(c)
				return
			}
		} else {
			blob = blobCandidate
			createdBlobID = blobCandidate.BlobID
		}
	}

	if blob == nil {
		setRequestErrmsgText(c, "文件实体创建失败")
		response.InternalError(c)
		return
	}

	now := time.Now().UTC()
	createdBy := actorUserID
	attachment := &models.DocumentAttachment{
		AttachmentID:    strings.ToLower(ulid.Make().String()),
		BlobID:          strings.TrimSpace(blob.BlobID),
		DocumentID:      documentID,
		SpaceID:         spaceID,
		StorageProvider: strings.TrimSpace(blob.StorageProvider),
		FileName:        strings.TrimSpace(fileHeader.Filename),
		ObjectKey:       strings.TrimSpace(blob.ObjectKey),
		ObjectURL:       strings.TrimSpace(blob.ObjectURL),
		MimeType:        strings.TrimSpace(blob.MimeType),
		SizeBytes:       blob.SizeBytes,
		ContentHashAlgo: strings.TrimSpace(blob.ContentHashAlgo),
		ContentHash:     strings.TrimSpace(blob.ContentHash),
		PreviewKind:     resolveDocumentAttachmentPreviewKind(contentType, fileHeader.Filename),
		Status:          models.EntityStatusActive,
		CreatedByUserID: &createdBy,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if strings.TrimSpace(attachment.FileName) == "" {
		attachment.FileName = attachment.AttachmentID
	}
	if strings.TrimSpace(attachment.MimeType) == "" {
		attachment.MimeType = "application/octet-stream"
	}
	if strings.TrimSpace(attachment.StorageProvider) == "" {
		attachment.StorageProvider = string(targetProvider)
	}
	if strings.TrimSpace(attachment.ObjectURL) == "" {
		switch targetProvider {
		case service.ImageHostingProviderLocal:
			attachment.ObjectURL = resolvePublicURL(config.Local.PublicBaseURL, attachment.ObjectKey, "/uploads")
		case service.ImageHostingProviderCloudflareR2:
			attachment.ObjectURL = resolveObjectStoragePublicURL(config.CloudflareR2.PublicBaseURL, attachment.ObjectKey)
		case service.ImageHostingProviderAliyunOSS:
			attachment.ObjectURL = resolveObjectStoragePublicURL(config.AliyunOSS.PublicBaseURL, attachment.ObjectKey)
		default:
			attachment.ObjectURL = resolvePublicURL(config.Local.PublicBaseURL, attachment.ObjectKey, "/uploads")
		}
	}
	if strings.TrimSpace(attachment.ContentHashAlgo) == "" {
		attachment.ContentHashAlgo = contentHashAlgo
	}
	if strings.TrimSpace(attachment.ContentHash) == "" {
		attachment.ContentHash = contentHash
	}
	if strings.TrimSpace(attachment.PreviewKind) == "" {
		attachment.PreviewKind = documentAttachmentPreviewKindNone
	}

	if err := h.documentAttachmentRepo.Create(c.Request.Context(), attachment); err != nil {
		if createdBlobID != "" {
			hardDeletedBlob, cleanupErr := h.documentAttachmentRepo.HardDeleteBlobIfUnreferenced(c.Request.Context(), createdBlobID)
			if cleanupErr != nil {
				setRequestErrmsg(c, cleanupErr, "回滚文件实体失败")
			} else if hardDeletedBlob && savedTargetPath != "" {
				if removeErr := os.Remove(savedTargetPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
					setRequestErrmsg(c, removeErr, "回滚上传文件失败")
				}
			}
		}
		setRequestErrmsg(c, err, "创建文档附件记录失败")
		response.InternalError(c)
		return
	}

	publicReadable := h.isDocumentPubliclyReadable(c.Request.Context(), documentID)
	publicDownloadURL := ""
	if publicReadable {
		resolvedURL, resolvedURLErr := h.resolveAttachmentPublicDownloadURL(
			c.Request.Context(),
			*attachment,
			config,
			service.DocumentAttachmentLinkPurposeDownload,
		)
		if resolvedURLErr != nil {
			setRequestErrmsg(c, resolvedURLErr, "生成附件公开链接失败")
		} else {
			publicDownloadURL = resolvedURL
		}
	}
	response.JSON(
		c,
		http.StatusOK,
		mapWorkspaceDocumentAttachmentResponse(*attachment, publicDownloadURL, !publicReadable),
	)
}

// DeleteDocumentAttachment 删除文档附件。
// - physicalDelete=false: 仅逻辑删除（status=deleted，保留记录与文件）
// - physicalDelete=true: 物理删除（删除文件并硬删除记录）
func (h *workspaceHandler) DeleteDocumentAttachment(c *gin.Context) {
	actorUserID, ok := h.requireActorUserID(c)
	if !ok {
		return
	}
	if h == nil || h.workspaceRepo == nil || h.documentAttachmentRepo == nil {
		setRequestErrmsgText(c, "初始化失败: handler or dependencies is nil")
		response.InternalError(c)
		return
	}

	documentID := strings.TrimSpace(c.Param("docId"))
	if documentID == "" {
		response.WorkspaceErrDocumentIDRequired.Write(c)
		return
	}
	attachmentID := strings.TrimSpace(c.Param("attachmentId"))
	if attachmentID == "" {
		response.DocumentAttachmentErrAttachmentIDRequired.Write(c)
		return
	}

	currentRecord, err := h.workspaceRepo.GetDocumentByDocumentID(c.Request.Context(), documentID)
	if err != nil {
		setRequestErrmsg(c, err, "查询文档失败")
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.WorkspaceErrDocumentNotFound.Write(c)
			return
		}
		response.InternalError(c)
		return
	}
	spaceID := strings.TrimSpace(currentRecord.SpaceID)
	if _, err := h.ensureSpaceWritable(c.Request.Context(), spaceID, actorUserID); err != nil {
		setRequestErrmsg(c, err, "验证空间权限失败")
		switch {
		case errors.Is(err, service.ErrSpaceNotFound):
			response.WorkspaceErrSpaceNotFound.Write(c)
		case errors.Is(err, service.ErrSpaceAccessDenied):
			response.WorkspaceErrInsufficientSpacePermission.Write(c)
		default:
			response.InternalError(c)
		}
		return
	}

	attachment, err := h.documentAttachmentRepo.GetByAttachmentID(c.Request.Context(), attachmentID)
	if err != nil {
		setRequestErrmsg(c, err, "查询文档附件失败")
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.DocumentAttachmentErrAttachmentNotFound.Write(c)
			return
		}
		response.InternalError(c)
		return
	}
	if strings.TrimSpace(attachment.DocumentID) != documentID {
		response.DocumentAttachmentErrAttachmentNotFound.Write(c)
		return
	}
	physicalDelete := parseBoolLikeValue(c.Query("physicalDelete"))
	if !physicalDelete {
		if attachment.Status == models.EntityStatusDeleted {
			response.JSON(c, http.StatusOK, struct{}{})
			return
		}
		deleted, deleteErr := h.documentAttachmentRepo.SoftDelete(c.Request.Context(), attachmentID, time.Now().UTC())
		if deleteErr != nil {
			setRequestErrmsg(c, deleteErr, "逻辑删除文档附件失败")
			response.InternalError(c)
			return
		}
		if !deleted {
			response.DocumentAttachmentErrAttachmentNotFound.Write(c)
			return
		}
		response.JSON(c, http.StatusOK, struct{}{})
		return
	}

	blobID := strings.TrimSpace(attachment.BlobID)
	if blobID == "" {
		setRequestErrmsgText(c, "附件缺少 blob_id，无法执行物理删除")
		response.InternalError(c)
		return
	}

	hardDeleted, hardDeleteErr := h.documentAttachmentRepo.HardDelete(c.Request.Context(), attachmentID)
	if hardDeleteErr != nil {
		setRequestErrmsg(c, hardDeleteErr, "物理删除附件记录失败")
		response.InternalError(c)
		return
	}
	if !hardDeleted {
		response.DocumentAttachmentErrAttachmentNotFound.Write(c)
		return
	}

	activeRefCount, countRefErr := h.documentAttachmentRepo.CountActiveReferencesByBlobID(c.Request.Context(), blobID)
	if countRefErr != nil {
		setRequestErrmsg(c, countRefErr, "查询附件引用数量失败")
		response.InternalError(c)
		return
	}
	if activeRefCount == 0 {
		blob, getBlobErr := h.documentAttachmentRepo.GetBlobByBlobID(c.Request.Context(), blobID)
		if getBlobErr != nil && !errors.Is(getBlobErr, gorm.ErrRecordNotFound) {
			setRequestErrmsg(c, getBlobErr, "查询附件文件实体失败")
			response.InternalError(c)
			return
		}

		targetAttachment := *attachment
		if blob != nil {
			targetAttachment.StorageProvider = strings.TrimSpace(blob.StorageProvider)
			targetAttachment.ObjectKey = strings.TrimSpace(blob.ObjectKey)
			targetAttachment.ObjectURL = strings.TrimSpace(blob.ObjectURL)
		}
		if strings.TrimSpace(targetAttachment.ObjectKey) != "" {
			if deletePhysicalErr := h.deleteDocumentAttachmentPhysicalObject(c.Request.Context(), &targetAttachment); deletePhysicalErr != nil {
				setRequestErrmsg(c, deletePhysicalErr, "物理删除附件文件失败")
				response.InternalError(c)
				return
			}
		}

		if _, blobDeleteErr := h.documentAttachmentRepo.HardDeleteBlobIfUnreferenced(
			c.Request.Context(),
			blobID,
		); blobDeleteErr != nil {
			setRequestErrmsg(c, blobDeleteErr, "删除附件文件实体失败")
			response.InternalError(c)
			return
		}
	}

	response.JSON(c, http.StatusOK, struct{}{})
}

// RedirectDocumentAttachmentDownload 提供可直接访问的附件下载入口。
// 用于静态链接场景（例如阅读页导出 PDF 后点击附件名称下载）。
func (h *workspaceHandler) RedirectDocumentAttachmentDownload(c *gin.Context) {
	if h == nil || h.documentAttachmentRepo == nil || h.visibilityService == nil || h.attachmentTokenService == nil {
		setRequestErrmsgText(c, "初始化失败: handler or dependencies is nil")
		response.InternalError(c)
		return
	}

	documentID := strings.TrimSpace(c.Param("docId"))
	if documentID == "" {
		response.WorkspaceErrDocumentIDRequired.Write(c)
		return
	}
	attachmentID := strings.TrimSpace(c.Param("attachmentId"))
	if attachmentID == "" {
		response.DocumentAttachmentErrAttachmentIDRequired.Write(c)
		return
	}

	viewerUserID, err := h.resolveOptionalViewerUserID(c)
	if err != nil {
		setRequestErrmsg(c, err, "解析访问令牌失败")
		response.WorkspaceErrAccessToken.Write(c)
		return
	}
	if _, err := h.visibilityService.GetDocument(c.Request.Context(), documentID, viewerUserID); err != nil {
		setRequestErrmsg(c, err, "验证文档访问权限失败")
		writeWorkspaceDocumentAccessError(c, err)
		return
	}

	attachment, err := h.documentAttachmentRepo.GetByAttachmentID(c.Request.Context(), attachmentID)
	if err != nil {
		setRequestErrmsg(c, err, "查询文档附件失败")
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.DocumentAttachmentErrAttachmentNotFound.Write(c)
			return
		}
		response.InternalError(c)
		return
	}
	if strings.TrimSpace(attachment.DocumentID) != documentID || attachment.Status == models.EntityStatusDeleted {
		response.DocumentAttachmentErrAttachmentNotFound.Write(c)
		return
	}

	publicReadable := h.isDocumentPubliclyReadable(c.Request.Context(), documentID)
	isLocalStorageProvider := strings.EqualFold(
		strings.TrimSpace(attachment.StorageProvider),
		string(service.ImageHostingProviderLocal),
	)
	if publicReadable {
		config := service.DefaultImageHostingConfig()
		if h.imageHostingService != nil {
			if loadedConfig, configErr := h.imageHostingService.GetConfig(c.Request.Context()); configErr == nil {
				config = loadedConfig
			} else {
				setRequestErrmsg(c, configErr, "读取图床配置失败")
			}
		}
		publicDownloadURL, publicDownloadURLErr := h.resolveAttachmentPublicDownloadURL(
			c.Request.Context(),
			*attachment,
			config,
			service.DocumentAttachmentLinkPurposeDownload,
		)
		if publicDownloadURLErr != nil {
			setRequestErrmsg(c, publicDownloadURLErr, "生成附件公开链接失败")
		}
		if publicDownloadURL != "" {
			if isLocalStorageProvider {
				// 本地存储下载需要附带 Content-Disposition，需继续走 token 下载链路。
			} else {
				c.Redirect(http.StatusTemporaryRedirect, publicDownloadURL)
				return
			}
		}
	}

	token, _, err := h.attachmentTokenService.Issue(service.IssueDocumentAttachmentDownloadTokenInput{
		AttachmentID: attachmentID,
		DocumentID:   documentID,
		Purpose:      service.DocumentAttachmentLinkPurposeDownload,
	})
	if err != nil {
		setRequestErrmsg(c, err, "签发附件下载令牌失败")
		response.InternalError(c)
		return
	}

	c.Redirect(http.StatusTemporaryRedirect, "/api/attachment-downloads/"+url.PathEscape(token))
}

// CreateDocumentAttachmentAccessLink 生成附件下载或预览访问链接。
func (h *workspaceHandler) CreateDocumentAttachmentAccessLink(c *gin.Context) {
	if h == nil || h.documentAttachmentRepo == nil || h.visibilityService == nil || h.attachmentTokenService == nil {
		setRequestErrmsgText(c, "初始化失败: handler or dependencies is nil")
		response.InternalError(c)
		return
	}

	documentID := strings.TrimSpace(c.Param("docId"))
	if documentID == "" {
		response.WorkspaceErrDocumentIDRequired.Write(c)
		return
	}
	attachmentID := strings.TrimSpace(c.Param("attachmentId"))
	if attachmentID == "" {
		response.DocumentAttachmentErrAttachmentIDRequired.Write(c)
		return
	}

	viewerUserID, err := h.resolveOptionalViewerUserID(c)
	if err != nil {
		setRequestErrmsg(c, err, "解析访问令牌失败")
		response.WorkspaceErrAccessToken.Write(c)
		return
	}
	if _, err := h.visibilityService.GetDocument(c.Request.Context(), documentID, viewerUserID); err != nil {
		setRequestErrmsg(c, err, "验证文档访问权限失败")
		writeWorkspaceDocumentAccessError(c, err)
		return
	}

	attachment, err := h.documentAttachmentRepo.GetByAttachmentID(c.Request.Context(), attachmentID)
	if err != nil {
		setRequestErrmsg(c, err, "查询文档附件失败")
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.DocumentAttachmentErrAttachmentNotFound.Write(c)
			return
		}
		response.InternalError(c)
		return
	}
	if strings.TrimSpace(attachment.DocumentID) != documentID || attachment.Status == models.EntityStatusDeleted {
		response.DocumentAttachmentErrAttachmentNotFound.Write(c)
		return
	}

	rawPurpose := strings.TrimSpace(c.Query("purpose"))
	purpose := normalizeWorkspaceDocumentAttachmentPurpose(rawPurpose)
	if rawPurpose != "" && purpose == "" {
		response.DocumentAttachmentErrInvalidPurpose.Write(c)
		return
	}
	if purpose == "" {
		purpose = workspaceDocumentAttachmentPurposeDownload
	}
	if purpose == workspaceDocumentAttachmentPurposePreview && !isDocumentAttachmentPreviewSupported(attachment.PreviewKind) {
		response.DocumentAttachmentErrPreviewUnsupported.Write(c)
		return
	}

	publicReadable := h.isDocumentPubliclyReadable(c.Request.Context(), documentID)
	isLocalStorageProvider := strings.EqualFold(
		strings.TrimSpace(attachment.StorageProvider),
		string(service.ImageHostingProviderLocal),
	)
	if publicReadable {
		config := service.DefaultImageHostingConfig()
		if h.imageHostingService != nil {
			if loadedConfig, configErr := h.imageHostingService.GetConfig(c.Request.Context()); configErr == nil {
				config = loadedConfig
			} else {
				setRequestErrmsg(c, configErr, "读取图床配置失败")
			}
		}
		publicDownloadURL, publicDownloadURLErr := h.resolveAttachmentPublicDownloadURL(
			c.Request.Context(),
			*attachment,
			config,
			service.DocumentAttachmentLinkPurpose(purpose),
		)
		if publicDownloadURLErr != nil {
			setRequestErrmsg(c, publicDownloadURLErr, "生成附件公开链接失败")
		}
		if publicDownloadURL != "" {
			if purpose == workspaceDocumentAttachmentPurposeDownload && isLocalStorageProvider {
				response.JSON(c, http.StatusOK, workspaceDocumentAttachmentAccessLinkResponse{
					URL: "/api/docs/" +
						url.PathEscape(documentID) +
						"/attachments/" +
						url.PathEscape(attachmentID) +
						"/download",
					Purpose:      string(purpose),
					PreviewKind:  normalizeDocumentAttachmentPreviewKind(attachment.PreviewKind),
					RequiresAuth: false,
				})
				return
			}
			response.JSON(c, http.StatusOK, workspaceDocumentAttachmentAccessLinkResponse{
				URL:          publicDownloadURL,
				Purpose:      string(purpose),
				PreviewKind:  normalizeDocumentAttachmentPreviewKind(attachment.PreviewKind),
				RequiresAuth: false,
			})
			return
		}
	}

	token, expiresAt, err := h.attachmentTokenService.Issue(service.IssueDocumentAttachmentDownloadTokenInput{
		AttachmentID: attachmentID,
		DocumentID:   documentID,
		Purpose:      service.DocumentAttachmentLinkPurpose(purpose),
	})
	if err != nil {
		setRequestErrmsg(c, err, "签发附件访问令牌失败")
		response.InternalError(c)
		return
	}

	response.JSON(c, http.StatusOK, workspaceDocumentAttachmentAccessLinkResponse{
		URL:          "/api/attachment-downloads/" + url.PathEscape(token),
		Purpose:      string(purpose),
		PreviewKind:  normalizeDocumentAttachmentPreviewKind(attachment.PreviewKind),
		RequiresAuth: !publicReadable,
		ExpiresAt:    expiresAt.UTC().Format(time.RFC3339),
	})
}

// ServeDocumentAttachmentByToken 通过签名链接下载或预览附件。
func (h *workspaceHandler) ServeDocumentAttachmentByToken(c *gin.Context) {
	if h == nil || h.documentAttachmentRepo == nil || h.visibilityService == nil || h.attachmentTokenService == nil {
		setRequestErrmsgText(c, "初始化失败: handler or dependencies is nil")
		response.InternalError(c)
		return
	}

	rawToken := strings.TrimSpace(c.Param("token"))
	if rawToken == "" {
		response.DocumentAttachmentErrDownloadLinkInvalid.Write(c)
		return
	}
	claims, err := h.attachmentTokenService.Parse(rawToken)
	if err != nil {
		setRequestErrmsg(c, err, "解析附件访问令牌失败")
		if errors.Is(err, service.ErrDocumentAttachmentDownloadTokenExpired) {
			response.DocumentAttachmentErrDownloadLinkExpired.Write(c)
			return
		}
		response.DocumentAttachmentErrDownloadLinkInvalid.Write(c)
		return
	}

	attachment, err := h.documentAttachmentRepo.GetByAttachmentID(c.Request.Context(), claims.AttachmentID)
	if err != nil {
		setRequestErrmsg(c, err, "查询文档附件失败")
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.DocumentAttachmentErrAttachmentNotFound.Write(c)
			return
		}
		response.InternalError(c)
		return
	}
	if strings.TrimSpace(attachment.DocumentID) != strings.TrimSpace(claims.DocumentID) || attachment.Status == models.EntityStatusDeleted {
		response.DocumentAttachmentErrAttachmentNotFound.Write(c)
		return
	}

	viewerUserID, err := h.resolveOptionalViewerUserID(c)
	if err != nil {
		setRequestErrmsg(c, err, "解析访问令牌失败")
		response.WorkspaceErrAccessToken.Write(c)
		return
	}
	if _, err := h.visibilityService.GetDocument(c.Request.Context(), claims.DocumentID, viewerUserID); err != nil {
		setRequestErrmsg(c, err, "验证文档访问权限失败")
		writeWorkspaceDocumentAccessError(c, err)
		return
	}

	if claims.Purpose == service.DocumentAttachmentLinkPurposePreview &&
		!isDocumentAttachmentPreviewSupported(attachment.PreviewKind) {
		response.DocumentAttachmentErrPreviewUnsupported.Write(c)
		return
	}

	if strings.EqualFold(strings.TrimSpace(attachment.StorageProvider), string(service.ImageHostingProviderLocal)) {
		h.serveLocalDocumentAttachment(c, attachment, claims.Purpose)
		return
	}

	config := service.DefaultImageHostingConfig()
	if h.imageHostingService != nil {
		loadedConfig, configErr := h.imageHostingService.GetConfig(c.Request.Context())
		if configErr != nil {
			setRequestErrmsg(c, configErr, "读取图床配置失败")
			response.InternalError(c)
			return
		}
		config = loadedConfig
	}

	targetURL, targetURLErr := h.resolveNonLocalAttachmentObjectURL(
		c.Request.Context(),
		*attachment,
		config,
		claims.Purpose,
	)
	if targetURLErr != nil {
		setRequestErrmsg(c, targetURLErr, "生成附件下载链接失败")
		response.InternalError(c)
		return
	}
	if targetURL == "" {
		setRequestErrmsgText(c, "附件对象 URL 为空")
		response.DocumentAttachmentErrAttachmentNotFound.Write(c)
		return
	}
	c.Redirect(http.StatusTemporaryRedirect, targetURL)
}

func (h *workspaceHandler) serveLocalDocumentAttachment(
	c *gin.Context,
	attachment *models.DocumentAttachment,
	purpose service.DocumentAttachmentLinkPurpose,
) {
	if c == nil || attachment == nil {
		setRequestErrmsgText(c, "本地附件下载处理参数无效")
		response.InternalError(c)
		return
	}

	targetPath, err := h.resolveLocalAttachmentTargetPath(attachment.ObjectKey)
	if err != nil {
		setRequestErrmsg(c, err, "解析本地附件目标路径失败")
		response.ImageHostingInvalidFilePath(c)
		return
	}
	fileInfo, err := os.Stat(targetPath)
	if err != nil {
		setRequestErrmsg(c, err, "读取本地附件文件失败")
		if errors.Is(err, os.ErrNotExist) {
			response.DocumentAttachmentErrAttachmentNotFound.Write(c)
			return
		}
		response.InternalError(c)
		return
	}
	if fileInfo.IsDir() {
		response.DocumentAttachmentErrAttachmentNotFound.Write(c)
		return
	}

	mimeType := strings.TrimSpace(attachment.MimeType)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	fileName := strings.TrimSpace(attachment.FileName)
	if fileName == "" {
		fileName = strings.TrimSpace(attachment.AttachmentID)
	}
	dispositionType := "attachment"
	if purpose == service.DocumentAttachmentLinkPurposePreview {
		dispositionType = "inline"
	}
	c.Header(
		"Content-Disposition",
		fmt.Sprintf("%s; filename*=UTF-8''%s", dispositionType, url.PathEscape(fileName)),
	)
	c.Header("Content-Type", mimeType)
	c.Header("Cache-Control", "private, no-store, max-age=0")
	c.File(targetPath)
}

func (h *workspaceHandler) resolveOptionalViewerUserID(c *gin.Context) (string, error) {
	rawToken, ok := bearerTokenFromRequest(c)
	if !ok {
		return "", nil
	}
	return h.resolveViewerUserIDByToken(c, rawToken)
}

func (h *workspaceHandler) resolveViewerUserIDByToken(c *gin.Context, rawToken string) (string, error) {
	if h == nil || h.authService == nil {
		return "", errors.New("auth service is nil")
	}
	session, err := h.authService.Me(c.Request.Context(), rawToken)
	if err != nil {
		setRequestErrmsg(c, err, "解析访问令牌失败")
		return "", err
	}
	return strings.TrimSpace(session.User.ID), nil
}

func (h *workspaceHandler) isDocumentPubliclyReadable(ctx context.Context, documentID string) bool {
	if h == nil || h.visibilityService == nil {
		return false
	}
	_, err := h.visibilityService.GetDocument(ctx, documentID, "")
	return err == nil
}

func writeWorkspaceDocumentAccessError(c *gin.Context, err error) {
	setRequestErrmsg(c, err, "文档访问权限校验失败")
	switch {
	case errors.Is(err, service.ErrDocumentNotFound):
		response.WorkspaceErrDocumentNotFound.Write(c)
	case errors.Is(err, service.ErrViewerLoginRequired):
		response.WorkspaceErrLoginRequired.Write(c)
	case errors.Is(err, service.ErrDocumentAccessDenied):
		response.WorkspaceErrInsufficientDocumentPermission.Write(c)
	default:
		response.InternalError(c)
	}
}

func setRequestErrmsg(c *gin.Context, err error, message string) {
	if c == nil || err == nil {
		return
	}
	wrappedErr := err
	normalizedMessage := strings.TrimSpace(message)
	if normalizedMessage != "" {
		wrappedErr = fmt.Errorf("%s: %w", normalizedMessage, err)
	}
	logit.SetRequestAttrs(c.Request.Context(), logit.Error("errmsg", wrappedErr))
}

func setRequestErrmsgText(c *gin.Context, message string) {
	if c == nil {
		return
	}
	normalizedMessage := strings.TrimSpace(message)
	if normalizedMessage == "" {
		return
	}
	logit.SetRequestAttrs(c.Request.Context(), logit.Error("errmsg", errors.New(normalizedMessage)))
}

func mapWorkspaceDocumentAttachmentResponse(
	attachment models.DocumentAttachment,
	publicDownloadURL string,
	requiresAuthDownload bool,
) workspaceDocumentAttachmentResponse {
	previewKind := normalizeDocumentAttachmentPreviewKind(attachment.PreviewKind)
	return workspaceDocumentAttachmentResponse{
		AttachmentID:         strings.TrimSpace(attachment.AttachmentID),
		DocumentID:           strings.TrimSpace(attachment.DocumentID),
		SpaceID:              strings.TrimSpace(attachment.SpaceID),
		FileName:             strings.TrimSpace(attachment.FileName),
		MimeType:             strings.TrimSpace(attachment.MimeType),
		SizeBytes:            attachment.SizeBytes,
		StorageProvider:      strings.TrimSpace(attachment.StorageProvider),
		PreviewKind:          previewKind,
		PreviewSupported:     isDocumentAttachmentPreviewSupported(previewKind),
		RequiresAuthDownload: requiresAuthDownload,
		PublicDownloadURL:    strings.TrimSpace(publicDownloadURL),
		CreatedAt:            attachment.CreatedAt,
		UpdatedAt:            attachment.UpdatedAt,
		DeletedAt:            attachment.DeletedAt,
	}
}

func (h *workspaceHandler) resolveAttachmentPublicDownloadURL(
	ctx context.Context,
	attachment models.DocumentAttachment,
	config service.ImageHostingConfig,
	purpose service.DocumentAttachmentLinkPurpose,
) (string, error) {
	provider := strings.ToLower(strings.TrimSpace(attachment.StorageProvider))
	switch provider {
	case string(service.ImageHostingProviderLocal):
		return resolvePublicURL(config.Local.PublicBaseURL, attachment.ObjectKey, "/uploads"), nil
	default:
		return h.resolveNonLocalAttachmentObjectURL(
			ctx,
			attachment,
			config,
			purpose,
		)
	}
}

func (h *workspaceHandler) resolveNonLocalAttachmentObjectURL(
	ctx context.Context,
	attachment models.DocumentAttachment,
	config service.ImageHostingConfig,
	purpose service.DocumentAttachmentLinkPurpose,
) (string, error) {
	normalizedProvider := service.ImageHostingProvider(strings.ToLower(strings.TrimSpace(attachment.StorageProvider)))
	switch normalizedProvider {
	case service.ImageHostingProviderCloudflareR2, service.ImageHostingProviderAliyunOSS:
	default:
		return "", nil
	}
	if h == nil || h.imageHostingService == nil {
		return "", errors.New("image hosting service is nil")
	}

	return h.imageHostingService.BuildObjectReadURL(ctx, config, service.BuildImageHostingObjectReadURLInput{
		Provider:  normalizedProvider,
		ObjectKey: attachment.ObjectKey,
		ObjectURL: attachment.ObjectURL,
		FileName:  attachment.FileName,
		Purpose:   purpose,
	})
}

func normalizeWorkspaceDocumentAttachmentPurpose(
	rawPurpose string,
) workspaceDocumentAttachmentPurpose {
	switch workspaceDocumentAttachmentPurpose(strings.ToLower(strings.TrimSpace(rawPurpose))) {
	case workspaceDocumentAttachmentPurposeDownload:
		return workspaceDocumentAttachmentPurposeDownload
	case workspaceDocumentAttachmentPurposePreview:
		return workspaceDocumentAttachmentPurposePreview
	default:
		return ""
	}
}

func parseBoolLikeValue(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func isLikelyUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "unique constraint") ||
		strings.Contains(message, "duplicate key") ||
		strings.Contains(message, "duplicated")
}

func computeUploadedFileSHA256(fileHeader *multipart.FileHeader) (string, error) {
	if fileHeader == nil {
		return "", errors.New("file header is nil")
	}
	file, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, copyErr := io.Copy(hash, file); copyErr != nil {
		return "", copyErr
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func detectUploadedFileContentType(fileHeader *multipart.FileHeader) (string, error) {
	if fileHeader == nil {
		return "", errors.New("file header is nil")
	}
	file, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()

	buffer := make([]byte, 512)
	readBytes, readErr := io.ReadFull(file, buffer)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) && !errors.Is(readErr, io.EOF) {
		return "", readErr
	}
	detected := strings.TrimSpace(http.DetectContentType(buffer[:readBytes]))
	if detected == "" {
		detected = "application/octet-stream"
	}
	return detected, nil
}

func (h *workspaceHandler) uploadDocumentAttachmentToProvider(
	c *gin.Context,
	fileHeader *multipart.FileHeader,
	contentType string,
	objectKey string,
	provider service.ImageHostingProvider,
	config service.ImageHostingConfig,
) (string, string, error) {
	if c == nil || fileHeader == nil {
		return "", "", errors.New("attachment upload context is invalid")
	}

	fileContent, readErr := readUploadedFileContent(fileHeader, maxWorkspaceAttachmentSizeBytes)
	if readErr != nil {
		return "", "", readErr
	}
	if len(fileContent) == 0 {
		return "", "", errors.New("attachment content is empty")
	}

	return h.uploadRawContentToProvider(c.Request.Context(), fileContent, contentType, objectKey, provider, config)
}

func buildDocumentAttachmentObjectKey(
	fileName string,
	contentType string,
	spaceID string,
	documentID string,
	uploaderUserID string,
	now time.Time,
	uploadPathTemplate string,
) (string, error) {
	extension := sanitizePathSegment(resolveDocumentAttachmentFileExtension(fileName, contentType), "bin")
	assetID := strings.ToLower(ulid.Make().String())
	replaced, err := service.RenderImageHostingUploadPathTemplate(uploadPathTemplate, map[string]string{
		"spaceId":    sanitizePathSegment(spaceID, "space"),
		"docId":      sanitizePathSegment(documentID, "doc"),
		"yyyy":       fmt.Sprintf("%04d", now.Year()),
		"mm":         fmt.Sprintf("%02d", int(now.Month())),
		"dd":         fmt.Sprintf("%02d", now.Day()),
		"hh":         fmt.Sprintf("%02d", now.Hour()),
		"assetId":    sanitizePathSegment(assetID, "asset"),
		"origName":   sanitizePathSegment(resolveOriginName(fileName), "file"),
		"ext":        extension,
		"uploaderId": sanitizePathSegment(uploaderUserID, "uploader"),
	})
	if err != nil {
		return "", err
	}

	normalizedObjectKey := strings.TrimSpace(strings.TrimPrefix(replaced, "/"))
	if normalizedObjectKey == "" {
		return "", errors.New("attachment object key is empty")
	}
	cleanObjectKey := path.Clean(normalizedObjectKey)
	if cleanObjectKey == "." || cleanObjectKey == "/" || strings.HasPrefix(cleanObjectKey, "../") {
		return "", errors.New("attachment object key is invalid")
	}
	if !strings.HasPrefix(cleanObjectKey, "images/") {
		return "", errors.New("attachment object key must start with images/")
	}
	if len(cleanObjectKey) > 512 {
		return "", errors.New("attachment object key is too long")
	}
	return cleanObjectKey, nil
}

func resolveDocumentAttachmentFileExtension(fileName string, contentType string) string {
	extension := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(path.Ext(fileName)), "."))
	if isSafeFileExtension(extension) {
		return extension
	}
	extensions, err := mime.ExtensionsByType(strings.TrimSpace(contentType))
	if err == nil {
		for _, item := range extensions {
			candidate := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(item), "."))
			if isSafeFileExtension(candidate) {
				return candidate
			}
		}
	}
	return "bin"
}

func normalizeDocumentAttachmentPreviewKind(rawPreviewKind string) string {
	switch strings.ToLower(strings.TrimSpace(rawPreviewKind)) {
	case documentAttachmentPreviewKindImage:
		return documentAttachmentPreviewKindImage
	case documentAttachmentPreviewKindPDF:
		return documentAttachmentPreviewKindPDF
	case documentAttachmentPreviewKindOffice:
		return documentAttachmentPreviewKindOffice
	case documentAttachmentPreviewKindText:
		return documentAttachmentPreviewKindText
	default:
		return documentAttachmentPreviewKindNone
	}
}

func resolveDocumentAttachmentPreviewKind(contentType string, fileName string) string {
	mimeType := strings.ToLower(strings.TrimSpace(contentType))
	if strings.HasPrefix(mimeType, "image/") {
		return documentAttachmentPreviewKindImage
	}
	if mimeType == "application/pdf" {
		return documentAttachmentPreviewKindPDF
	}
	if strings.HasPrefix(mimeType, "text/") || mimeType == "application/json" || mimeType == "application/xml" {
		return documentAttachmentPreviewKindText
	}

	extension := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(path.Ext(fileName)), "."))
	switch extension {
	case "pdf":
		return documentAttachmentPreviewKindPDF
	case "doc", "docx", "xls", "xlsx", "ppt", "pptx":
		return documentAttachmentPreviewKindOffice
	case "md", "markdown", "txt", "json", "xml", "yaml", "yml":
		return documentAttachmentPreviewKindText
	}
	return documentAttachmentPreviewKindNone
}

func isDocumentAttachmentPreviewSupported(previewKind string) bool {
	normalizedPreviewKind := normalizeDocumentAttachmentPreviewKind(previewKind)
	return normalizedPreviewKind == documentAttachmentPreviewKindImage ||
		normalizedPreviewKind == documentAttachmentPreviewKindPDF ||
		normalizedPreviewKind == documentAttachmentPreviewKindOffice ||
		normalizedPreviewKind == documentAttachmentPreviewKindText
}

func (h *workspaceHandler) deleteDocumentAttachmentPhysicalObject(
	ctx context.Context,
	attachment *models.DocumentAttachment,
) error {
	if attachment == nil {
		return errors.New("attachment is nil")
	}

	storageProvider := service.ImageHostingProvider(
		strings.ToLower(strings.TrimSpace(attachment.StorageProvider)),
	)
	if storageProvider == "" {
		storageProvider = service.ImageHostingProviderLocal
	}

	switch storageProvider {
	case service.ImageHostingProviderLocal:
		targetPath, pathErr := h.resolveLocalAttachmentTargetPath(attachment.ObjectKey)
		if pathErr != nil {
			return pathErr
		}
		if removeErr := os.Remove(targetPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return removeErr
		}
		return nil
	case service.ImageHostingProviderCloudflareR2, service.ImageHostingProviderAliyunOSS:
		if h == nil || h.imageHostingService == nil {
			return errors.New("image hosting service is nil")
		}
		config, configErr := h.imageHostingService.GetConfig(ctx)
		if configErr != nil {
			return configErr
		}
		normalizedObjectKey := strings.TrimSpace(attachment.ObjectKey)
		if normalizedObjectKey == "" {
			return errors.New("attachment object key is empty")
		}
		if storageProvider == service.ImageHostingProviderCloudflareR2 {
			return deleteImageFromCloudflareR2(ctx, normalizedObjectKey, config)
		}
		return deleteImageFromAliyunOSS(normalizedObjectKey, config)
	default:
		return errors.New("unsupported attachment storage provider")
	}
}

func (h *workspaceHandler) resolveLocalAttachmentTargetPath(objectKey string) (string, error) {
	normalizedObjectKey := strings.TrimSpace(strings.TrimPrefix(objectKey, "/"))
	if normalizedObjectKey == "" {
		return "", errors.New("object key is empty")
	}
	cleanObjectKey := path.Clean(normalizedObjectKey)
	if cleanObjectKey == "." || cleanObjectKey == "/" || strings.HasPrefix(cleanObjectKey, "../") {
		return "", errors.New("object key is invalid")
	}

	localRootDir := strings.TrimSpace(h.localImageRootDir)
	if localRootDir == "" {
		localRootDir = defaultLocalImageStorageRoot
	}
	targetPath := filepath.Join(localRootDir, filepath.FromSlash(cleanObjectKey))
	targetAbsPath, err := filepath.Abs(targetPath)
	if err != nil {
		return "", err
	}
	rootAbsPath, err := filepath.Abs(localRootDir)
	if err != nil {
		return "", err
	}
	if !isPathWithinRoot(rootAbsPath, targetAbsPath) {
		return "", errors.New("object key is out of root")
	}
	return targetAbsPath, nil
}
