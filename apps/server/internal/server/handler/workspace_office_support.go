package handler

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/service"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"
)

const (
	officeDocumentMIMEDOCX = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	officeDocumentMIMEXLSX = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
)

var errWorkspaceDocumentFormatInvalid = errors.New("workspace document format invalid")

//go:embed office_templates/*.docx office_templates/*.xlsx
var officeTemplateFS embed.FS

type builtinOfficeTemplate struct {
	fileName string
	mimeType string
	content  []byte
}

func resolveCreateNodeDocumentFormat(rawFormat *models.DocumentFormat) (models.DocumentFormat, error) {
	if rawFormat == nil {
		return models.DocumentFormatMarkdown, nil
	}

	normalized := models.DocumentFormat(strings.ToLower(strings.TrimSpace(string(*rawFormat))))
	if !models.IsValidDocumentFormat(normalized) {
		return "", errWorkspaceDocumentFormatInvalid
	}
	return normalized, nil
}

func loadBuiltinOfficeTemplate(format models.DocumentFormat) (builtinOfficeTemplate, error) {
	switch format {
	case models.DocumentFormatDOCX:
		content, err := officeTemplateFS.ReadFile("office_templates/blank.docx")
		if err != nil {
			return builtinOfficeTemplate{}, err
		}
		return builtinOfficeTemplate{
			fileName: "blank.docx",
			mimeType: officeDocumentMIMEDOCX,
			content:  content,
		}, nil
	case models.DocumentFormatXLSX:
		content, err := officeTemplateFS.ReadFile("office_templates/blank.xlsx")
		if err != nil {
			return builtinOfficeTemplate{}, err
		}
		return builtinOfficeTemplate{
			fileName: "blank.xlsx",
			mimeType: officeDocumentMIMEXLSX,
			content:  content,
		}, nil
	default:
		return builtinOfficeTemplate{}, fmt.Errorf("unsupported office document format: %s", format)
	}
}

func resolveOfficeSourceFileName(title string, format models.DocumentFormat) string {
	baseName := strings.TrimSpace(title)
	baseName = strings.ReplaceAll(baseName, "/", "-")
	baseName = strings.ReplaceAll(baseName, "\\", "-")
	baseName = strings.Join(strings.Fields(baseName), " ")
	if baseName == "" {
		if format == models.DocumentFormatXLSX {
			baseName = "未命名表格"
		} else {
			baseName = "未命名文档"
		}
	}
	return baseName + "." + officeDocumentFileExtension(format)
}

func officeDocumentFileExtension(format models.DocumentFormat) string {
	switch format {
	case models.DocumentFormatXLSX:
		return "xlsx"
	default:
		return "docx"
	}
}

func (h *workspaceHandler) bootstrapOfficeDocumentSource(
	ctx context.Context,
	spaceID string,
	documentID string,
	actorUserID string,
	title string,
	format models.DocumentFormat,
	now time.Time,
) (*models.DocumentAttachmentBlob, string, string, error) {
	if h == nil || h.documentAttachmentRepo == nil {
		return nil, "", "", errors.New("document attachment repository is nil")
	}

	template, err := loadBuiltinOfficeTemplate(format)
	if err != nil {
		return nil, "", "", err
	}

	fileName := resolveOfficeSourceFileName(title, format)
	blob, err := h.ensureBlobForContent(
		ctx,
		template.content,
		template.mimeType,
		fileName,
		spaceID,
		documentID,
		actorUserID,
		now,
	)
	if err != nil {
		return nil, "", "", err
	}
	return blob, fileName, template.mimeType, nil
}

