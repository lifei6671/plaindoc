package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
)

func TestRouter_ListDocumentRevisionsReturnsSummariesWithoutContent(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "revision-summary-owner@example.com")
	spaceID := "01krevisionsummaryspace0000001"
	nodeID := "01krevisionsummarynode00000001"
	documentID := "01krevisionsummarydoc00000001"
	revisionID := "01krevisionsummaryrev00000001"
	now := time.Now().UTC()

	seedSpaceAndDocumentForAccess(t, database, ownerUserID, spaceID, nodeID, documentID, "member", "member")
	if err := database.ORM.Table("documents").
		Where("document_id = ?", documentID).
		Updates(map[string]any{
			"format":             "markdown",
			"updated_by_user_id": ownerUserID,
		}).Error; err != nil {
		t.Fatalf("update document format failed: %v", err)
	}
	if err := database.ORM.Table("document_revisions").Create(map[string]any{
		"document_revision_id": revisionID,
		"document_id":          documentID,
		"version":              2,
		"content_md":           "# 不应出现在列表里的历史正文",
		"base_version":         1,
		"editor_user_id":       ownerUserID,
		"source":               "remote",
		"created_at":           now,
	}).Error; err != nil {
		t.Fatalf("seed document revision failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/docs/"+documentID+"/revisions", nil)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	rec := serve(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected list revisions status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var envelope jsonResultEnvelopeForTest
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode revision summary envelope failed: %v body=%s", err, rec.Body.String())
	}
	var revisions []map[string]json.RawMessage
	if err := json.Unmarshal(envelope.Data, &revisions); err != nil {
		t.Fatalf("decode revision summary payload failed: %v body=%s", err, rec.Body.String())
	}
	if len(revisions) != 1 {
		t.Fatalf("expected one revision summary, got %d body=%s", len(revisions), rec.Body.String())
	}
	if _, ok := revisions[0]["contentMd"]; ok {
		t.Fatalf("revision summary must not include contentMd, body=%s", rec.Body.String())
	}
	assertRawJSONFieldEquals(t, revisions[0], "format", "markdown")
	assertRawJSONFieldEquals(t, revisions[0], "id", revisionID)
	var editorUser struct {
		UserID      string `json:"userId"`
		DisplayName string `json:"displayName"`
	}
	if err := json.Unmarshal(revisions[0]["editorUser"], &editorUser); err != nil {
		t.Fatalf("decode editorUser failed: %v body=%s", err, rec.Body.String())
	}
	if editorUser.UserID != ownerUserID || editorUser.DisplayName == "" {
		t.Fatalf("expected editor user summary, got %+v body=%s", editorUser, rec.Body.String())
	}
}

