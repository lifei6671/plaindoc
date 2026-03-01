package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage"
)

const tinyPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+X7uQAAAAASUVORK5CYII="

func TestRouter_ImageHostingConfigAndLocalUpload(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()
	defer func() {
		_ = os.RemoveAll("uploads")
	}()

	ownerUserID, _, accessToken := registerAccessUser(t, serve, "image-hosting@example.com")
	spaceID := "01kz8j1x8s0c9n6f2m4b7v3q5r"
	seedImageUploadSpace(t, database, spaceID, ownerUserID)

	getConfigReq := httptest.NewRequest(http.MethodGet, "/api/image-hosting", nil)
	getConfigReq.Header.Set("Authorization", "Bearer "+accessToken)
	getConfigRec := serve(getConfigReq)
	if getConfigRec.Code != http.StatusOK {
		t.Fatalf("expected get image-hosting config status 200, got %d body=%s", getConfigRec.Code, getConfigRec.Body.String())
	}

	configPayload := decodeJSONResultData[map[string]any](t, getConfigRec.Body.Bytes())
	localConfig, ok := configPayload["local"].(map[string]any)
	if !ok || localConfig == nil {
		t.Fatalf("expected local config object in response, got %+v", configPayload)
	}
	uploadEndpoint, _ := localConfig["uploadEndpoint"].(string)
	publicBaseURL, _ := localConfig["publicBaseUrl"].(string)
	if uploadEndpoint != "/api/uploads/images" {
		t.Fatalf("expected local upload endpoint /api/uploads/images, got %s", uploadEndpoint)
	}
	if publicBaseURL != "/uploads" {
		t.Fatalf("expected local public base url /uploads, got %s", publicBaseURL)
	}
	if _, exists := configPayload["cloudflareR2"]; exists {
		t.Fatalf("unexpected cloudflareR2 config in client response: %+v", configPayload["cloudflareR2"])
	}
	if _, exists := configPayload["aliyunOss"]; exists {
		t.Fatalf("unexpected aliyunOss config in client response: %+v", configPayload["aliyunOss"])
	}
	defaultProvider, _ := configPayload["defaultProvider"].(string)
	if defaultProvider != "local" {
		t.Fatalf("expected default provider local, got %s", defaultProvider)
	}
	objectKeyEndpoint, _ := configPayload["objectKeyEndpoint"].(string)
	if objectKeyEndpoint != "/api/uploads/images/object-key" {
		t.Fatalf("expected object key endpoint /api/uploads/images/object-key, got %s", objectKeyEndpoint)
	}

	issuePayload := map[string]any{
		"provider":    "cloudflare-r2",
		"spaceId":     spaceID,
		"docId":       "01kz8j1x8s0c9n6f2m4b7v3q5s",
		"fileName":    "demo-image.png",
		"contentType": "image/png",
	}
	issueBody, _ := json.Marshal(issuePayload)
	issueReq := httptest.NewRequest(http.MethodPost, "/api/uploads/images/object-key", bytes.NewReader(issueBody))
	issueReq.Header.Set("Authorization", "Bearer "+accessToken)
	issueReq.Header.Set("Content-Type", "application/json")
	issueRec := serve(issueReq)
	if issueRec.Code != http.StatusOK {
		t.Fatalf("expected issue image object key status 200, got %d body=%s", issueRec.Code, issueRec.Body.String())
	}
	issuedPayload := decodeJSONResultData[struct {
		Provider string `json:"provider"`
		Key      string `json:"key"`
	}](t, issueRec.Body.Bytes())
	if issuedPayload.Provider != "cloudflare-r2" {
		t.Fatalf("expected issued provider cloudflare-r2, got %s", issuedPayload.Provider)
	}
	if !strings.HasPrefix(issuedPayload.Key, "images/") || !strings.Contains(issuedPayload.Key, "/"+spaceID+"/") {
		t.Fatalf("expected issued key contain images prefix and space id, got %s", issuedPayload.Key)
	}

	imageBytes := decodeTinyPNG(t)
	uploadReq := buildImageUploadRequest(t, "/api/uploads/images", "demo.png", imageBytes, map[string]string{
		"spaceId": spaceID,
	})
	uploadReq.Header.Set("Authorization", "Bearer "+accessToken)
	uploadRec := serve(uploadReq)
	if uploadRec.Code != http.StatusOK {
		t.Fatalf("expected local upload status 200, got %d body=%s", uploadRec.Code, uploadRec.Body.String())
	}

	uploadPayload := decodeJSONResultData[struct {
		Key string `json:"key"`
		URL string `json:"url"`
	}](t, uploadRec.Body.Bytes())
	if uploadPayload.Key == "" || uploadPayload.URL == "" {
		t.Fatalf("expected upload key/url in response, got %+v", uploadPayload)
	}
	if !strings.HasPrefix(uploadPayload.URL, "/uploads/") {
		t.Fatalf("expected local upload url prefix /uploads/, got %s", uploadPayload.URL)
	}

	fetchUploadedReq := httptest.NewRequest(http.MethodGet, uploadPayload.URL, nil)
	fetchUploadedRec := serve(fetchUploadedReq)
	if fetchUploadedRec.Code != http.StatusOK {
		t.Fatalf("expected fetch local upload status 200, got %d body=%s", fetchUploadedRec.Code, fetchUploadedRec.Body.String())
	}
	if !strings.HasPrefix(fetchUploadedRec.Header().Get("Content-Type"), "image/") {
		t.Fatalf("expected image content type, got %s", fetchUploadedRec.Header().Get("Content-Type"))
	}
	legacyPath := strings.Replace(uploadPayload.URL, "/uploads/", "/uploads/local/", 1)
	fetchLegacyReq := httptest.NewRequest(http.MethodGet, legacyPath, nil)
	fetchLegacyRec := serve(fetchLegacyReq)
	if fetchLegacyRec.Code != http.StatusOK {
		t.Fatalf("expected fetch legacy local upload status 200, got %d body=%s", fetchLegacyRec.Code, fetchLegacyRec.Body.String())
	}

	now := time.Now().UTC()
	configValueJSON := `{"defaultProvider":"cloudflare-r2","cloudflareR2":{"accountId":"acc","bucket":"bucket","accessKeyId":"key","secretAccessKey":"secret","publicBaseUrl":"https://img.example.com"},"aliyunOss":{"region":"","bucket":"","endpoint":"","accessKeyId":"","accessKeySecret":"","publicBaseUrl":""},"local":{"uploadEndpoint":"/api/uploads/images","publicBaseUrl":"/api/uploads/local"}}`
	if err := database.ORM.Table("system_configs").Create(map[string]any{
		"config_key":         "image-hosting",
		"config_value_json":  configValueJSON,
		"version":            1,
		"updated_by_user_id": nil,
		"created_at":         now,
		"updated_at":         now,
	}).Error; err != nil {
		t.Fatalf("insert image-hosting system config failed: %v", err)
	}
	getConfigAfterInsertReq := httptest.NewRequest(http.MethodGet, "/api/image-hosting", nil)
	getConfigAfterInsertReq.Header.Set("Authorization", "Bearer "+accessToken)
	getConfigAfterInsertRec := serve(getConfigAfterInsertReq)
	if getConfigAfterInsertRec.Code != http.StatusOK {
		t.Fatalf("expected get image-hosting config status 200 after insert, got %d body=%s", getConfigAfterInsertRec.Code, getConfigAfterInsertRec.Body.String())
	}
	configAfterInsertPayload := decodeJSONResultData[map[string]any](t, getConfigAfterInsertRec.Body.Bytes())
	insertedLocalConfig, ok := configAfterInsertPayload["local"].(map[string]any)
	if !ok || insertedLocalConfig == nil {
		t.Fatalf("expected local config object after insert, got %+v", configAfterInsertPayload)
	}
	insertedPublicBaseURL, _ := insertedLocalConfig["publicBaseUrl"].(string)
	if insertedPublicBaseURL != "/uploads" {
		t.Fatalf("expected normalized local public base url /uploads, got %s", insertedPublicBaseURL)
	}
	if _, exists := configAfterInsertPayload["cloudflareR2"]; exists {
		t.Fatalf("unexpected cloudflareR2 config leak after insert: %+v", configAfterInsertPayload["cloudflareR2"])
	}
	if _, exists := configAfterInsertPayload["aliyunOss"]; exists {
		t.Fatalf("unexpected aliyunOss config leak after insert: %+v", configAfterInsertPayload["aliyunOss"])
	}

}

