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

	_, _, accessToken := registerAccessUser(t, serve, "image-hosting@example.com")

	getConfigReq := httptest.NewRequest(http.MethodGet, "/api/image-hosting", nil)
	getConfigReq.Header.Set("Authorization", "Bearer "+accessToken)
	getConfigRec := serve(getConfigReq)
	if getConfigRec.Code != http.StatusOK {
		t.Fatalf("expected get image-hosting config status 200, got %d body=%s", getConfigRec.Code, getConfigRec.Body.String())
	}

	var configPayload struct {
		DefaultProvider string `json:"defaultProvider"`
		Local           struct {
			UploadEndpoint string `json:"uploadEndpoint"`
			PublicBaseURL  string `json:"publicBaseUrl"`
		} `json:"local"`
	}
	if err := json.Unmarshal(getConfigRec.Body.Bytes(), &configPayload); err != nil {
		t.Fatalf("decode image-hosting config response failed: %v", err)
	}
	if configPayload.DefaultProvider != "local" {
		t.Fatalf("expected default provider local, got %s", configPayload.DefaultProvider)
	}
	if configPayload.Local.UploadEndpoint != "/api/uploads/images" {
		t.Fatalf("expected local upload endpoint /api/uploads/images, got %s", configPayload.Local.UploadEndpoint)
	}
	if configPayload.Local.PublicBaseURL != "/api/uploads/local" {
		t.Fatalf("expected local public base url /api/uploads/local, got %s", configPayload.Local.PublicBaseURL)
	}

	imageBytes := decodeTinyPNG(t)
	uploadReq := buildImageUploadRequest(t, "/api/uploads/images", "demo.png", imageBytes)
	uploadReq.Header.Set("Authorization", "Bearer "+accessToken)
	uploadRec := serve(uploadReq)
	if uploadRec.Code != http.StatusOK {
		t.Fatalf("expected local upload status 200, got %d body=%s", uploadRec.Code, uploadRec.Body.String())
	}

	var uploadPayload struct {
		Key string `json:"key"`
		URL string `json:"url"`
	}
	if err := json.Unmarshal(uploadRec.Body.Bytes(), &uploadPayload); err != nil {
		t.Fatalf("decode local upload response failed: %v", err)
	}
	if uploadPayload.Key == "" || uploadPayload.URL == "" {
		t.Fatalf("expected upload key/url in response, got %+v", uploadPayload)
	}
	if !strings.HasPrefix(uploadPayload.URL, "/api/uploads/local/") {
		t.Fatalf("expected local upload url prefix /api/uploads/local/, got %s", uploadPayload.URL)
	}

	fetchUploadedReq := httptest.NewRequest(http.MethodGet, uploadPayload.URL, nil)
	fetchUploadedRec := serve(fetchUploadedReq)
	if fetchUploadedRec.Code != http.StatusOK {
		t.Fatalf("expected fetch local upload status 200, got %d body=%s", fetchUploadedRec.Code, fetchUploadedRec.Body.String())
	}
	if !strings.HasPrefix(fetchUploadedRec.Header().Get("Content-Type"), "image/") {
		t.Fatalf("expected image content type, got %s", fetchUploadedRec.Header().Get("Content-Type"))
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

	disabledUploadReq := buildImageUploadRequest(t, "/api/uploads/images", "demo.png", imageBytes)
	disabledUploadReq.Header.Set("Authorization", "Bearer "+accessToken)
	disabledUploadRec := serve(disabledUploadReq)
	if disabledUploadRec.Code != http.StatusBadRequest {
		t.Fatalf("expected upload disabled status 400, got %d body=%s", disabledUploadRec.Code, disabledUploadRec.Body.String())
	}
}

func buildImageUploadRequest(
	t *testing.T,
	uploadPath string,
	fileName string,
	content []byte,
) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
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

func decodeTinyPNG(t *testing.T) []byte {
	t.Helper()

	content, err := base64.StdEncoding.DecodeString(tinyPNGBase64)
	if err != nil {
		t.Fatalf("decode tiny png failed: %v", err)
	}
	return content
}