func TestRouter_ListDocumentRevisionsPaginatesAndRejectsNoReadPermission(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "revision-list-owner@example.com")
	_, _, outsiderToken := registerAccessUser(t, serve, "revision-list-outsider@example.com")
	spaceID := "01krevisionlistspace00000001"
	nodeID := "01krevisionlistnode000000001"
	documentID := "01krevisionlistdoc0000000001"
	now := time.Now().UTC()

	seedSpaceAndDocumentForAccess(t, database, ownerUserID, spaceID, nodeID, documentID, "member", "member")
	for version := 1; version <= 3; version++ {
		if err := database.ORM.Table("document_revisions").Create(map[string]any{
			"document_revision_id": "01krevisionlistrev000000000" + string(rune('0'+version)),
			"document_id":          documentID,
			"version":              version,
			"content_md":           "# 历史正文",
			"base_version":         version - 1,
			"editor_user_id":       ownerUserID,
			"source":               "remote",
			"created_at":           now.Add(time.Duration(version) * time.Minute),
		}).Error; err != nil {
			t.Fatalf("seed paginated revision failed: %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/docs/"+documentID+"/revisions?page=2&pageSize=1", nil)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	rec := serve(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected list revisions status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	revisions := decodeJSONResultData[[]map[string]json.RawMessage](t, rec.Body.Bytes())
	if len(revisions) != 1 {
		t.Fatalf("expected one paginated revision, got %d body=%s", len(revisions), rec.Body.String())
	}
	assertRawJSONFieldEquals(t, revisions[0], "id", "01krevisionlistrev0000000002")
	if _, ok := revisions[0]["contentMd"]; ok {
		t.Fatalf("paginated revision summary must not include contentMd, body=%s", rec.Body.String())
	}

	deniedReq := httptest.NewRequest(http.MethodGet, "/api/docs/"+documentID+"/revisions?page=1&pageSize=1", nil)
	deniedReq.Header.Set("Authorization", "Bearer "+outsiderToken)
	deniedRec := serve(deniedReq)
	if deniedRec.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden status for revision list without read permission, got %d body=%s", deniedRec.Code, deniedRec.Body.String())
	}
}

func TestRouter_GetDocumentRevisionDetailReturnsMarkdownAndOffice(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "revision-detail-owner@example.com")
	spaceID := "01krevisiondetailspace000001"
	markdownNodeID := "01krevisiondetailnode0000001"
	markdownDocumentID := "01krevisiondetaildoc00000001"
	markdownRevisionID := "01krevisiondetailrev00000001"
	officeNodeID := "01krevisiondetailofficenode001"
	officeDocumentID := "01krevisiondetailofficedoc0001"
	officeRevisionID := "01krevisiondetailofficerev0001"
	officeBlobID := "01krevisiondetailofficeblob001"
	officeMimeType := "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	now := time.Now().UTC()

	seedSpaceAndDocumentForAccess(t, database, ownerUserID, spaceID, markdownNodeID, markdownDocumentID, "member", "member")
	if err := database.ORM.Table("document_revisions").Create(map[string]any{
		"document_revision_id": markdownRevisionID,
		"document_id":          markdownDocumentID,
		"version":              2,
		"content_md":           "# Markdown 历史正文",
		"base_version":         1,
		"editor_user_id":       ownerUserID,
		"source":               "remote",
		"created_at":           now,
	}).Error; err != nil {
		t.Fatalf("seed markdown revision detail failed: %v", err)
	}
	if err := database.ORM.Table("nodes").Create(map[string]any{
		"node_id":    officeNodeID,
		"space_id":   spaceID,
		"type":       "doc",
		"title":      "Office Detail",
		"sort":       2,
		"created_at": now,
		"updated_at": now,
	}).Error; err != nil {
		t.Fatalf("seed office node failed: %v", err)
	}
	if err := database.ORM.Table("file_blobs").Create(map[string]any{
		"blob_id":           officeBlobID,
		"storage_provider":  "local",
		"object_key":        "uploads/revisions/office-v2.docx",
		"object_url":        "/uploads/revisions/office-v2.docx",
		"mime_type":         officeMimeType,
		"size_bytes":        2048,
		"content_hash_algo": "sha256",
		"content_hash":      "office-revision-hash",
		"created_at":        now,
		"updated_at":        now,
	}).Error; err != nil {
		t.Fatalf("seed office blob failed: %v", err)
	}
	if err := database.ORM.Table("documents").Create(map[string]any{
		"document_id":        officeDocumentID,
		"node_id":            officeNodeID,
		"theme_id":           "default",
		"visibility":         "member",
		"status":             "active",
		"title":              "Office Detail",
		"format":             "docx",
		"version":            2,
		"content_version":    2,
		"source_blob_id":     officeBlobID,
		"source_file_name":   "office-v2.docx",
		"source_mime_type":   officeMimeType,
		"created_by_user_id": ownerUserID,
		"updated_by_user_id": ownerUserID,
		"created_at":         now,
		"updated_at":         now,
	}).Error; err != nil {
		t.Fatalf("seed office document failed: %v", err)
	}
	if err := database.ORM.Table("document_file_revisions").Create(map[string]any{
		"document_file_revision_id": officeRevisionID,
		"document_id":               officeDocumentID,
		"blob_id":                   officeBlobID,
		"file_name":                 "office-v2.docx",
		"mime_type":                 officeMimeType,
		"version":                   2,
		"base_version":              1,
		"editor_user_id":            ownerUserID,
		"source":                    "remote",
		"created_at":                now,
	}).Error; err != nil {
		t.Fatalf("seed office revision detail failed: %v", err)
	}

	markdownReq := httptest.NewRequest(http.MethodGet, "/api/docs/"+markdownDocumentID+"/revisions/"+markdownRevisionID, nil)
	markdownReq.Header.Set("Authorization", "Bearer "+ownerToken)
	markdownRec := serve(markdownReq)
	if markdownRec.Code != http.StatusOK {
		t.Fatalf("expected markdown detail status 200, got %d body=%s", markdownRec.Code, markdownRec.Body.String())
	}
	markdownDetail := decodeJSONResultData[map[string]json.RawMessage](t, markdownRec.Body.Bytes())
	assertRawJSONFieldEquals(t, markdownDetail, "contentMd", "# Markdown 历史正文")
	if _, ok := markdownDetail["file"]; ok {
		t.Fatalf("markdown revision detail must not include file metadata, body=%s", markdownRec.Body.String())
	}

	officeReq := httptest.NewRequest(http.MethodGet, "/api/docs/"+officeDocumentID+"/revisions/"+officeRevisionID, nil)
	officeReq.Header.Set("Authorization", "Bearer "+ownerToken)
	officeRec := serve(officeReq)
	if officeRec.Code != http.StatusOK {
		t.Fatalf("expected office detail status 200, got %d body=%s", officeRec.Code, officeRec.Body.String())
	}
	officeDetail := decodeJSONResultData[map[string]json.RawMessage](t, officeRec.Body.Bytes())
	assertRawJSONFieldEquals(t, officeDetail, "format", "docx")
	var file struct {
		BlobID   string `json:"blobId"`
		FileName string `json:"fileName"`
		MimeType string `json:"mimeType"`
	}
	if err := json.Unmarshal(officeDetail["file"], &file); err != nil {
		t.Fatalf("decode office detail file metadata failed: %v body=%s", err, officeRec.Body.String())
	}
	if file.BlobID != officeBlobID || file.FileName != "office-v2.docx" || file.MimeType != officeMimeType {
		t.Fatalf("unexpected office detail file metadata: %+v", file)
	}
	if _, ok := officeDetail["contentMd"]; ok {
		t.Fatalf("office revision detail must not include contentMd, body=%s", officeRec.Body.String())
	}
}

