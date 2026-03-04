package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
)

func TestRouter_ListDocumentTemplates_EnabledOnlyAndFilters(t *testing.T) {
	database, serve := setupThemeTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := database.ORM.Table("document_templates").Create([]map[string]any{
		{
			"template_id":        "meeting-notes",
			"scene_key":          "meeting",
			"scene_name":         "会议纪要",
			"name":               "会议纪要模板",
			"description":        "用于会议记录",
			"default_title":      "会议纪要",
			"content_md":         "# 议题\n\n- 待补充",
			"sort":               1,
			"is_builtin":         1,
			"is_enabled":         1,
			"created_by_user_id": nil,
			"updated_by_user_id": nil,
			"created_at":         now,
			"updated_at":         now,
		},
		{
			"template_id":        "retro-template",
			"scene_key":          "meeting",
			"scene_name":         "会议纪要",
			"name":               "复盘模板",
			"description":        "用于事故复盘",
			"default_title":      "复盘文档",
			"content_md":         "# 复盘",
			"sort":               2,
			"is_builtin":         0,
			"is_enabled":         0,
			"created_by_user_id": nil,
			"updated_by_user_id": nil,
			"created_at":         now,
			"updated_at":         now,
		},
	}).Error; err != nil {
		t.Fatalf("insert document templates failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/document-templates?sceneKey=meeting&keyword=会议", nil)
	rec := serve(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	payload := decodeJSONResultData[struct {
		Items []struct {
			TemplateID string `json:"templateId"`
			Enabled    bool   `json:"enabled"`
		} `json:"items"`
		Pagination struct {
			Page     int   `json:"page"`
			PageSize int   `json:"pageSize"`
			Total    int64 `json:"total"`
		} `json:"pagination"`
	}](t, rec.Body.Bytes())

	if payload.Pagination.Total != 1 {
		t.Fatalf("expected pagination total 1, got %d", payload.Pagination.Total)
	}
	if len(payload.Items) != 1 {
		t.Fatalf("expected 1 enabled template item, got %d", len(payload.Items))
	}
	if payload.Items[0].TemplateID != "meeting-notes" {
		t.Fatalf("expected template meeting-notes, got %s", payload.Items[0].TemplateID)
	}
	if !payload.Items[0].Enabled {
		t.Fatalf("expected template enabled=true")
	}
}

func TestRouter_GetDocumentTemplate(t *testing.T) {
	database, serve := setupThemeTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := database.ORM.Table("document_templates").Create(map[string]any{
		"template_id":        "prd-template",
		"scene_key":          "requirement",
		"scene_name":         "需求评审",
		"name":               "PRD 模板",
		"description":        "产品需求文档模板",
		"default_title":      "产品需求文档",
		"content_md":         "# 背景\n\n# 目标",
		"sort":               10,
		"is_builtin":         0,
		"is_enabled":         1,
		"created_by_user_id": nil,
		"updated_by_user_id": nil,
		"created_at":         now,
		"updated_at":         now,
	}).Error; err != nil {
		t.Fatalf("insert document template failed: %v", err)
	}

	t.Run("success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/document-templates/prd-template", nil)
		rec := serve(req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
		}

		payload := decodeJSONResultData[struct {
			TemplateID string `json:"templateId"`
			ContentMD  string `json:"contentMd"`
		}](t, rec.Body.Bytes())
		if payload.TemplateID != "prd-template" {
			t.Fatalf("expected templateId prd-template, got %s", payload.TemplateID)
		}
		if !strings.Contains(payload.ContentMD, "# 背景") {
			t.Fatalf("expected content contains template markdown, got %q", payload.ContentMD)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/document-templates/not-exists", nil)
		rec := serve(req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected normalized status 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		if decodeJSONResultCode(t, rec.Body.Bytes()) != response.ResolveErrorCode(response.CodeTemplateNotFound) {
			t.Fatalf(
				"expected template not found code %d, got %d body=%s",
				response.ResolveErrorCode(response.CodeTemplateNotFound),
				decodeJSONResultCode(t, rec.Body.Bytes()),
				rec.Body.String(),
			)
		}
	})

	t.Run("invalid_template_id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/document-templates/_invalid", nil)
		rec := serve(req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected normalized status 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		if decodeJSONResultCode(t, rec.Body.Bytes()) != response.ResolveErrorCode(response.CodeInvalidTemplateID) {
			t.Fatalf(
				"expected invalid template id code %d, got %d body=%s",
				response.ResolveErrorCode(response.CodeInvalidTemplateID),
				decodeJSONResultCode(t, rec.Body.Bytes()),
				rec.Body.String(),
			)
		}
	})
}
