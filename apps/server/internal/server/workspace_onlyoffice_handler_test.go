package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"os"
	"strings"
	"testing"
)

const onlyOfficeTestMIMEDOCX = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"

func TestRouter_WorkspaceOnlyOfficeEditConfigAndSource(t *testing.T) {
	t.Cleanup(func() {
		_ = os.RemoveAll("uploads")
	})

	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "owner-onlyoffice-config@example.com")
	spaceID := "01h1onlyofficeconfigspace000001"
	seedSpaceForWorkspaceCreateNode(t, database, ownerUserID, spaceID, "member")
	seedOnlyOfficeEnabledConfig(t, database)

	documentID := createOfficeDocumentForOnlyOfficeTest(t, serve, spaceID, ownerToken, "docx", "项目计划")

	editConfigReq := httptest.NewRequest(http.MethodGet, "/api/docs/"+documentID+"/onlyoffice/edit-config", nil)
	editConfigReq.Header.Set("Authorization", "Bearer "+ownerToken)
	editConfigRec := serve(editConfigReq)
	if editConfigRec.Code != http.StatusOK {
		t.Fatalf("expected edit config status 200, got %d body=%s", editConfigRec.Code, editConfigRec.Body.String())
	}

	payload := decodeJSONResultData[struct {
		DocumentServerURL string         `json:"documentServerUrl"`
		Config            map[string]any `json:"config"`
	}](t, editConfigRec.Body.Bytes())
	if payload.DocumentServerURL != "https://onlyoffice.example.com" {
		t.Fatalf("expected onlyoffice document server url, got %+v", payload)
	}

	documentConfig := requireJSONMap(t, payload.Config["document"], "document config")
	editorConfig := requireJSONMap(t, payload.Config["editorConfig"], "editorConfig")
	if documentConfig["fileType"] != "docx" {
		t.Fatalf("expected fileType docx, got %+v", documentConfig)
	}
	if payload.Config["documentType"] != "text" {
		t.Fatalf("expected documentType text, got %+v", payload.Config)
	}
	if strings.TrimSpace(stringValue(documentConfig["key"])) == "" {
		t.Fatalf("expected document key, got %+v", documentConfig)
	}
	if strings.TrimSpace(stringValue(payload.Config["token"])) == "" {
		t.Fatalf("expected signed onlyoffice token, got %+v", payload.Config)
	}

	sourceURL := stringValue(documentConfig["url"])
	if !strings.Contains(sourceURL, "/api/docs/"+documentID+"/onlyoffice/source") {
		t.Fatalf("expected source url point to onlyoffice source endpoint, got %q", sourceURL)
	}
	callbackURL := stringValue(editorConfig["callbackUrl"])
	if !strings.Contains(callbackURL, "/api/docs/"+documentID+"/onlyoffice/callback") {
		t.Fatalf("expected callback url point to onlyoffice callback endpoint, got %q", callbackURL)
	}

	sourceReq := httptest.NewRequest(http.MethodGet, requestURIFromAbsoluteURL(t, sourceURL), nil)
	sourceRec := serve(sourceReq)
	if sourceRec.Code != http.StatusOK {
		t.Fatalf("expected source status 200, got %d body=%s", sourceRec.Code, sourceRec.Body.String())
	}
	if contentType := sourceRec.Header().Get("Content-Type"); contentType != onlyOfficeTestMIMEDOCX {
		t.Fatalf("expected source mime %q, got %q", onlyOfficeTestMIMEDOCX, contentType)
	}
	if !strings.Contains(sourceRec.Header().Get("Content-Disposition"), ".docx") {
		t.Fatalf("expected source content disposition include docx filename, headers=%v", sourceRec.Header())
	}
	if sourceRec.Body.Len() == 0 {
		t.Fatal("expected source body not empty")
	}
}

