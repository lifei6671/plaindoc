package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage"
)

func TestRouter_GetDocument_VisibilityAccessMatrix(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "owner-doc@example.com")
	viewerUserID, _, viewerToken := registerAccessUser(t, serve, "viewer-doc@example.com")

	spaceID := "01h1spaceaccess0000000000001"
	nodeID := "01h1nodeaccess00000000000002"
	docID := "01h1docaccess000000000000003"
	seedSpaceAndDocumentForAccess(t, database, ownerUserID, spaceID, nodeID, docID, "public", "member")

	anonReq := httptest.NewRequest(http.MethodGet, "/api/docs/"+docID, nil)
	anonRec := serve(anonReq)
	if anonRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected anonymous access status 401, got %d body=%s", anonRec.Code, anonRec.Body.String())
	}

	viewerReq := httptest.NewRequest(http.MethodGet, "/api/docs/"+docID, nil)
	viewerReq.Header.Set("Authorization", "Bearer "+viewerToken)
	viewerRec := serve(viewerReq)
	if viewerRec.Code != http.StatusForbidden {
		t.Fatalf("expected non-member access status 403, got %d body=%s", viewerRec.Code, viewerRec.Body.String())
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := database.ORM.Table("space_members").Create(map[string]any{
		"space_id":   spaceID,
		"user_id":    viewerUserID,
		"role":       "reader",
		"created_at": now,
		"updated_at": now,
	}).Error; err != nil {
		t.Fatalf("insert space member failed: %v", err)
	}

	memberReq := httptest.NewRequest(http.MethodGet, "/api/docs/"+docID, nil)
	memberReq.Header.Set("Authorization", "Bearer "+viewerToken)
	memberRec := serve(memberReq)
	if memberRec.Code != http.StatusOK {
		t.Fatalf("expected member access status 200, got %d body=%s", memberRec.Code, memberRec.Body.String())
	}

	var payload struct {
		ID         string `json:"id"`
		Visibility string `json:"visibility"`
	}
	if err := json.Unmarshal(memberRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode document response failed: %v", err)
	}
	if payload.ID != docID {
		t.Fatalf("expected document id %s, got %s", docID, payload.ID)
	}
	if payload.Visibility != "member" {
		t.Fatalf("expected visibility member, got %s", payload.Visibility)
	}

	// owner 也应可访问 member 文档。
	ownerReq := httptest.NewRequest(http.MethodGet, "/api/docs/"+docID, nil)
	ownerReq.Header.Set("Authorization", "Bearer "+ownerToken)
	ownerRec := serve(ownerReq)
	if ownerRec.Code != http.StatusOK {
		t.Fatalf("expected owner access status 200, got %d body=%s", ownerRec.Code, ownerRec.Body.String())
	}
}

func TestRouter_GetSpace_AuthenticatedVisibility(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, _ := registerAccessUser(t, serve, "owner-space@example.com")
	_, _, viewerToken := registerAccessUser(t, serve, "viewer-space@example.com")

	spaceID := "01h1spaceauth00000000000001"
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := database.ORM.Table("spaces").Create(map[string]any{
		"space_id":      spaceID,
		"name":          "Auth Space",
		"owner_user_id": ownerUserID,
		"visibility":    "authenticated",
		"created_at":    now,
		"updated_at":    now,
	}).Error; err != nil {
		t.Fatalf("insert space failed: %v", err)
	}

	anonReq := httptest.NewRequest(http.MethodGet, "/api/spaces/"+spaceID, nil)
	anonRec := serve(anonReq)
	if anonRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected anonymous access status 401, got %d body=%s", anonRec.Code, anonRec.Body.String())
	}

	viewerReq := httptest.NewRequest(http.MethodGet, "/api/spaces/"+spaceID, nil)
	viewerReq.Header.Set("Authorization", "Bearer "+viewerToken)
	viewerRec := serve(viewerReq)
	if viewerRec.Code != http.StatusOK {
		t.Fatalf("expected authenticated access status 200, got %d body=%s", viewerRec.Code, viewerRec.Body.String())
	}
}

