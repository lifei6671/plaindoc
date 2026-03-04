package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
)

func TestRouter_AdminDocumentTemplate_CRUD(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	spaceAdminUserID, _, spaceAdminToken := registerAccessUser(t, serve, "doc-template-space-admin@example.com")
	platformAdminUserID, _, platformAdminToken := registerAccessUser(t, serve, "doc-template-platform-admin@example.com")
	grantAdminRole(t, database, spaceAdminUserID, "space_admin")
	grantAdminRole(t, database, platformAdminUserID, "platform_admin")

	t.Run("space_admin_forbidden", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/document-templates", nil)
		req.Header.Set("Authorization", "Bearer "+spaceAdminToken)
		rec := serve(req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected status 403, got %d body=%s", rec.Code, rec.Body.String())
		}
		if gotCode := decodeJSONResultCode(t, rec.Body.Bytes()); gotCode != response.ResolveErrorCode(response.CodeForbidden) {
			t.Fatalf("expected forbidden code %d, got %d body=%s", response.ResolveErrorCode(response.CodeForbidden), gotCode, rec.Body.String())
		}
	})

	t.Run("platform_admin_crud", func(t *testing.T) {
		createBody, err := json.Marshal(map[string]any{
			"templateId":   "meeting-template",
			"sceneKey":     "meeting",
			"sceneName":    "会议纪要",
			"name":         "会议模板",
			"description":  "用于周会",
			"defaultTitle": "会议纪要",
			"contentMd":    "# 议题\n\n- 待补充",
			"sort":         2,
			"enabled":      true,
		})
		if err != nil {
			t.Fatalf("marshal create body failed: %v", err)
		}

		createReq := httptest.NewRequest(http.MethodPost, "/api/admin/document-templates", bytes.NewReader(createBody))
		createReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
		createReq.Header.Set("Content-Type", "application/json")
		createRec := serve(createReq)
		if createRec.Code != http.StatusOK {
			t.Fatalf("expected create status 200, got %d body=%s", createRec.Code, createRec.Body.String())
		}
		createPayload := decodeJSONResultData[struct {
			TemplateID string `json:"templateId"`
			Name       string `json:"name"`
			Enabled    bool   `json:"enabled"`
		}](t, createRec.Body.Bytes())
		if createPayload.TemplateID != "meeting-template" {
			t.Fatalf("expected templateId meeting-template, got %s", createPayload.TemplateID)
		}
		if createPayload.Name != "会议模板" {
			t.Fatalf("expected name 会议模板, got %s", createPayload.Name)
		}
		if !createPayload.Enabled {
			t.Fatalf("expected created template enabled")
		}

		listReq := httptest.NewRequest(http.MethodGet, "/api/admin/document-templates?keyword=会议", nil)
		listReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
		listRec := serve(listReq)
		if listRec.Code != http.StatusOK {
			t.Fatalf("expected list status 200, got %d body=%s", listRec.Code, listRec.Body.String())
		}
		listPayload := decodeJSONResultData[struct {
			Items []struct {
				TemplateID string `json:"templateId"`
			} `json:"items"`
		}](t, listRec.Body.Bytes())
		if len(listPayload.Items) != 1 || listPayload.Items[0].TemplateID != "meeting-template" {
			t.Fatalf("expected 1 template meeting-template, body=%s", listRec.Body.String())
		}

		getReq := httptest.NewRequest(http.MethodGet, "/api/admin/document-templates/meeting-template", nil)
		getReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
		getRec := serve(getReq)
		if getRec.Code != http.StatusOK {
			t.Fatalf("expected get status 200, got %d body=%s", getRec.Code, getRec.Body.String())
		}
		getPayload := decodeJSONResultData[struct {
			TemplateID string `json:"templateId"`
			ContentMD  string `json:"contentMd"`
		}](t, getRec.Body.Bytes())
		if getPayload.TemplateID != "meeting-template" {
			t.Fatalf("expected get templateId meeting-template, got %s", getPayload.TemplateID)
		}
		if getPayload.ContentMD != "# 议题\n\n- 待补充" {
			t.Fatalf("expected template content from create payload, got %q", getPayload.ContentMD)
		}

		updateToken := issueAdminOperationToken(
			t,
			serve,
			platformAdminToken,
			"document_template.update",
			"document_template",
			"meeting-template",
		)
		updateBody := []byte(`{"name":"会议模板V2","enabled":false}`)
		updateReq := httptest.NewRequest(http.MethodPut, "/api/admin/document-templates/meeting-template", bytes.NewReader(updateBody))
		updateReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
		updateReq.Header.Set("Content-Type", "application/json")
		updateReq.Header.Set("X-Admin-Operation-Token", updateToken)
		updateRec := serve(updateReq)
		if updateRec.Code != http.StatusOK {
			t.Fatalf("expected update status 200, got %d body=%s", updateRec.Code, updateRec.Body.String())
		}
		updatePayload := decodeJSONResultData[struct {
			Name    string `json:"name"`
			Enabled bool   `json:"enabled"`
		}](t, updateRec.Body.Bytes())
		if updatePayload.Name != "会议模板V2" {
			t.Fatalf("expected updated name 会议模板V2, got %s", updatePayload.Name)
		}
		if updatePayload.Enabled {
			t.Fatalf("expected updated enabled=false")
		}

		deleteToken := issueAdminOperationToken(
			t,
			serve,
			platformAdminToken,
			"document_template.delete",
			"document_template",
			"meeting-template",
		)
		deleteReq := httptest.NewRequest(http.MethodDelete, "/api/admin/document-templates/meeting-template", nil)
		deleteReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
		deleteReq.Header.Set("X-Admin-Operation-Token", deleteToken)
		deleteRec := serve(deleteReq)
		if deleteRec.Code != http.StatusOK {
			t.Fatalf("expected delete status 200, got %d body=%s", deleteRec.Code, deleteRec.Body.String())
		}

		listAfterDeleteReq := httptest.NewRequest(http.MethodGet, "/api/admin/document-templates", nil)
		listAfterDeleteReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
		listAfterDeleteRec := serve(listAfterDeleteReq)
		if listAfterDeleteRec.Code != http.StatusOK {
			t.Fatalf("expected list after delete status 200, got %d body=%s", listAfterDeleteRec.Code, listAfterDeleteRec.Body.String())
		}
		listAfterDeletePayload := decodeJSONResultData[struct {
			Items []struct {
				TemplateID string `json:"templateId"`
			} `json:"items"`
		}](t, listAfterDeleteRec.Body.Bytes())
		if len(listAfterDeletePayload.Items) != 0 {
			t.Fatalf("expected 0 templates after delete, body=%s", listAfterDeleteRec.Body.String())
		}
	})
}

func TestRouter_AdminDocumentTemplate_CreateInvalidTemplateID(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	platformAdminUserID, _, platformAdminToken := registerAccessUser(t, serve, "doc-template-invalid-platform-admin@example.com")
	grantAdminRole(t, database, platformAdminUserID, "platform_admin")

	reqBody := []byte(`{
		"templateId":"_invalid",
		"sceneKey":"meeting",
		"sceneName":"会议纪要",
		"name":"会议模板",
		"description":"",
		"defaultTitle":"",
		"contentMd":"# content"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/document-templates", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer "+platformAdminToken)
	req.Header.Set("Content-Type", "application/json")
	rec := serve(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	expected := response.ResolveErrorCode(response.CodeInvalidTemplateID)
	if code := decodeJSONResultCode(t, rec.Body.Bytes()); code != expected {
		t.Fatalf("expected invalid template id code %d, got %d body=%s", expected, code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "template id") {
		t.Fatalf("expected response body contains template id hint, body=%s", rec.Body.String())
	}
}