func TestRouter_WorkspaceOnlyOfficeCallbackPersistsFileRevision(t *testing.T) {
	t.Cleanup(func() {
		_ = os.RemoveAll("uploads")
	})

	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "owner-onlyoffice-callback@example.com")
	spaceID := "01h1onlyofficecallbackspace00001"
	seedSpaceForWorkspaceCreateNode(t, database, ownerUserID, spaceID, "member")
	seedOnlyOfficeEnabledConfig(t, database)

	documentID := createOfficeDocumentForOnlyOfficeTest(t, serve, spaceID, ownerToken, "docx", "合同正文")
	configPayload := fetchOnlyOfficeEditConfigForTest(t, serve, ownerToken, documentID)
	documentConfig := requireJSONMap(t, configPayload.Config["document"], "document config")
	editorConfig := requireJSONMap(t, configPayload.Config["editorConfig"], "editorConfig")

	var beforeDoc struct {
		Version        int     `gorm:"column:version"`
		ContentVersion int     `gorm:"column:content_version"`
		SourceBlobID   *string `gorm:"column:source_blob_id"`
	}
	if err := database.ORM.Table("documents").
		Select("version", "content_version", "source_blob_id").
		Where("document_id = ?", documentID).
		Take(&beforeDoc).Error; err != nil {
		t.Fatalf("query pre-callback document failed: %v", err)
	}
	if beforeDoc.SourceBlobID == nil || strings.TrimSpace(*beforeDoc.SourceBlobID) == "" {
		t.Fatalf("expected pre-callback source blob id, got %+v", beforeDoc.SourceBlobID)
	}

	callbackBinary := []byte("PK\x03\x04updated-office-binary")
	downloadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", onlyOfficeTestMIMEDOCX)
		_, _ = w.Write(callbackBinary)
	}))
	defer downloadServer.Close()

	callbackBody := []byte(`{"status":2,"url":"` + downloadServer.URL + `/download.docx","key":"` + stringValue(documentConfig["key"]) + `"}`)
	callbackReq := httptest.NewRequest(
		http.MethodPost,
		requestURIFromAbsoluteURL(t, stringValue(editorConfig["callbackUrl"])),
		bytes.NewReader(callbackBody),
	)
	callbackReq.Header.Set("Content-Type", "application/json")
	callbackRec := serve(callbackReq)
	if callbackRec.Code != http.StatusOK {
		t.Fatalf("expected callback status 200, got %d body=%s", callbackRec.Code, callbackRec.Body.String())
	}

	var callbackResult struct {
		Error int `json:"error"`
	}
	if err := json.Unmarshal(callbackRec.Body.Bytes(), &callbackResult); err != nil {
		t.Fatalf("decode callback result failed: %v body=%s", err, callbackRec.Body.String())
	}
	if callbackResult.Error != 0 {
		t.Fatalf("expected callback error=0, got %+v", callbackResult)
	}

	var afterDoc struct {
		Version        int     `gorm:"column:version"`
		ContentVersion int     `gorm:"column:content_version"`
		SourceBlobID   *string `gorm:"column:source_blob_id"`
		SourceFileName *string `gorm:"column:source_file_name"`
		SourceMimeType *string `gorm:"column:source_mime_type"`
	}
	if err := database.ORM.Table("documents").
		Select("version", "content_version", "source_blob_id", "source_file_name", "source_mime_type").
		Where("document_id = ?", documentID).
		Take(&afterDoc).Error; err != nil {
		t.Fatalf("query post-callback document failed: %v", err)
	}
	if afterDoc.Version != 2 || afterDoc.ContentVersion != 2 {
		t.Fatalf("expected document version/contentVersion 2/2, got %d/%d", afterDoc.Version, afterDoc.ContentVersion)
	}
	if afterDoc.SourceBlobID == nil || strings.TrimSpace(*afterDoc.SourceBlobID) == "" {
		t.Fatalf("expected post-callback source blob id, got %+v", afterDoc.SourceBlobID)
	}
	if strings.TrimSpace(*afterDoc.SourceBlobID) == strings.TrimSpace(*beforeDoc.SourceBlobID) {
		t.Fatalf("expected callback generate new source blob id, before=%q after=%q", *beforeDoc.SourceBlobID, *afterDoc.SourceBlobID)
	}
	if afterDoc.SourceFileName == nil || *afterDoc.SourceFileName != "合同正文.docx" {
		t.Fatalf("expected source file name keep doc title, got %+v", afterDoc.SourceFileName)
	}
	if afterDoc.SourceMimeType == nil || *afterDoc.SourceMimeType != onlyOfficeTestMIMEDOCX {
		t.Fatalf("expected source mime type %q, got %+v", onlyOfficeTestMIMEDOCX, afterDoc.SourceMimeType)
	}

	var revisionCount int64
	if err := database.ORM.Table("document_file_revisions").
		Where("document_id = ?", documentID).
		Count(&revisionCount).Error; err != nil {
		t.Fatalf("count file revisions failed: %v", err)
	}
	if revisionCount != 2 {
		t.Fatalf("expected 2 file revisions after callback, got %d", revisionCount)
	}

	var latestRevision struct {
		BlobID      string `gorm:"column:blob_id"`
		Version     int    `gorm:"column:version"`
		BaseVersion int    `gorm:"column:base_version"`
		Source      string `gorm:"column:source"`
	}
	if err := database.ORM.Table("document_file_revisions").
		Select("blob_id", "version", "base_version", "source").
		Where("document_id = ?", documentID).
		Order("version DESC").
		Take(&latestRevision).Error; err != nil {
		t.Fatalf("query latest file revision failed: %v", err)
	}
	if latestRevision.Version != 2 || latestRevision.BaseVersion != 1 {
		t.Fatalf("expected latest file revision version/baseVersion 2/1, got %d/%d", latestRevision.Version, latestRevision.BaseVersion)
	}
	if latestRevision.Source != "remote" {
		t.Fatalf("expected latest file revision source remote, got %q", latestRevision.Source)
	}
	if latestRevision.BlobID != strings.TrimSpace(*afterDoc.SourceBlobID) {
		t.Fatalf("expected latest file revision blob id %q, got %q", *afterDoc.SourceBlobID, latestRevision.BlobID)
	}

	var savedBlob struct {
		MimeType  string `gorm:"column:mime_type"`
		SizeBytes int64  `gorm:"column:size_bytes"`
	}
	if err := database.ORM.Table("file_blobs").
		Select("mime_type", "size_bytes").
		Where("blob_id = ?", strings.TrimSpace(*afterDoc.SourceBlobID)).
		Take(&savedBlob).Error; err != nil {
		t.Fatalf("query saved blob failed: %v", err)
	}
	if savedBlob.MimeType != onlyOfficeTestMIMEDOCX {
		t.Fatalf("expected saved blob mime %q, got %q", onlyOfficeTestMIMEDOCX, savedBlob.MimeType)
	}
	if savedBlob.SizeBytes != int64(len(callbackBinary)) {
		t.Fatalf("expected saved blob size %d, got %d", len(callbackBinary), savedBlob.SizeBytes)
	}

	var auditLog struct {
		ActorUserID *string `gorm:"column:actor_user_id"`
		Module      string  `gorm:"column:module"`
		Action      string  `gorm:"column:action"`
		TargetType  string  `gorm:"column:target_type"`
		TargetID    string  `gorm:"column:target_id"`
		Summary     string  `gorm:"column:summary"`
		DetailJSON  string  `gorm:"column:detail_json"`
	}
	if err := database.ORM.Table("audit_logs").
		Select("actor_user_id", "module", "action", "target_type", "target_id", "summary", "detail_json").
		Where("module = ? AND action = ? AND target_type = ? AND target_id = ?", "document", "update", "document", documentID).
		Order("id DESC").
		Take(&auditLog).Error; err != nil {
		t.Fatalf("query onlyoffice callback audit log failed: %v", err)
	}
	if auditLog.ActorUserID == nil || strings.TrimSpace(*auditLog.ActorUserID) != ownerUserID {
		t.Fatalf("expected audit actor %q, got %+v", ownerUserID, auditLog.ActorUserID)
	}
	if auditLog.Summary != "onlyoffice callback updated office document" {
		t.Fatalf("unexpected onlyoffice audit summary: %+v", auditLog)
	}
	if !strings.Contains(auditLog.DetailJSON, `"source":"onlyoffice_callback"`) ||
		!strings.Contains(auditLog.DetailJSON, `"contentVersion":2`) {
		t.Fatalf("expected onlyoffice audit detail payload, got %s", auditLog.DetailJSON)
	}
}