func TestRouter_UpdateVisibility_OnlyOwnerCanUpdate(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "owner-update@example.com")
	_, _, viewerToken := registerAccessUser(t, serve, "viewer-update@example.com")

	spaceID := "01h1spaceupdate0000000000001"
	nodeID := "01h1nodeupdate00000000000002"
	docID := "01h1docupdate000000000000003"
	seedSpaceAndDocumentForAccess(t, database, ownerUserID, spaceID, nodeID, docID, "member", "member")

	updateSpaceBody := []byte(`{"visibility":"public"}`)
	updateSpaceByViewerReq := httptest.NewRequest(http.MethodPut, "/api/spaces/"+spaceID+"/visibility", bytes.NewReader(updateSpaceBody))
	updateSpaceByViewerReq.Header.Set("Authorization", "Bearer "+viewerToken)
	updateSpaceByViewerReq.Header.Set("Content-Type", "application/json")
	updateSpaceByViewerRec := serve(updateSpaceByViewerReq)
	if updateSpaceByViewerRec.Code != http.StatusForbidden {
		t.Fatalf("expected non-owner update space status 403, got %d body=%s", updateSpaceByViewerRec.Code, updateSpaceByViewerRec.Body.String())
	}

	updateSpaceByOwnerReq := httptest.NewRequest(http.MethodPut, "/api/spaces/"+spaceID+"/visibility", bytes.NewReader(updateSpaceBody))
	updateSpaceByOwnerReq.Header.Set("Authorization", "Bearer "+ownerToken)
	updateSpaceByOwnerReq.Header.Set("Content-Type", "application/json")
	updateSpaceByOwnerRec := serve(updateSpaceByOwnerReq)
	if updateSpaceByOwnerRec.Code != http.StatusOK {
		t.Fatalf("expected owner update space status 200, got %d body=%s", updateSpaceByOwnerRec.Code, updateSpaceByOwnerRec.Body.String())
	}

	updateDocumentBody := []byte(`{"visibility":"authenticated"}`)
	updateDocumentByViewerReq := httptest.NewRequest(http.MethodPut, "/api/docs/"+docID+"/visibility", bytes.NewReader(updateDocumentBody))
	updateDocumentByViewerReq.Header.Set("Authorization", "Bearer "+viewerToken)
	updateDocumentByViewerReq.Header.Set("Content-Type", "application/json")
	updateDocumentByViewerRec := serve(updateDocumentByViewerReq)
	if updateDocumentByViewerRec.Code != http.StatusForbidden {
		t.Fatalf("expected non-owner update document status 403, got %d body=%s", updateDocumentByViewerRec.Code, updateDocumentByViewerRec.Body.String())
	}

	updateDocumentByOwnerReq := httptest.NewRequest(http.MethodPut, "/api/docs/"+docID+"/visibility", bytes.NewReader(updateDocumentBody))
	updateDocumentByOwnerReq.Header.Set("Authorization", "Bearer "+ownerToken)
	updateDocumentByOwnerReq.Header.Set("Content-Type", "application/json")
	updateDocumentByOwnerRec := serve(updateDocumentByOwnerReq)
	if updateDocumentByOwnerRec.Code != http.StatusOK {
		t.Fatalf("expected owner update document status 200, got %d body=%s", updateDocumentByOwnerRec.Code, updateDocumentByOwnerRec.Body.String())
	}

	var persistedDoc struct {
		Visibility string `gorm:"column:visibility"`
	}
	if err := database.ORM.Table("documents").
		Select("visibility").
		Where("document_id = ?", docID).
		Scan(&persistedDoc).Error; err != nil {
		t.Fatalf("query document visibility failed: %v", err)
	}
	if persistedDoc.Visibility != "authenticated" {
		t.Fatalf("expected persisted document visibility authenticated, got %s", persistedDoc.Visibility)
	}
}

func registerAccessUser(
	t *testing.T,
	serve func(*http.Request) *httptest.ResponseRecorder,
	email string,
) (string, string, string) {
	t.Helper()

	body := []byte(`{"email":"` + email + `","password":"123456","name":"Test User"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := serve(req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register failed, status=%d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		User struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"user"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode register response failed: %v", err)
	}
	if payload.User.ID == "" || payload.Token == "" {
		t.Fatalf("register response missing id/token, body=%s", rec.Body.String())
	}
	return payload.User.ID, payload.User.Email, payload.Token
}

func seedSpaceAndDocumentForAccess(
	t *testing.T,
	database *storage.Database,
	ownerUserID string,
	spaceID string,
	nodeID string,
	docID string,
	spaceVisibility string,
	documentVisibility string,
) {
	t.Helper()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := database.ORM.Table("spaces").Create(map[string]any{
		"space_id":      spaceID,
		"name":          "Access Space",
		"owner_user_id": ownerUserID,
		"visibility":    spaceVisibility,
		"created_at":    now,
		"updated_at":    now,
	}).Error; err != nil {
		t.Fatalf("insert space failed: %v", err)
	}

	if err := database.ORM.Table("nodes").Create(map[string]any{
		"node_id":        nodeID,
		"space_id":       spaceID,
		"parent_node_id": nil,
		"type":           "doc",
		"title":          "Access Doc",
		"sort":           1,
		"created_at":     now,
		"updated_at":     now,
	}).Error; err != nil {
		t.Fatalf("insert node failed: %v", err)
	}

	if err := database.ORM.Table("documents").Create(map[string]any{
		"document_id":        docID,
		"node_id":            nodeID,
		"theme_id":           "default",
		"visibility":         documentVisibility,
		"title":              "Access Doc",
		"content_md":         "# hello",
		"version":            1,
		"updated_by_user_id": ownerUserID,
		"created_at":         now,
		"updated_at":         now,
	}).Error; err != nil {
		t.Fatalf("insert document failed: %v", err)
	}
}
