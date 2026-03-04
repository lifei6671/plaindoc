package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
)

func TestRouter_CreateNode_WithTemplateInitializesDocumentAndRevision(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "owner-create-node-template@example.com")
	spaceID := "01h1createtemplatenodespace0000001"
	seedSpaceForWorkspaceCreateNode(t, database, ownerUserID, spaceID, "member")

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := database.ORM.Table("document_templates").Create(map[string]any{
		"template_id":        "meeting-template",
		"scene_key":          "meeting",
		"scene_name":         "会议纪要",
		"name":               "会议模板",
		"description":        "",
		"default_title":      "会议纪要",
		"content_md":         "# 议题\n\n- 待补充",
		"sort":               1,
		"is_builtin":         0,
		"is_enabled":         1,
		"created_by_user_id": ownerUserID,
		"updated_by_user_id": ownerUserID,
		"created_at":         now,
		"updated_at":         now,
	}).Error; err != nil {
		t.Fatalf("insert document template failed: %v", err)
	}

	body := []byte(`{"parentId":null,"type":"doc","title":"  ","templateId":"meeting-template"}`)
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
		Title     string `gorm:"column:title"`
		ContentMD string `gorm:"column:content_md"`
		Version   int    `gorm:"column:version"`
	}
	if err := database.ORM.Table("documents").
		Select("title", "content_md", "version").
		Where("document_id = ?", payload.DocID).
		Take(&persistedDoc).Error; err != nil {
		t.Fatalf("query created document failed: %v", err)
	}
	if persistedDoc.Title != "会议纪要" {
		t.Fatalf("expected created document title 会议纪要, got %q", persistedDoc.Title)
	}
	if persistedDoc.ContentMD != "# 议题\n\n- 待补充" {
		t.Fatalf("expected created document content from template, got %q", persistedDoc.ContentMD)
	}
	if persistedDoc.Version != 1 {
		t.Fatalf("expected created document version 1, got %d", persistedDoc.Version)
	}

	var persistedRevision struct {
		ContentMD string `gorm:"column:content_md"`
		Version   int    `gorm:"column:version"`
	}
	if err := database.ORM.Table("document_revisions").
		Select("content_md", "version").
		Where("document_id = ?", payload.DocID).
		Take(&persistedRevision).Error; err != nil {
		t.Fatalf("query created revision failed: %v", err)
	}
	if persistedRevision.ContentMD != "# 议题\n\n- 待补充" {
		t.Fatalf("expected revision content from template, got %q", persistedRevision.ContentMD)
	}
	if persistedRevision.Version != 1 {
		t.Fatalf("expected revision version 1, got %d", persistedRevision.Version)
	}
}

func TestRouter_CreateNode_WithTemplateInvalidID(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "owner-create-node-template-invalid@example.com")
	spaceID := "01h1createtemplatenodespaceinvalid01"
	seedSpaceForWorkspaceCreateNode(t, database, ownerUserID, spaceID, "member")

	body := []byte(`{"parentId":null,"type":"doc","title":"模板测试","templateId":"_invalid"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/spaces/"+spaceID+"/nodes", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", "application/json")
	rec := serve(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	expectedCode := response.ResolveErrorCode(response.CodeInvalidTemplateID)
	if code := decodeJSONResultCode(t, rec.Body.Bytes()); code != expectedCode {
		t.Fatalf("expected invalid template id code %d, got %d body=%s", expectedCode, code, rec.Body.String())
	}
}