func TestRouter_WorkspaceOnlyOfficeCallbackRejectsInvalidPayload(t *testing.T) {
	t.Cleanup(func() {
		_ = os.RemoveAll("uploads")
	})

	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "owner-onlyoffice-callback-invalid@example.com")
	spaceID := "01h1onlyofficecallbackinvalid001"
	seedSpaceForWorkspaceCreateNode(t, database, ownerUserID, spaceID, "member")
	seedOnlyOfficeEnabledConfig(t, database)

	documentID := createOfficeDocumentForOnlyOfficeTest(t, serve, spaceID, ownerToken, "docx", "回调失败保护")
	configPayload := fetchOnlyOfficeEditConfigForTest(t, serve, ownerToken, documentID)
	editorConfig := requireJSONMap(t, configPayload.Config["editorConfig"], "editorConfig")

	callbackReq := httptest.NewRequest(
		http.MethodPost,
		requestURIFromAbsoluteURL(t, stringValue(editorConfig["callbackUrl"])),
		bytes.NewReader([]byte(`{"status":2}`)),
	)
	callbackReq.Header.Set("Content-Type", "application/json")
	callbackRec := serve(callbackReq)
	if callbackRec.Code != http.StatusOK {
		t.Fatalf("expected callback status 200, got %d body=%s", callbackRec.Code, callbackRec.Body.String())
	}

	var callbackResult struct {
		Error int `json:"error"`
	}
	if err := json.Unmarshal(callbackRec.Body.Bytes(), &callbackResult); err != nil {
		t.Fatalf("decode callback result failed: %v body=%s", err, callbackRec.Body.String())
	}
	if callbackResult.Error != 1 {
		t.Fatalf("expected callback error=1 for invalid payload, got %+v", callbackResult)
	}

	var persistedDoc struct {
		Version        int `gorm:"column:version"`
		ContentVersion int `gorm:"column:content_version"`
	}
	if err := database.ORM.Table("documents").
		Select("version", "content_version").
		Where("document_id = ?", documentID).
		Take(&persistedDoc).Error; err != nil {
		t.Fatalf("query document after invalid callback failed: %v", err)
	}
	if persistedDoc.Version != 1 || persistedDoc.ContentVersion != 1 {
		t.Fatalf("expected invalid callback keep version/contentVersion 1/1, got %d/%d", persistedDoc.Version, persistedDoc.ContentVersion)
	}
}