func TestRouter_ImageUploadSpacePermission(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()
	defer func() {
		_ = os.RemoveAll("uploads")
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "upload-owner@example.com")
	readerUserID, _, readerToken := registerAccessUser(t, serve, "upload-reader@example.com")
	collaboratorUserID, _, collaboratorToken := registerAccessUser(t, serve, "upload-collaborator@example.com")

	spaceID := "01kz8j7yp4d2x3m6n9c1v5b8q0"
	seedImageUploadSpace(t, database, spaceID, ownerUserID)
	seedImageUploadMember(t, database, spaceID, readerUserID, "reader")
	seedImageUploadMember(t, database, spaceID, collaboratorUserID, "collaborator")

	imageBytes := decodeTinyPNG(t)

	ownerReq := buildImageUploadRequest(t, "/api/uploads/images", "owner.png", imageBytes, map[string]string{
		"spaceId": spaceID,
	})
	ownerReq.Header.Set("Authorization", "Bearer "+ownerToken)
	ownerRec := serve(ownerReq)
	if ownerRec.Code != http.StatusOK {
		t.Fatalf("expected owner upload status 200, got %d body=%s", ownerRec.Code, ownerRec.Body.String())
	}

	collaboratorReq := buildImageUploadRequest(t, "/api/uploads/images", "collaborator.png", imageBytes, map[string]string{
		"spaceId": spaceID,
	})
	collaboratorReq.Header.Set("Authorization", "Bearer "+collaboratorToken)
	collaboratorRec := serve(collaboratorReq)
	if collaboratorRec.Code != http.StatusOK {
		t.Fatalf("expected collaborator upload status 200, got %d body=%s", collaboratorRec.Code, collaboratorRec.Body.String())
	}

	readerReq := buildImageUploadRequest(t, "/api/uploads/images", "reader.png", imageBytes, map[string]string{
		"spaceId": spaceID,
	})
	readerReq.Header.Set("Authorization", "Bearer "+readerToken)
	readerRec := serve(readerReq)
	if readerRec.Code != http.StatusForbidden {
		t.Fatalf("expected reader upload status 403, got %d body=%s", readerRec.Code, readerRec.Body.String())
	}
}