func TestRouter_GetDocumentRevisionDetailRejectsNoPermissionAndCrossDocumentRevision(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "revision-detail-access-owner@example.com")
	_, _, outsiderToken := registerAccessUser(t, serve, "revision-detail-access-outsider@example.com")
	spaceID := "01krevisionaccessspace000001"
	nodeID := "01krevisionaccessnode0000001"
	documentID := "01krevisionaccessdoc00000001"
	otherNodeID := "01krevisionaccessnode0000002"
	otherDocumentID := "01krevisionaccessdoc00000002"
	otherRevisionID := "01krevisionaccessrev00000002"
	now := time.Now().UTC()

	seedSpaceAndDocumentForAccess(t, database, ownerUserID, spaceID, nodeID, documentID, "member", "member")
	if err := database.ORM.Table("nodes").Create(map[string]any{
		"node_id":    otherNodeID,
		"space_id":   spaceID,
		"type":       "doc",
		"title":      "Other Doc",
		"sort":       2,
		"created_at": now,
		"updated_at": now,
	}).Error; err != nil {
		t.Fatalf("seed other node failed: %v", err)
	}
	if err := database.ORM.Table("documents").Create(map[string]any{
		"document_id":        otherDocumentID,
		"node_id":            otherNodeID,
		"theme_id":           "default",
		"visibility":         "member",
		"status":             "active",
		"title":              "Other Doc",
		"format":             "markdown",
		"content_md":         "# other",
		"version":            1,
		"content_version":    1,
		"created_by_user_id": ownerUserID,
		"updated_by_user_id": ownerUserID,
		"created_at":         now,
		"updated_at":         now,
	}).Error; err != nil {
		t.Fatalf("seed other document failed: %v", err)
	}
	if err := database.ORM.Table("document_revisions").Create(map[string]any{
		"document_revision_id": otherRevisionID,
		"document_id":          otherDocumentID,
		"version":              1,
		"content_md":           "# other revision",
		"base_version":         0,
		"editor_user_id":       ownerUserID,
		"source":               "local",
		"created_at":           now,
	}).Error; err != nil {
		t.Fatalf("seed other revision failed: %v", err)
	}

	deniedReq := httptest.NewRequest(http.MethodGet, "/api/docs/"+documentID+"/revisions/"+otherRevisionID, nil)
	deniedReq.Header.Set("Authorization", "Bearer "+outsiderToken)
	deniedRec := serve(deniedReq)
	if deniedRec.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden status for revision detail without read permission, got %d body=%s", deniedRec.Code, deniedRec.Body.String())
	}

	crossDocReq := httptest.NewRequest(http.MethodGet, "/api/docs/"+documentID+"/revisions/"+otherRevisionID, nil)
	crossDocReq.Header.Set("Authorization", "Bearer "+ownerToken)
	crossDocRec := serve(crossDocReq)
	if crossDocRec.Code != http.StatusOK {
		t.Fatalf("expected normalized not found status 200, got %d body=%s", crossDocRec.Code, crossDocRec.Body.String())
	}
	expectedCode := response.ResolveErrorCode(response.CodeDocumentNotFound)
	if code := decodeJSONResultCode(t, crossDocRec.Body.Bytes()); code != expectedCode {
		t.Fatalf("expected document not found code %d for cross-document revision, got %d body=%s", expectedCode, code, crossDocRec.Body.String())
	}
}

func TestRouter_RestoreDocumentRevisionContractValidatesBaseVersionAndNotFound(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "revision-restore-contract-owner@example.com")
	spaceID := "01krevisionrestorespace00001"
	nodeID := "01krevisionrestorenode000001"
	documentID := "01krevisionrestoredoc0000001"

	seedSpaceAndDocumentForAccess(t, database, ownerUserID, spaceID, nodeID, documentID, "member", "member")

	missingBaseVersionReq := httptest.NewRequest(
		http.MethodPost,
		"/api/docs/"+documentID+"/revisions/missing-revision/restore",
		bytes.NewReader([]byte(`{}`)),
	)
	missingBaseVersionReq.Header.Set("Authorization", "Bearer "+ownerToken)
	missingBaseVersionReq.Header.Set("Content-Type", "application/json")
	missingBaseVersionRec := serve(missingBaseVersionReq)
	if missingBaseVersionRec.Code != http.StatusOK {
		t.Fatalf("expected normalized invalid baseVersion status 200, got %d body=%s", missingBaseVersionRec.Code, missingBaseVersionRec.Body.String())
	}
	expectedInvalidCode := response.ResolveErrorCode(response.CodeInvalidRequest)
	if code := decodeJSONResultCode(t, missingBaseVersionRec.Body.Bytes()); code != expectedInvalidCode {
		t.Fatalf("expected invalid request code %d for missing baseVersion, got %d body=%s", expectedInvalidCode, code, missingBaseVersionRec.Body.String())
	}

	notFoundReq := httptest.NewRequest(
		http.MethodPost,
		"/api/docs/"+documentID+"/revisions/missing-revision/restore",
		bytes.NewReader([]byte(`{"baseVersion":1}`)),
	)
	notFoundReq.Header.Set("Authorization", "Bearer "+ownerToken)
	notFoundReq.Header.Set("Content-Type", "application/json")
	notFoundRec := serve(notFoundReq)
	if notFoundRec.Code != http.StatusOK {
		t.Fatalf("expected normalized revision not found status 200, got %d body=%s", notFoundRec.Code, notFoundRec.Body.String())
	}
	expectedNotFoundCode := response.ResolveErrorCode(response.CodeDocumentNotFound)
	if code := decodeJSONResultCode(t, notFoundRec.Body.Bytes()); code != expectedNotFoundCode {
		t.Fatalf("expected document not found code %d for missing revision, got %d body=%s", expectedNotFoundCode, code, notFoundRec.Body.String())
	}
}

