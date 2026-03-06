package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage"
)

func TestRouter_CreateNode_WithOfficeFormatInitializesSourceBlobAndFileRevision(t *testing.T) {
	t.Cleanup(func() {
		_ = os.RemoveAll("uploads")
	})

	type testCase struct {
		name             string
		format           string
		title            string
		expectedMimeType string
		expectedFileName string
	}

	cases := []testCase{
		{
			name:             "word document",
			format:           "docx",
			title:            "季度报告",
			expectedMimeType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			expectedFileName: "季度报告.docx",
		},
		{
			name:             "excel workbook",
			format:           "xlsx",
			title:            "季度预算",
			expectedMimeType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			expectedFileName: "季度预算.xlsx",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			database, serve := setupAuthTestRouter(t)
			defer func() {
				_ = database.Close()
			}()

			ownerUserID, _, ownerToken := registerAccessUser(t, serve, "owner-create-node-"+tc.format+"@example.com")
			spaceID := "01h1createoffice" + tc.format + "space000001"
			seedSpaceForWorkspaceCreateNode(t, database, ownerUserID, spaceID, "member")
			seedOnlyOfficeEnabledConfig(t, database)

			body := []byte(`{"parentId":null,"type":"doc","title":"` + tc.title + `","format":"` + tc.format + `"}`)
			req := httptest.NewRequest(http.MethodPost, "/api/spaces/"+spaceID+"/nodes", bytes.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+ownerToken)
			req.Header.Set("Content-Type", "application/json")
			rec := serve(req)
			if rec.Code != http.StatusOK {
				t.Fatalf("expected create node status 200, got %d body=%s", rec.Code, rec.Body.String())
			}

			payload := decodeJSONResultData[struct {
				DocID string `json:"docId"`
			}](t, rec.Body.Bytes())
			if payload.DocID == "" {
				t.Fatalf("expected created doc id, body=%s", rec.Body.String())
			}

			var persistedDoc struct {
				Format         string  `gorm:"column:format"`
				ContentMD      string  `gorm:"column:content_md"`
				Version        int     `gorm:"column:version"`
				ContentVersion int     `gorm:"column:content_version"`
				SourceBlobID   *string `gorm:"column:source_blob_id"`
				SourceFileName *string `gorm:"column:source_file_name"`
				SourceMimeType *string `gorm:"column:source_mime_type"`
			}
			if err := database.ORM.Table("documents").
				Select("format", "content_md", "version", "content_version", "source_blob_id", "source_file_name", "source_mime_type").
				Where("document_id = ?", payload.DocID).
				Take(&persistedDoc).Error; err != nil {
				t.Fatalf("query created document failed: %v", err)
			}
			if persistedDoc.Format != tc.format {
				t.Fatalf("expected document format %s, got %q", tc.format, persistedDoc.Format)
			}
			if persistedDoc.ContentMD != "" {
				t.Fatalf("expected office document markdown content empty, got %q", persistedDoc.ContentMD)
			}
			if persistedDoc.Version != 1 {
				t.Fatalf("expected document version 1, got %d", persistedDoc.Version)
			}
			if persistedDoc.ContentVersion != 1 {
				t.Fatalf("expected document content version 1, got %d", persistedDoc.ContentVersion)
			}
			if persistedDoc.SourceBlobID == nil || strings.TrimSpace(*persistedDoc.SourceBlobID) == "" {
				t.Fatalf("expected source blob id, got %+v", persistedDoc.SourceBlobID)
			}
			if persistedDoc.SourceFileName == nil || *persistedDoc.SourceFileName != tc.expectedFileName {
				t.Fatalf("expected source file name %q, got %+v", tc.expectedFileName, persistedDoc.SourceFileName)
			}
			if persistedDoc.SourceMimeType == nil || *persistedDoc.SourceMimeType != tc.expectedMimeType {
				t.Fatalf("expected source mime type %q, got %+v", tc.expectedMimeType, persistedDoc.SourceMimeType)
			}

			var persistedBlob struct {
				StorageProvider string `gorm:"column:storage_provider"`
				ObjectKey       string `gorm:"column:object_key"`
				ObjectURL       string `gorm:"column:object_url"`
				MimeType        string `gorm:"column:mime_type"`
				SizeBytes       int64  `gorm:"column:size_bytes"`
			}
			if err := database.ORM.Table("file_blobs").
				Select("storage_provider", "object_key", "object_url", "mime_type", "size_bytes").
				Where("blob_id = ?", strings.TrimSpace(*persistedDoc.SourceBlobID)).
				Take(&persistedBlob).Error; err != nil {
				t.Fatalf("query source blob failed: %v", err)
			}
			if persistedBlob.StorageProvider != "local" {
				t.Fatalf("expected local storage provider, got %q", persistedBlob.StorageProvider)
			}
			if persistedBlob.MimeType != tc.expectedMimeType {
				t.Fatalf("expected blob mime type %q, got %q", tc.expectedMimeType, persistedBlob.MimeType)
			}
			if persistedBlob.SizeBytes <= 0 {
				t.Fatalf("expected blob size > 0, got %d", persistedBlob.SizeBytes)
			}
			if !strings.HasPrefix(persistedBlob.ObjectURL, "/uploads/") {
				t.Fatalf("expected local object url under /uploads, got %q", persistedBlob.ObjectURL)
			}

			blobFilePath := filepath.Join("uploads", filepath.FromSlash(strings.TrimSpace(persistedBlob.ObjectKey)))
			if _, err := os.Stat(blobFilePath); err != nil {
				t.Fatalf("expected office template file persisted locally, stat err=%v", err)
			}

			var persistedFileRevision struct {
				BlobID      string `gorm:"column:blob_id"`
				FileName    string `gorm:"column:file_name"`
				MimeType    string `gorm:"column:mime_type"`
				Version     int    `gorm:"column:version"`
				BaseVersion int    `gorm:"column:base_version"`
				Source      string `gorm:"column:source"`
			}
			if err := database.ORM.Table("document_file_revisions").
				Select("blob_id", "file_name", "mime_type", "version", "base_version", "source").
				Where("document_id = ?", payload.DocID).
				Take(&persistedFileRevision).Error; err != nil {
				t.Fatalf("query file revision failed: %v", err)
			}
			if persistedFileRevision.BlobID != strings.TrimSpace(*persistedDoc.SourceBlobID) {
				t.Fatalf("expected file revision blob id %q, got %q", *persistedDoc.SourceBlobID, persistedFileRevision.BlobID)
			}
			if persistedFileRevision.FileName != tc.expectedFileName {
				t.Fatalf("expected file revision name %q, got %q", tc.expectedFileName, persistedFileRevision.FileName)
			}
			if persistedFileRevision.MimeType != tc.expectedMimeType {
				t.Fatalf("expected file revision mime type %q, got %q", tc.expectedMimeType, persistedFileRevision.MimeType)
			}
			if persistedFileRevision.Version != 1 || persistedFileRevision.BaseVersion != 0 {
				t.Fatalf(
					"expected file revision version/baseVersion 1/0, got %d/%d",
					persistedFileRevision.Version,
					persistedFileRevision.BaseVersion,
				)
			}
			if persistedFileRevision.Source != "remote" {
				t.Fatalf("expected file revision source remote, got %q", persistedFileRevision.Source)
			}
		})
	}
}

