package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage"
)

func TestRouter_ReaderOnlyOfficeViewConfig_PublicOfficeDocument(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "reader-onlyoffice-owner@example.com")
	spaceID := "01h1readeronlyofficespace000001"
	seedOnlyOfficeEnabledConfig(t, database)
	seedSpaceForWorkspaceCreateNode(t, database, ownerUserID, spaceID, "member")

	documentID := createOfficeDocumentForOnlyOfficeTest(t, serve, spaceID, ownerToken, "docx", "公开阅读合同")
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := database.ORM.Table("spaces").Where("space_id = ?", spaceID).Updates(map[string]any{
		"visibility": "public",
		"updated_at": now,
	}).Error; err != nil {
		t.Fatalf("update space visibility failed: %v", err)
	}
	if err := database.ORM.Table("documents").Where("document_id = ?", documentID).Updates(map[string]any{
		"visibility": "public",
		"updated_at": now,
	}).Error; err != nil {
		t.Fatalf("update document visibility failed: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/reader/spaces/"+spaceID+"/docs/"+documentID+"/onlyoffice/view-config",
		nil,
	)
	rec := serve(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected reader onlyoffice view config status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	payload := decodeJSONResultData[struct {
		DocumentServerURL string         `json:"documentServerUrl"`
		Config            map[string]any `json:"config"`
	}](t, rec.Body.Bytes())
	if payload.DocumentServerURL != "https://onlyoffice.example.com" {
		t.Fatalf("expected onlyoffice document server url, got %+v", payload)
	}

	documentConfig := requireJSONMap(t, payload.Config["document"], "document")
	editorConfig := requireJSONMap(t, payload.Config["editorConfig"], "editorConfig")
	permissions := requireJSONMap(t, documentConfig["permissions"], "permissions")
	if stringValue(editorConfig["mode"]) != "view" {
		t.Fatalf("expected onlyoffice reader mode view, got %+v", editorConfig)
	}
	if permissions["edit"] != false {
		t.Fatalf("expected read-only permissions, got %+v", permissions)
	}
	if !strings.Contains(stringValue(documentConfig["url"]), "/api/docs/"+documentID+"/onlyoffice/source") {
		t.Fatalf("expected source url point to onlyoffice source endpoint, got %+v", documentConfig)
	}
}

func TestRouter_ReaderLandingRedirectsToOfficeDocument(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "reader-onlyoffice-landing-owner@example.com")
	spaceID := "01h1readeronlyofficelanding0001"
	seedOnlyOfficeEnabledConfig(t, database)
	seedSpaceForWorkspaceCreateNode(t, database, ownerUserID, spaceID, "member")

	documentID := createOfficeDocumentForOnlyOfficeTest(t, serve, spaceID, ownerToken, "docx", "空间入口合同")
	publishOnlyOfficeReaderDocument(t, database, spaceID, documentID)

	req := httptest.NewRequest(http.MethodGet, "/r/"+spaceID, nil)
	rec := serve(req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected reader landing redirect status 303, got %d body=%s", rec.Code, rec.Body.String())
	}
	if location := rec.Header().Get("Location"); location != "/r/"+spaceID+"/"+documentID {
		t.Fatalf("expected reader landing redirect to office document, got %q", location)
	}
}

func TestRouter_ShareOnlyOfficeViewConfig_PublicShare(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "share-onlyoffice-owner@example.com")
	spaceID := "01h1shareonlyofficespace000001"
	shareID := "01h1shareonlyofficeconfig000001"
	seedOnlyOfficeEnabledConfig(t, database)
	seedSpaceForWorkspaceCreateNode(t, database, ownerUserID, spaceID, "member")

	documentID := createOfficeDocumentForOnlyOfficeTest(t, serve, spaceID, ownerToken, "xlsx", "项目预算表")
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := database.ORM.Table("document_shares").Create(map[string]any{
		"share_id":       shareID,
		"document_id":    documentID,
		"space_id":       spaceID,
		"mode":           "public",
		"password_hint":  "",
		"access_version": 1,
		"created_at":     now,
		"updated_at":     now,
	}).Error; err != nil {
		t.Fatalf("insert document share failed: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/shares/"+spaceID+"/"+documentID+"/onlyoffice/view-config",
		nil,
	)
	rec := serve(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected share onlyoffice view config status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	payload := decodeJSONResultData[struct {
		DocumentServerURL string         `json:"documentServerUrl"`
		Config            map[string]any `json:"config"`
	}](t, rec.Body.Bytes())
	if payload.DocumentServerURL != "https://onlyoffice.example.com" {
		t.Fatalf("expected onlyoffice document server url, got %+v", payload)
	}

	documentConfig := requireJSONMap(t, payload.Config["document"], "document")
	editorConfig := requireJSONMap(t, payload.Config["editorConfig"], "editorConfig")
	if stringValue(editorConfig["mode"]) != "view" {
		t.Fatalf("expected share onlyoffice reader mode view, got %+v", editorConfig)
	}
	if !strings.Contains(stringValue(documentConfig["url"]), "/api/docs/"+documentID+"/onlyoffice/source") {
		t.Fatalf("expected share source url point to onlyoffice source endpoint, got %+v", documentConfig)
	}
}

func TestRouter_ReaderPage_PublicOfficeDocumentSerializesOfficeState(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "reader-onlyoffice-page-owner@example.com")
	spaceID := "01h1readeronlyofficepage000001"
	seedOnlyOfficeEnabledConfig(t, database)
	seedSpaceForWorkspaceCreateNode(t, database, ownerUserID, spaceID, "member")

	documentID := createOfficeDocumentForOnlyOfficeTest(t, serve, spaceID, ownerToken, "docx", "公开阅读合同页")
	publishOnlyOfficeReaderDocument(t, database, spaceID, documentID)

	req := httptest.NewRequest(http.MethodGet, "/r/"+spaceID+"/"+documentID, nil)
	rec := serve(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected office reader page status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, "SSR 渲染暂时不可用") {
		t.Fatalf("expected test router fallback html, body=%s", body)
	}
	if !strings.Contains(body, `name="robots" content="noindex, nofollow"`) {
		t.Fatalf("expected office reader fallback keeps robots noindex, body=%s", body)
	}

	state := decodeReaderInitialState(t, body)
	documentState := requireJSONMap(t, state["document"], "document")
	if stringValue(documentState["format"]) != "docx" {
		t.Fatalf("expected serialized office reader format docx, got %+v", documentState)
	}
	if !strings.Contains(stringValue(documentState["sourceFileName"]), ".docx") {
		t.Fatalf("expected serialized office source file name, got %+v", documentState)
	}
	treeRows, ok := state["tree"].([]any)
	if !ok || len(treeRows) != 1 {
		t.Fatalf("expected reader tree rows, got %+v", state["tree"])
	}
	treeDocument := requireJSONMap(t, treeRows[0], "tree[0]")
	if stringValue(treeDocument["documentFormat"]) != "docx" {
		t.Fatalf("expected tree node carries office format, got %+v", treeDocument)
	}
}