func buildImageUploadRequest(
	t *testing.T,
	uploadPath string,
	fileName string,
	content []byte,
	formFields map[string]string,
) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range formFields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("write multipart field %s failed: %v", key, err)
		}
	}
	fileWriter, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		t.Fatalf("create multipart file failed: %v", err)
	}
	if _, err := fileWriter.Write(content); err != nil {
		t.Fatalf("write multipart file failed: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, uploadPath, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func seedImageUploadSpace(t *testing.T, database *storage.Database, spaceID string, ownerUserID string) {
	t.Helper()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := database.ORM.Table("spaces").Create(map[string]any{
		"space_id":      spaceID,
		"name":          "Image Upload Space",
		"owner_user_id": ownerUserID,
		"visibility":    "member",
		"created_at":    now,
		"updated_at":    now,
	}).Error; err != nil {
		t.Fatalf("insert image upload space failed: %v", err)
	}
}

func seedImageUploadMember(
	t *testing.T,
	database *storage.Database,
	spaceID string,
	userID string,
	role string,
) {
	t.Helper()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := database.ORM.Table("space_members").Create(map[string]any{
		"space_id":   spaceID,
		"user_id":    userID,
		"role":       role,
		"created_at": now,
		"updated_at": now,
	}).Error; err != nil {
		t.Fatalf("insert image upload member failed: %v", err)
	}
}

func decodeTinyPNG(t *testing.T) []byte {
	t.Helper()

	content, err := base64.StdEncoding.DecodeString(tinyPNGBase64)
	if err != nil {
		t.Fatalf("decode tiny png failed: %v", err)
	}
	return content
}