func TestRouter_SaveDocument_RejectsOfficeDocument(t *testing.T) {
	t.Cleanup(func() {
		_ = os.RemoveAll("uploads")
	})

	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "owner-save-office@example.com")
	spaceID := "01h1saveofficespace0000000000001"
	seedSpaceForWorkspaceCreateNode(t, database, ownerUserID, spaceID, "member")
	seedOnlyOfficeEnabledConfig(t, database)

	createBody := []byte(`{"parentId":null,"type":"doc","title":"合同正文","format":"docx"}`)
	createReq := httptest.NewRequest(http.MethodPost, "/api/spaces/"+spaceID+"/nodes", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+ownerToken)
	createReq.Header.Set("Content-Type", "application/json")
	createRec := serve(createReq)
	if createRec.Code != http.StatusOK {
		t.Fatalf("expected create node status 200, got %d body=%s", createRec.Code, createRec.Body.String())
	}

	createPayload := decodeJSONResultData[struct {
		DocID string `json:"docId"`
	}](t, createRec.Body.Bytes())
	if createPayload.DocID == "" {
		t.Fatalf("expected created doc id, body=%s", createRec.Body.String())
	}

	saveBody := []byte(`{"contentMd":"# should fail","baseVersion":1}`)
	saveReq := httptest.NewRequest(http.MethodPut, "/api/docs/"+createPayload.DocID, bytes.NewReader(saveBody))
	saveReq.Header.Set("Authorization", "Bearer "+ownerToken)
	saveReq.Header.Set("Content-Type", "application/json")
	saveRec := serve(saveReq)
	if saveRec.Code != http.StatusOK {
		t.Fatalf("expected save status 200, got %d body=%s", saveRec.Code, saveRec.Body.String())
	}

	expectedCode := response.ResolveErrorCode(response.CodeInvalidRequest)
	if code := decodeJSONResultCode(t, saveRec.Body.Bytes()); code != expectedCode {
		t.Fatalf("expected invalid request code %d, got %d body=%s", expectedCode, code, saveRec.Body.String())
	}
}