func (h *workspaceHandler) ensureBlobForContent(
	ctx context.Context,
	content []byte,
	contentType string,
	fileName string,
	spaceID string,
	documentID string,
	actorUserID string,
	now time.Time,
) (*models.DocumentAttachmentBlob, error) {
	if h == nil || h.documentAttachmentRepo == nil {
		return nil, errors.New("document attachment repository is nil")
	}
	if len(content) == 0 {
		return nil, errors.New("content is empty")
	}

	config := service.DefaultImageHostingConfig()
	if h.imageHostingService != nil {
		loadedConfig, err := h.imageHostingService.GetConfig(ctx)
		if err != nil {
			return nil, err
		}
		config = loadedConfig
	}

	targetProvider := config.DefaultProvider
	if targetProvider == "" {
		targetProvider = service.ImageHostingProviderLocal
	}

	hashValue := sha256.Sum256(content)
	contentHash := hex.EncodeToString(hashValue[:])
	contentSize := int64(len(content))
	blob, err := h.documentAttachmentRepo.FindBlobByHash(
		ctx,
		string(targetProvider),
		"sha256",
		contentHash,
		contentSize,
	)
	if err == nil {
		return blob, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	objectKey, err := buildDocumentAttachmentObjectKey(
		fileName,
		contentType,
		spaceID,
		documentID,
		actorUserID,
		now,
		config.AttachmentUploadPathTemplate(targetProvider),
	)
	if err != nil {
		return nil, err
	}

	objectURL, savedTargetPath, err := h.uploadRawContentToProvider(
		ctx,
		content,
		contentType,
		objectKey,
		targetProvider,
		config,
	)
	if err != nil {
		return nil, err
	}

	blobCandidate := &models.DocumentAttachmentBlob{
		BlobID:          strings.ToLower(ulid.Make().String()),
		StorageProvider: string(targetProvider),
		ObjectKey:       objectKey,
		ObjectURL:       strings.TrimSpace(objectURL),
		MimeType:        strings.TrimSpace(contentType),
		SizeBytes:       contentSize,
		ContentHashAlgo: "sha256",
		ContentHash:     contentHash,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if blobCandidate.MimeType == "" {
		blobCandidate.MimeType = "application/octet-stream"
	}

	if err := h.documentAttachmentRepo.CreateBlob(ctx, blobCandidate); err != nil {
		if savedTargetPath != "" {
			if cleanupErr := os.Remove(savedTargetPath); cleanupErr != nil && !errors.Is(cleanupErr, os.ErrNotExist) {
				return nil, cleanupErr
			}
		}
		if !isLikelyUniqueConstraintError(err) {
			return nil, err
		}
		existingBlob, lookupErr := h.documentAttachmentRepo.FindBlobByHash(
			ctx,
			string(targetProvider),
			"sha256",
			contentHash,
			contentSize,
		)
		if lookupErr != nil {
			return nil, lookupErr
		}
		return existingBlob, nil
	}

	return blobCandidate, nil
}

func (h *workspaceHandler) uploadRawContentToProvider(
	ctx context.Context,
	fileContent []byte,
	contentType string,
	objectKey string,
	provider service.ImageHostingProvider,
	config service.ImageHostingConfig,
) (string, string, error) {
	if len(fileContent) == 0 {
		return "", "", errors.New("attachment content is empty")
	}

	switch provider {
	case service.ImageHostingProviderLocal:
		targetPath, pathErr := h.resolveLocalAttachmentTargetPath(objectKey)
		if pathErr != nil {
			return "", "", pathErr
		}
		targetDir := filepath.Dir(targetPath)
		if mkdirErr := os.MkdirAll(targetDir, 0o755); mkdirErr != nil {
			return "", "", mkdirErr
		}
		if saveErr := os.WriteFile(targetPath, fileContent, 0o644); saveErr != nil {
			return "", "", saveErr
		}
		return resolvePublicURL(config.Local.PublicBaseURL, objectKey, "/uploads"), targetPath, nil
	case service.ImageHostingProviderCloudflareR2:
		uploadedURL, uploadErr := uploadImageToCloudflareR2(ctx, fileContent, contentType, objectKey, config)
		return uploadedURL, "", uploadErr
	case service.ImageHostingProviderAliyunOSS:
		uploadedURL, uploadErr := uploadImageToAliyunOSS(fileContent, contentType, objectKey, config)
		return uploadedURL, "", uploadErr
	default:
		return "", "", errors.New("unsupported attachment storage provider")
	}
}
