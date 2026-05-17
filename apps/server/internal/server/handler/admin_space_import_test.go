package handler

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
)

func TestAdminSpaceImportHandler_InspectRejectsOversizedMultipartBeforeInspect(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() {
		_ = os.RemoveAll("data")
	})

	body, contentType := buildAdminSpaceImportHandlerMultipart(t, buildAdminSpaceImportHandlerPackage(t))
	previousMaxRequestBytes := adminSpaceImportMaxRequestBytes
	previousMultipartMemory := adminSpaceImportMultipartMemory
	adminSpaceImportMaxRequestBytes = int64(body.Len() - 1)
	adminSpaceImportMultipartMemory = 64
	t.Cleanup(func() {
		adminSpaceImportMaxRequestBytes = previousMaxRequestBytes
		adminSpaceImportMultipartMemory = previousMultipartMemory
	})

	router := gin.New()
	router.POST("/inspect", func(c *gin.Context) {
		c.Set("admin_actor_user_id", "actor-user")
		NewAdminSpaceImportHandler(service.NewAdminSpaceImportService(nil)).Inspect(c)
	})

	request := httptest.NewRequest(http.MethodPost, "/inspect", bytes.NewReader(body.Bytes()))
	request.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if !strings.Contains(response.Body.String(), "导入 zip 无效") {
		t.Fatalf("expected oversized multipart to be rejected before inspect, got status=%d body=%s", response.Code, response.Body.String())
	}
}

func buildAdminSpaceImportHandlerMultipart(t *testing.T, payload []byte) (*bytes.Buffer, string) {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "space.plaindoc")
	if err != nil {
		t.Fatalf("create multipart file failed: %v", err)
	}
	if _, err := part.Write(payload); err != nil {
		t.Fatalf("write multipart payload failed: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer failed: %v", err)
	}
	return body, writer.FormDataContentType()
}

func buildAdminSpaceImportHandlerPackage(t *testing.T) []byte {
	t.Helper()

	const (
		prefix     = "space-space-source"
		documentID = "doc-a"
		nodeID     = "node-a"
		content    = "# A"
	)
	sum := sha256.Sum256([]byte(content))
	manifest := map[string]any{
		"version":     service.AdminSpaceExportPackageVersion,
		"packageType": service.AdminSpaceExportPackageType,
		"exportedAt":  time.Now().UTC().Format(time.RFC3339),
		"format":      string(service.AdminSpaceExportFormatSourceZip),
		"importable":  true,
		"space": map[string]any{
			"spaceId":    "space-source",
			"name":       "源空间",
			"visibility": "member",
		},
		"summary": map[string]any{
			"folderCount":       0,
			"documentCount":     1,
			"attachmentCount":   0,
			"officeSourceCount": 0,
		},
		"documents": []map[string]any{
			{
				"documentId":    documentID,
				"nodeId":        nodeID,
				"title":         "A",
				"format":        "markdown",
				"visibility":    "member",
				"path":          "documents/doc-a.md",
				"contentSha256": hex.EncodeToString(sum[:]),
				"attachments":   []string{},
				"source":        nil,
			},
		},
	}
	tree := map[string]any{
		"version": service.AdminSpaceExportPackageVersion,
		"root": []map[string]any{
			{
				"nodeId":     nodeID,
				"documentId": documentID,
				"type":       "doc",
				"title":      "A",
				"sort":       1,
				"format":     "markdown",
			},
		},
	}

	buffer := &bytes.Buffer{}
	zipWriter := zip.NewWriter(buffer)
	writeAdminSpaceImportHandlerJSON(t, zipWriter, prefix+"/manifest.json", manifest)
	writeAdminSpaceImportHandlerJSON(t, zipWriter, prefix+"/tree.json", tree)
	writeAdminSpaceImportHandlerFile(t, zipWriter, prefix+"/documents/doc-a.md", []byte(content))
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close import package failed: %v", err)
	}
	return buffer.Bytes()
}

func writeAdminSpaceImportHandlerJSON(t *testing.T, zipWriter *zip.Writer, name string, value any) {
	t.Helper()

	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal %s failed: %v", name, err)
	}
	writeAdminSpaceImportHandlerFile(t, zipWriter, name, payload)
}

func writeAdminSpaceImportHandlerFile(t *testing.T, zipWriter *zip.Writer, name string, payload []byte) {
	t.Helper()

	writer, err := zipWriter.Create(name)
	if err != nil {
		t.Fatalf("create zip entry %s failed: %v", name, err)
	}
	if _, err := writer.Write(payload); err != nil {
		t.Fatalf("write zip entry %s failed: %v", name, err)
	}
}