func TestRouter_CreateNode_RejectsOfficeFormatWhenOnlyOfficeDisabled(t *testing.T) {
	t.Cleanup(func() {
		_ = os.RemoveAll("uploads")
	})

	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "owner-create-office-disabled@example.com")
	spaceID := "01h1officefeaturedisabled000001"
	seedSpaceForWorkspaceCreateNode(t, database, ownerUserID, spaceID, "member")

	createBody := []byte(`{"parentId":null,"type":"doc","title":"预算表","format":"xlsx"}`)
	createReq := httptest.NewRequest(http.MethodPost, "/api/spaces/"+spaceID+"/nodes", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+ownerToken)
	createReq.Header.Set("Content-Type", "application/json")
	createRec := serve(createReq)
	if createRec.Code != http.StatusOK {
		t.Fatalf("expected create node status 200, got %d body=%s", createRec.Code, createRec.Body.String())
	}

	expectedCode := response.ResolveErrorCode(response.CodeInvalidRequest)
	if code := decodeJSONResultCode(t, createRec.Body.Bytes()); code != expectedCode {
		t.Fatalf("expected invalid request code %d, got %d body=%s", expectedCode, code, createRec.Body.String())
	}
	if !strings.Contains(createRec.Body.String(), "ONLYOFFICE 未启用") {
		t.Fatalf("expected response mention onlyoffice disabled, body=%s", createRec.Body.String())
	}
}

func seedOnlyOfficeEnabledConfig(t *testing.T, database *storage.Database) {
	t.Helper()

	now := time.Now().UTC()
	if err := database.ORM.Table("system_configs").Where("config_key = ?", "onlyoffice").Delete(nil).Error; err != nil {
		t.Fatalf("clear onlyoffice system config failed: %v", err)
	}
	if err := database.ORM.Table("system_configs").Create(map[string]any{
		"config_key": "onlyoffice",
		"config_value_json": `{
			"enabled":true,
			"documentServerUrl":"https://onlyoffice.example.com",
			"callbackPublicBaseUrl":"https://api.example.com",
			"jwtSecret":"onlyoffice-secret"
		}`,
		"version":            1,
		"updated_by_user_id": nil,
		"created_at":         now,
		"updated_at":         now,
	}).Error; err != nil {
		t.Fatalf("seed onlyoffice system config failed: %v", err)
	}
}