func TestRouter_RestoreMarkdownDocumentRevisionCreatesNewVersion(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "revision-restore-markdown-owner@example.com")
	spaceID := "01krestoremarkdownspace0001"
	nodeID := "01krestoremarkdownnode00001"
	documentID := "01krestoremarkdowndoc000001"
	revisionID := "01krestoremarkdownrev000001"
	now := time.Now().UTC()

	seedSpaceAndDocumentForAccess(t, database, ownerUserID, spaceID, nodeID, documentID, "member", "member")
	if err := database.ORM.Table("documents").
		Where("document_id = ?", documentID).
		Updates(map[string]any{
			"content_md":      "# 当前正文",
			"version":         2,
			"content_version": 2,
			"format":          "markdown",
		}).Error; err != nil {
		t.Fatalf("seed current document version failed: %v", err)
	}
	if err := database.ORM.Table("document_revisions").Create(map[string]any{
		"document_revision_id": revisionID,
		"document_id":          documentID,
		"version":              1,
		"content_md":           "# 需要恢复的历史正文",
		"base_version":         0,
		"editor_user_id":       ownerUserID,
		"source":               "remote",
		"created_at":           now,
	}).Error; err != nil {
		t.Fatalf("seed restore target revision failed: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/docs/"+documentID+"/revisions/"+revisionID+"/restore",
		bytes.NewReader([]byte(`{"baseVersion":2}`)),
	)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", "application/json")
	rec := serve(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected restore markdown status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	payload := decodeJSONResultData[struct {
		Document struct {
			ID        string `json:"id"`
			ContentMD string `json:"contentMd"`
			Version   int    `json:"version"`
		} `json:"document"`
		RestoredFromRevision struct {
			ID      string `json:"id"`
			Version int    `json:"version"`
			Format  string `json:"format"`
		} `json:"restoredFromRevision"`
	}](t, rec.Body.Bytes())
	if payload.Document.ID != documentID || payload.Document.ContentMD != "# 需要恢复的历史正文" || payload.Document.Version != 3 {
		t.Fatalf("unexpected restored document payload: %+v body=%s", payload.Document, rec.Body.String())
	}
	if payload.RestoredFromRevision.ID != revisionID || payload.RestoredFromRevision.Version != 1 || payload.RestoredFromRevision.Format != "markdown" {
		t.Fatalf("unexpected restoredFromRevision payload: %+v body=%s", payload.RestoredFromRevision, rec.Body.String())
	}

	var latestRevision struct {
		ContentMD    string `gorm:"column:content_md"`
		Version      int    `gorm:"column:version"`
		BaseVersion  int    `gorm:"column:base_version"`
		EditorUserID string `gorm:"column:editor_user_id"`
	}
	if err := database.ORM.Table("document_revisions").
		Select("content_md", "version", "base_version", "editor_user_id").
		Where("document_id = ?", documentID).
		Order("version DESC").
		Take(&latestRevision).Error; err != nil {
		t.Fatalf("query latest restored revision failed: %v", err)
	}
	if latestRevision.ContentMD != "# 需要恢复的历史正文" || latestRevision.Version != 3 || latestRevision.BaseVersion != 2 || latestRevision.EditorUserID != ownerUserID {
		t.Fatalf("unexpected latest restored revision: %+v", latestRevision)
	}

	var originalRevision struct {
		ContentMD   string `gorm:"column:content_md"`
		Version     int    `gorm:"column:version"`
		BaseVersion int    `gorm:"column:base_version"`
	}
	if err := database.ORM.Table("document_revisions").
		Select("content_md", "version", "base_version").
		Where("document_revision_id = ?", revisionID).
		Take(&originalRevision).Error; err != nil {
		t.Fatalf("query original restore target revision failed: %v", err)
	}
	if originalRevision.ContentMD != "# 需要恢复的历史正文" || originalRevision.Version != 1 || originalRevision.BaseVersion != 0 {
		t.Fatalf("restore must keep original revision unchanged, got %+v", originalRevision)
	}
}

func TestRouter_RestoreMarkdownDocumentRevisionRejectsStaleBaseVersion(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "revision-restore-conflict-owner@example.com")
	spaceID := "01krestoreconflictspace0001"
	nodeID := "01krestoreconflictnode00001"
	documentID := "01krestoreconflictdoc000001"
	revisionID := "01krestoreconflictrev000001"
	now := time.Now().UTC()

	seedSpaceAndDocumentForAccess(t, database, ownerUserID, spaceID, nodeID, documentID, "member", "member")
	if err := database.ORM.Table("documents").
		Where("document_id = ?", documentID).
		Updates(map[string]any{
			"content_md":      "# 当前正文",
			"version":         2,
			"content_version": 2,
			"format":          "markdown",
		}).Error; err != nil {
		t.Fatalf("seed current document version failed: %v", err)
	}
	if err := database.ORM.Table("document_revisions").Create(map[string]any{
		"document_revision_id": revisionID,
		"document_id":          documentID,
		"version":              1,
		"content_md":           "# 不应覆盖的新正文",
		"base_version":         0,
		"editor_user_id":       ownerUserID,
		"source":               "remote",
		"created_at":           now,
	}).Error; err != nil {
		t.Fatalf("seed restore target revision failed: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/docs/"+documentID+"/revisions/"+revisionID+"/restore",
		bytes.NewReader([]byte(`{"baseVersion":1}`)),
	)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", "application/json")
	rec := serve(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected normalized version conflict status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	expectedCode := response.ResolveErrorCode(response.CodeDocumentVersionConflict)
	if code := decodeJSONResultCode(t, rec.Body.Bytes()); code != expectedCode {
		t.Fatalf("expected document version conflict code %d, got %d body=%s", expectedCode, code, rec.Body.String())
	}

	payload := decodeJSONResultData[struct {
		LatestDocument struct {
			ID        string `json:"id"`
			ContentMD string `json:"contentMd"`
			Version   int    `json:"version"`
		} `json:"latestDocument"`
	}](t, rec.Body.Bytes())
	if payload.LatestDocument.ID != documentID || payload.LatestDocument.ContentMD != "# 当前正文" || payload.LatestDocument.Version != 2 {
		t.Fatalf("unexpected latest document payload on conflict: %+v body=%s", payload.LatestDocument, rec.Body.String())
	}
}

func TestRouter_RestoreOfficeDocumentRevisionCreatesNewFileRevision(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "revision-restore-office-owner@example.com")
	spaceID := "01krestoreofficespace000001"
	nodeID := "01krestoreofficenode0000001"
	documentID := "01krestoreofficedoc0000001"
	targetRevisionID := "01krestoreofficerev0000001"
	currentRevisionID := "01krestoreofficerev0000002"
	targetBlobID := "01krestoreofficeblob000001"
	currentBlobID := "01krestoreofficeblob000002"
	officeMimeType := "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	now := time.Now().UTC()

	seedSpaceAndDocumentForAccess(t, database, ownerUserID, spaceID, nodeID, documentID, "member", "member")
	if err := database.ORM.Table("file_blobs").Create([]map[string]any{
		{
			"blob_id":           targetBlobID,
			"storage_provider":  "local",
			"object_key":        "uploads/revisions/target-v1.docx",
			"object_url":        "/uploads/revisions/target-v1.docx",
			"mime_type":         officeMimeType,
			"size_bytes":        1024,
			"content_hash_algo": "sha256",
			"content_hash":      "restore-office-target-hash",
			"created_at":        now,
			"updated_at":        now,
		},
		{
			"blob_id":           currentBlobID,
			"storage_provider":  "local",
			"object_key":        "uploads/revisions/current-v2.docx",
			"object_url":        "/uploads/revisions/current-v2.docx",
			"mime_type":         officeMimeType,
			"size_bytes":        2048,
			"content_hash_algo": "sha256",
			"content_hash":      "restore-office-current-hash",
			"created_at":        now,
			"updated_at":        now,
		},
	}).Error; err != nil {
		t.Fatalf("seed office blobs failed: %v", err)
	}
	if err := database.ORM.Table("documents").
		Where("document_id = ?", documentID).
		Updates(map[string]any{
			"format":             "docx",
			"version":            2,
			"content_version":    2,
			"source_blob_id":     currentBlobID,
			"source_file_name":   "current-v2.docx",
			"source_mime_type":   officeMimeType,
			"render_status":      "success",
			"render_error":       "",
			"rendered_at":        now,
			"updated_by_user_id": ownerUserID,
		}).Error; err != nil {
		t.Fatalf("seed current office document failed: %v", err)
	}
	if err := database.ORM.Table("document_file_revisions").Create([]map[string]any{
		{
			"document_file_revision_id": targetRevisionID,
			"document_id":               documentID,
			"blob_id":                   targetBlobID,
			"file_name":                 "target-v1.docx",
			"mime_type":                 officeMimeType,
			"version":                   1,
			"base_version":              0,
			"editor_user_id":            ownerUserID,
			"source":                    "remote",
			"created_at":                now,
		},
		{
			"document_file_revision_id": currentRevisionID,
			"document_id":               documentID,
			"blob_id":                   currentBlobID,
			"file_name":                 "current-v2.docx",
			"mime_type":                 officeMimeType,
			"version":                   2,
			"base_version":              1,
			"editor_user_id":            ownerUserID,
			"source":                    "remote",
			"created_at":                now.Add(time.Minute),
		},
	}).Error; err != nil {
		t.Fatalf("seed office file revisions failed: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/docs/"+documentID+"/revisions/"+targetRevisionID+"/restore",
		bytes.NewReader([]byte(`{"baseVersion":2}`)),
	)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", "application/json")
	rec := serve(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected restore office status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	payload := decodeJSONResultData[struct {
		Document struct {
			ID             string  `json:"id"`
			Format         string  `json:"format"`
			Version        int     `json:"version"`
			ContentVersion int     `json:"contentVersion"`
			SourceBlobID   *string `json:"sourceBlobId"`
			SourceFileName *string `json:"sourceFileName"`
			SourceMimeType *string `json:"sourceMimeType"`
		} `json:"document"`
		RestoredFromRevision struct {
			ID       string  `json:"id"`
			Format   string  `json:"format"`
			FileName *string `json:"fileName"`
			MimeType *string `json:"mimeType"`
		} `json:"restoredFromRevision"`
	}](t, rec.Body.Bytes())
	if payload.Document.ID != documentID || payload.Document.Format != "docx" || payload.Document.Version != 3 || payload.Document.ContentVersion != 3 {
		t.Fatalf("unexpected restored office document payload: %+v body=%s", payload.Document, rec.Body.String())
	}
	if payload.Document.SourceBlobID == nil || *payload.Document.SourceBlobID != targetBlobID {
		t.Fatalf("expected restored source blob %q, got %+v", targetBlobID, payload.Document.SourceBlobID)
	}
	if payload.Document.SourceFileName == nil || *payload.Document.SourceFileName != "target-v1.docx" {
		t.Fatalf("expected restored source file name target-v1.docx, got %+v", payload.Document.SourceFileName)
	}
	if payload.Document.SourceMimeType == nil || *payload.Document.SourceMimeType != officeMimeType {
		t.Fatalf("expected restored source mime %q, got %+v", officeMimeType, payload.Document.SourceMimeType)
	}
	if payload.RestoredFromRevision.ID != targetRevisionID ||
		payload.RestoredFromRevision.Format != "docx" ||
		payload.RestoredFromRevision.FileName == nil ||
		*payload.RestoredFromRevision.FileName != "target-v1.docx" ||
		payload.RestoredFromRevision.MimeType == nil ||
		*payload.RestoredFromRevision.MimeType != officeMimeType {
		t.Fatalf("unexpected restoredFromRevision payload: %+v body=%s", payload.RestoredFromRevision, rec.Body.String())
	}

	var persistedDoc struct {
		Version        int     `gorm:"column:version"`
		ContentVersion int     `gorm:"column:content_version"`
		SourceBlobID   *string `gorm:"column:source_blob_id"`
		SourceFileName *string `gorm:"column:source_file_name"`
		SourceMimeType *string `gorm:"column:source_mime_type"`
		RenderStatus   string  `gorm:"column:render_status"`
		RenderedAt     *string `gorm:"column:rendered_at"`
	}
	if err := database.ORM.Table("documents").
		Select("version", "content_version", "source_blob_id", "source_file_name", "source_mime_type", "render_status", "rendered_at").
		Where("document_id = ?", documentID).
		Take(&persistedDoc).Error; err != nil {
		t.Fatalf("query restored office document failed: %v", err)
	}
	if persistedDoc.Version != 3 || persistedDoc.ContentVersion != 3 ||
		persistedDoc.SourceBlobID == nil || *persistedDoc.SourceBlobID != targetBlobID ||
		persistedDoc.SourceFileName == nil || *persistedDoc.SourceFileName != "target-v1.docx" ||
		persistedDoc.SourceMimeType == nil || *persistedDoc.SourceMimeType != officeMimeType ||
		persistedDoc.RenderStatus != "pending" ||
		persistedDoc.RenderedAt != nil {
		t.Fatalf("unexpected restored office document row: %+v", persistedDoc)
	}

	var latestRevision struct {
		BlobID       string `gorm:"column:blob_id"`
		FileName     string `gorm:"column:file_name"`
		MimeType     string `gorm:"column:mime_type"`
		Version      int    `gorm:"column:version"`
		BaseVersion  int    `gorm:"column:base_version"`
		EditorUserID string `gorm:"column:editor_user_id"`
		Source       string `gorm:"column:source"`
	}
	if err := database.ORM.Table("document_file_revisions").
		Select("blob_id", "file_name", "mime_type", "version", "base_version", "editor_user_id", "source").
		Where("document_id = ?", documentID).
		Order("version DESC").
		Take(&latestRevision).Error; err != nil {
		t.Fatalf("query latest restored office file revision failed: %v", err)
	}
	if latestRevision.BlobID != targetBlobID ||
		latestRevision.FileName != "target-v1.docx" ||
		latestRevision.MimeType != officeMimeType ||
		latestRevision.Version != 3 ||
		latestRevision.BaseVersion != 2 ||
		latestRevision.EditorUserID != ownerUserID ||
		latestRevision.Source != "remote" {
		t.Fatalf("unexpected latest restored office file revision: %+v", latestRevision)
	}

	var originalRevision struct {
		BlobID      string `gorm:"column:blob_id"`
		Version     int    `gorm:"column:version"`
		BaseVersion int    `gorm:"column:base_version"`
	}
	if err := database.ORM.Table("document_file_revisions").
		Select("blob_id", "version", "base_version").
		Where("document_file_revision_id = ?", targetRevisionID).
		Take(&originalRevision).Error; err != nil {
		t.Fatalf("query original office file revision failed: %v", err)
	}
	if originalRevision.BlobID != targetBlobID || originalRevision.Version != 1 || originalRevision.BaseVersion != 0 {
		t.Fatalf("restore must keep original office file revision unchanged, got %+v", originalRevision)
	}

	var blobCount int64
	if err := database.ORM.Table("file_blobs").
		Where("blob_id IN ?", []string{targetBlobID, currentBlobID}).
		Count(&blobCount).Error; err != nil {
		t.Fatalf("count office blobs failed: %v", err)
	}
	if blobCount != 2 {
		t.Fatalf("restore must reuse existing office blobs, got blob count %d", blobCount)
	}
}

func TestRouter_RestoreOfficeDocumentRevisionRejectsAdvancedContentVersion(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "revision-restore-office-conflict-owner@example.com")
	spaceID := "01kofficeconflictspace00001"
	nodeID := "01kofficeconflictnode000001"
	documentID := "01kofficeconflictdoc0000001"
	targetRevisionID := "01kofficeconflictrev0000001"
	targetBlobID := "01kofficeconflictblob000001"
	currentBlobID := "01kofficeconflictblob000002"
	officeMimeType := "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	now := time.Now().UTC()

	seedSpaceAndDocumentForAccess(t, database, ownerUserID, spaceID, nodeID, documentID, "member", "member")
	if err := database.ORM.Table("file_blobs").Create([]map[string]any{
		{
			"blob_id":           targetBlobID,
			"storage_provider":  "local",
			"object_key":        "uploads/revisions/office-conflict-target.docx",
			"object_url":        "/uploads/revisions/office-conflict-target.docx",
			"mime_type":         officeMimeType,
			"size_bytes":        1024,
			"content_hash_algo": "sha256",
			"content_hash":      "office-conflict-target-hash",
			"created_at":        now,
			"updated_at":        now,
		},
		{
			"blob_id":           currentBlobID,
			"storage_provider":  "local",
			"object_key":        "uploads/revisions/office-conflict-current.docx",
			"object_url":        "/uploads/revisions/office-conflict-current.docx",
			"mime_type":         officeMimeType,
			"size_bytes":        2048,
			"content_hash_algo": "sha256",
			"content_hash":      "office-conflict-current-hash",
			"created_at":        now,
			"updated_at":        now,
		},
	}).Error; err != nil {
		t.Fatalf("seed office conflict blobs failed: %v", err)
	}
	if err := database.ORM.Table("documents").
		Where("document_id = ?", documentID).
		Updates(map[string]any{
			"format":           "docx",
			"version":          2,
			"content_version":  3,
			"source_blob_id":   currentBlobID,
			"source_file_name": "current-v3.docx",
			"source_mime_type": officeMimeType,
		}).Error; err != nil {
		t.Fatalf("seed advanced office document failed: %v", err)
	}
	if err := database.ORM.Table("document_file_revisions").Create(map[string]any{
		"document_file_revision_id": targetRevisionID,
		"document_id":               documentID,
		"blob_id":                   targetBlobID,
		"file_name":                 "target-v1.docx",
		"mime_type":                 officeMimeType,
		"version":                   1,
		"base_version":              0,
		"editor_user_id":            ownerUserID,
		"source":                    "remote",
		"created_at":                now,
	}).Error; err != nil {
		t.Fatalf("seed target office file revision failed: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/docs/"+documentID+"/revisions/"+targetRevisionID+"/restore",
		bytes.NewReader([]byte(`{"baseVersion":2}`)),
	)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", "application/json")
	rec := serve(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected normalized office conflict status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	expectedCode := response.ResolveErrorCode(response.CodeDocumentVersionConflict)
	if code := decodeJSONResultCode(t, rec.Body.Bytes()); code != expectedCode {
		t.Fatalf("expected office document version conflict code %d, got %d body=%s", expectedCode, code, rec.Body.String())
	}

	payload := decodeJSONResultData[struct {
		LatestDocument struct {
			ID             string  `json:"id"`
			Version        int     `json:"version"`
			ContentVersion int     `json:"contentVersion"`
			SourceBlobID   *string `json:"sourceBlobId"`
		} `json:"latestDocument"`
	}](t, rec.Body.Bytes())
	if payload.LatestDocument.ID != documentID ||
		payload.LatestDocument.Version != 2 ||
		payload.LatestDocument.ContentVersion != 3 ||
		payload.LatestDocument.SourceBlobID == nil ||
		*payload.LatestDocument.SourceBlobID != currentBlobID {
		t.Fatalf("unexpected latest document on office conflict: %+v body=%s", payload.LatestDocument, rec.Body.String())
	}

	var persistedDoc struct {
		SourceBlobID *string `gorm:"column:source_blob_id"`
	}
	if err := database.ORM.Table("documents").
		Select("source_blob_id").
		Where("document_id = ?", documentID).
		Take(&persistedDoc).Error; err != nil {
		t.Fatalf("query office conflict document failed: %v", err)
	}
	if persistedDoc.SourceBlobID == nil || *persistedDoc.SourceBlobID != currentBlobID {
		t.Fatalf("office conflict must not overwrite source blob, got %+v", persistedDoc.SourceBlobID)
	}

	var fileRevisionCount int64
	if err := database.ORM.Table("document_file_revisions").
		Where("document_id = ?", documentID).
		Count(&fileRevisionCount).Error; err != nil {
		t.Fatalf("count office conflict file revisions failed: %v", err)
	}
	if fileRevisionCount != 1 {
		t.Fatalf("office conflict must not append file revision, got %d", fileRevisionCount)
	}
}

func TestRouter_RestoreDocumentRevisionUsesWritePermissionAndRequestID(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, _ := registerAccessUser(t, serve, "revision-restore-permission-owner@example.com")
	readerUserID, _, readerToken := registerAccessUser(t, serve, "revision-restore-permission-reader@example.com")
	collaboratorUserID, _, collaboratorToken := registerAccessUser(t, serve, "revision-restore-permission-collaborator@example.com")
	spaceID := "01krestorepermissionspace001"
	nodeID := "01krestorepermissionnode001"
	documentID := "01krestorepermissiondoc0001"
	revisionID := "01krestorepermissionrev0001"
	now := time.Now().UTC()

	seedSpaceAndDocumentForAccess(t, database, ownerUserID, spaceID, nodeID, documentID, "member", "member")
	seedSpaceMemberForAccess(t, database, spaceID, readerUserID, "reader")
	seedSpaceMemberForAccess(t, database, spaceID, collaboratorUserID, "collaborator")
	if err := database.ORM.Table("documents").
		Where("document_id = ?", documentID).
		Updates(map[string]any{
			"content_md":      "# 当前正文",
			"version":         1,
			"content_version": 1,
			"format":          "markdown",
		}).Error; err != nil {
		t.Fatalf("seed permission document failed: %v", err)
	}
	if err := database.ORM.Table("document_revisions").Create(map[string]any{
		"document_revision_id": revisionID,
		"document_id":          documentID,
		"version":              1,
		"content_md":           "# 协作者恢复正文",
		"base_version":         0,
		"editor_user_id":       ownerUserID,
		"source":               "remote",
		"created_at":           now,
	}).Error; err != nil {
		t.Fatalf("seed permission revision failed: %v", err)
	}

	readerReq := httptest.NewRequest(
		http.MethodPost,
		"/api/docs/"+documentID+"/revisions/"+revisionID+"/restore",
		bytes.NewReader([]byte(`{"baseVersion":1}`)),
	)
	readerReq.Header.Set("Authorization", "Bearer "+readerToken)
	readerReq.Header.Set("Content-Type", "application/json")
	readerRec := serve(readerReq)
	if readerRec.Code != http.StatusForbidden {
		t.Fatalf("expected reader restore status 403, got %d body=%s", readerRec.Code, readerRec.Body.String())
	}
	var readerEnvelope jsonResultEnvelopeForTest
	if err := json.Unmarshal(readerRec.Body.Bytes(), &readerEnvelope); err != nil {
		t.Fatalf("decode reader restore error failed: %v body=%s", err, readerRec.Body.String())
	}
	if readerEnvelope.Code != response.ResolveErrorCode(response.CodeForbidden) {
		t.Fatalf("expected forbidden code for reader restore, got %d body=%s", readerEnvelope.Code, readerRec.Body.String())
	}
	if strings.TrimSpace(readerEnvelope.RequestID) == "" {
		t.Fatalf("expected requestId for reader restore error, body=%s", readerRec.Body.String())
	}

	var revisionCountAfterReader int64
	if err := database.ORM.Table("document_revisions").
		Where("document_id = ?", documentID).
		Count(&revisionCountAfterReader).Error; err != nil {
		t.Fatalf("count revisions after reader restore failed: %v", err)
	}
	if revisionCountAfterReader != 1 {
		t.Fatalf("reader restore must not append revision, got %d", revisionCountAfterReader)
	}

	collaboratorReq := httptest.NewRequest(
		http.MethodPost,
		"/api/docs/"+documentID+"/revisions/"+revisionID+"/restore",
		bytes.NewReader([]byte(`{"baseVersion":1}`)),
	)
	collaboratorReq.Header.Set("Authorization", "Bearer "+collaboratorToken)
	collaboratorReq.Header.Set("Content-Type", "application/json")
	collaboratorRec := serve(collaboratorReq)
	if collaboratorRec.Code != http.StatusOK {
		t.Fatalf("expected collaborator restore status 200, got %d body=%s", collaboratorRec.Code, collaboratorRec.Body.String())
	}
	collaboratorPayload := decodeJSONResultData[struct {
		Document struct {
			ID        string `json:"id"`
			ContentMD string `json:"contentMd"`
			Version   int    `json:"version"`
		} `json:"document"`
	}](t, collaboratorRec.Body.Bytes())
	if collaboratorPayload.Document.ID != documentID ||
		collaboratorPayload.Document.ContentMD != "# 协作者恢复正文" ||
		collaboratorPayload.Document.Version != 2 {
		t.Fatalf("unexpected collaborator restore payload: %+v body=%s", collaboratorPayload.Document, collaboratorRec.Body.String())
	}

	var latestRevision struct {
		Version      int    `gorm:"column:version"`
		BaseVersion  int    `gorm:"column:base_version"`
		EditorUserID string `gorm:"column:editor_user_id"`
	}
	if err := database.ORM.Table("document_revisions").
		Select("version", "base_version", "editor_user_id").
		Where("document_id = ?", documentID).
		Order("version DESC").
		Take(&latestRevision).Error; err != nil {
		t.Fatalf("query collaborator restored revision failed: %v", err)
	}
	if latestRevision.Version != 2 || latestRevision.BaseVersion != 1 || latestRevision.EditorUserID != collaboratorUserID {
		t.Fatalf("unexpected collaborator restored revision: %+v", latestRevision)
	}
}

func assertRawJSONFieldEquals(t *testing.T, fields map[string]json.RawMessage, fieldName string, expected string) {
	t.Helper()

	var actual string
	if err := json.Unmarshal(fields[fieldName], &actual); err != nil {
		t.Fatalf("decode field %s failed: %v", fieldName, err)
	}
	if actual != expected {
		t.Fatalf("expected field %s=%q, got %q", fieldName, expected, actual)
	}
}