func createOfficeDocumentForOnlyOfficeTest(
	t *testing.T,
	serve func(*http.Request) *httptest.ResponseRecorder,
	spaceID string,
	ownerToken string,
	format string,
	title string,
) string {
	t.Helper()

	body := []byte(`{"parentId":null,"type":"doc","title":"` + title + `","format":"` + format + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/spaces/"+spaceID+"/nodes", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", "application/json")
	rec := serve(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected create office doc status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	payload := decodeJSONResultData[struct {
		DocID string `json:"docId"`
	}](t, rec.Body.Bytes())
	if payload.DocID == "" {
		t.Fatalf("expected created office doc id, body=%s", rec.Body.String())
	}
	return payload.DocID
}

func fetchOnlyOfficeEditConfigForTest(
	t *testing.T,
	serve func(*http.Request) *httptest.ResponseRecorder,
	ownerToken string,
	documentID string,
) struct {
	DocumentServerURL string         `json:"documentServerUrl"`
	Config            map[string]any `json:"config"`
} {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/api/docs/"+documentID+"/onlyoffice/edit-config", nil)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	rec := serve(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected edit config status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	return decodeJSONResultData[struct {
		DocumentServerURL string         `json:"documentServerUrl"`
		Config            map[string]any `json:"config"`
	}](t, rec.Body.Bytes())
}

func requireJSONMap(t *testing.T, value any, field string) map[string]any {
	t.Helper()

	result, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("expected %s map, got %#v", field, value)
	}
	return result
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}

func requestURIFromAbsoluteURL(t *testing.T, rawURL string) string {
	t.Helper()

	parsed, err := neturl.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse absolute url %q failed: %v", rawURL, err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		t.Fatalf("expected absolute url, got %q", rawURL)
	}
	requestURI := parsed.RequestURI()
	if requestURI == "" {
		t.Fatalf("expected request uri from %q", rawURL)
	}
	return requestURI
}