func TestRouter_SharePage_PublicOfficeDocumentSerializesOfficeState(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "share-onlyoffice-page-owner@example.com")
	spaceID := "01h1shareonlyofficepage000001"
	shareID := "01h1shareonlyofficepage000001"
	seedOnlyOfficeEnabledConfig(t, database)
	seedSpaceForWorkspaceCreateNode(t, database, ownerUserID, spaceID, "member")

	documentID := createOfficeDocumentForOnlyOfficeTest(t, serve, spaceID, ownerToken, "xlsx", "预算分享页")
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := database.ORM.Table("document_shares").Create(map[string]any{
		"share_id":       shareID,
		"document_id":    documentID,
		"space_id":       spaceID,
		"mode":           "public",
		"password_hint":  "",
		"access_version": 1,
		"created_at":     now,
		"updated_at":     now,
	}).Error; err != nil {
		t.Fatalf("insert document share failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/s/"+spaceID+"/"+documentID, nil)
	rec := serve(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected office share page status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, "SSR 渲染暂时不可用") {
		t.Fatalf("expected test router fallback html, body=%s", body)
	}
	if !strings.Contains(body, `name="robots" content="noindex, nofollow"`) {
		t.Fatalf("expected office share fallback keeps robots noindex, body=%s", body)
	}

	state := decodeReaderInitialState(t, body)
	documentState := requireJSONMap(t, state["document"], "document")
	if stringValue(documentState["format"]) != "xlsx" {
		t.Fatalf("expected serialized office share format xlsx, got %+v", documentState)
	}
	if !strings.Contains(stringValue(documentState["sourceFileName"]), ".xlsx") {
		t.Fatalf("expected serialized share source file name, got %+v", documentState)
	}
	shareState := requireJSONMap(t, state["share"], "share")
	if shareState["enabled"] != true {
		t.Fatalf("expected share state enabled, got %+v", shareState)
	}
	if stringValue(shareState["spaceId"]) != spaceID {
		t.Fatalf("expected serialized share state space id %s, got %+v", spaceID, shareState)
	}
}

func publishOnlyOfficeReaderDocument(t *testing.T, database *storage.Database, spaceID string, documentID string) {
	t.Helper()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := database.ORM.Table("spaces").Where("space_id = ?", spaceID).Updates(map[string]any{
		"visibility": "public",
		"updated_at": now,
	}).Error; err != nil {
		t.Fatalf("update space visibility failed: %v", err)
	}
	if err := database.ORM.Table("documents").Where("document_id = ?", documentID).Updates(map[string]any{
		"visibility": "public",
		"updated_at": now,
	}).Error; err != nil {
		t.Fatalf("update document visibility failed: %v", err)
	}
}

func decodeReaderInitialState(t *testing.T, body string) map[string]any {
	t.Helper()

	marker := `<script id="plaindoc-reader-initial-state" type="application/json">`
	start := strings.Index(body, marker)
	if start < 0 {
		t.Fatalf("expected reader initial state script, body=%s", body)
	}
	start += len(marker)
	end := strings.Index(body[start:], "</script>")
	if end < 0 {
		t.Fatalf("expected reader initial state script end tag, body=%s", body)
	}

	payload := body[start : start+end]
	result := make(map[string]any)
	if err := json.Unmarshal([]byte(payload), &result); err != nil {
		t.Fatalf("decode reader initial state failed: %v payload=%s", err, payload)
	}
	return result
}
