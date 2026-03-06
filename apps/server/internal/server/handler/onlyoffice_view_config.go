package handler

import (
	"context"
	"errors"
	neturl "net/url"
	"strings"

	"github.com/lifei6671/plaindoc/apps/server/internal/service"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
)

var (
	errOnlyOfficeViewConfigNotOfficeDocument = errors.New("document is not an office document")
	errOnlyOfficeViewConfigSourceBlobEmpty   = errors.New("office document source blob is empty")
	errOnlyOfficeViewConfigDisabled          = errors.New("onlyoffice is disabled")
	errOnlyOfficeViewConfigConfigIncomplete  = errors.New("onlyoffice config is enabled but required urls are empty")
)

type onlyOfficeDocumentConfigResponse struct {
	DocumentServerURL string         `json:"documentServerUrl"`
	Config            map[string]any `json:"config"`
}

func buildOnlyOfficeViewConfig(
	ctx context.Context,
	onlyOfficeConfigService *service.OnlyOfficeConfigService,
	onlyOfficeTokenService *service.OnlyOfficeDocumentTokenService,
	document service.ReaderDocumentViewModel,
	actorUserID string,
	actorDisplayName string,
) (onlyOfficeDocumentConfigResponse, error) {
	if onlyOfficeConfigService == nil || onlyOfficeTokenService == nil {
		return onlyOfficeDocumentConfigResponse{}, errors.New("onlyoffice view config dependencies are nil")
	}

	documentFormat := models.NormalizeDocumentFormat(document.Format)
	if !models.IsOfficeDocumentFormat(documentFormat) {
		return onlyOfficeDocumentConfigResponse{}, errOnlyOfficeViewConfigNotOfficeDocument
	}
	if strings.TrimSpace(derefOptionalString(document.SourceBlobID)) == "" {
		return onlyOfficeDocumentConfigResponse{}, errOnlyOfficeViewConfigSourceBlobEmpty
	}

	onlyOfficeConfig, err := onlyOfficeConfigService.GetConfig(ctx)
	if err != nil {
		return onlyOfficeDocumentConfigResponse{}, err
	}
	if !onlyOfficeConfig.Enabled {
		return onlyOfficeDocumentConfigResponse{}, errOnlyOfficeViewConfigDisabled
	}
	if strings.TrimSpace(onlyOfficeConfig.DocumentServerURL) == "" ||
		strings.TrimSpace(onlyOfficeConfig.CallbackPublicBaseURL) == "" {
		return onlyOfficeDocumentConfigResponse{}, errOnlyOfficeViewConfigConfigIncomplete
	}

	contentVersion := normalizeWorkspaceContentVersion(document.ContentVersion, document.Version)
	normalizedActorUserID := strings.TrimSpace(actorUserID)
	sourceToken, _, err := onlyOfficeTokenService.Issue(service.IssueOnlyOfficeDocumentTokenInput{
		DocumentID:     strings.TrimSpace(document.ID),
		ContentVersion: contentVersion,
		ActorUserID:    normalizedActorUserID,
		Purpose:        service.OnlyOfficeDocumentTokenPurposeSource,
	})
	if err != nil {
		return onlyOfficeDocumentConfigResponse{}, err
	}

	sourceURL := buildOnlyOfficeAbsoluteURL(
		onlyOfficeConfig.CallbackPublicBaseURL,
		"/api/docs/"+neturl.PathEscape(strings.TrimSpace(document.ID))+"/onlyoffice/source",
		neturl.Values{onlyOfficeAccessTokenQueryKey: []string{sourceToken}},
	)
	fileName := strings.TrimSpace(derefOptionalString(document.SourceFileName))
	if fileName == "" {
		fileName = resolveOfficeSourceFileName(document.Title, documentFormat)
	}

	editorDisplayName := strings.TrimSpace(actorDisplayName)
	if editorDisplayName == "" {
		editorDisplayName = "访客"
	}
	configPayload := map[string]any{
		"documentType": onlyOfficeDocumentType(documentFormat),
		"document": map[string]any{
			"title":    fileName,
			"url":      sourceURL,
			"fileType": officeDocumentFileExtension(documentFormat),
			"key":      buildOnlyOfficeDocumentKey(strings.TrimSpace(document.ID), contentVersion),
			"permissions": map[string]any{
				"edit":     false,
				"download": true,
				"print":    true,
				"review":   false,
			},
		},
		"editorConfig": map[string]any{
			"mode": "view",
			"lang": "zh-CN",
			"user": map[string]any{
				"id":   normalizedActorUserID,
				"name": editorDisplayName,
			},
			"customization": map[string]any{
				"autosave":   false,
				"submitForm": false,
			},
		},
	}
	if strings.TrimSpace(onlyOfficeConfig.JWTSecret) != "" {
		token, err := signOnlyOfficeConfig(configPayload, onlyOfficeConfig.JWTSecret)
		if err != nil {
			return onlyOfficeDocumentConfigResponse{}, err
		}
		configPayload["token"] = token
	}
	return onlyOfficeDocumentConfigResponse{
		DocumentServerURL: onlyOfficeConfig.DocumentServerURL,
		Config:            configPayload,
	}, nil
}
