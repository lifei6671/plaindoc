package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
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
	if anonRec.Code != http.StatusForbidden {
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

	payload := decodeJSONResultData[struct {
		ID         string `json:"id"`
		Visibility string `json:"visibility"`
	}](t, memberRec.Body.Bytes())
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
	if anonRec.Code != http.StatusForbidden {
		t.Fatalf("expected anonymous access status 401, got %d body=%s", anonRec.Code, anonRec.Body.String())
	}

	viewerReq := httptest.NewRequest(http.MethodGet, "/api/spaces/"+spaceID, nil)
	viewerReq.Header.Set("Authorization", "Bearer "+viewerToken)
	viewerRec := serve(viewerReq)
	if viewerRec.Code != http.StatusOK {
		t.Fatalf("expected authenticated access status 200, got %d body=%s", viewerRec.Code, viewerRec.Body.String())
	}
}

func TestRouter_UpdateVisibility_AccessControl(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "owner-update@example.com")
	_, _, viewerToken := registerAccessUser(t, serve, "viewer-update@example.com")
	collaboratorUserID, _, collaboratorToken := registerAccessUser(t, serve, "collaborator-update@example.com")
	readerUserID, _, readerToken := registerAccessUser(t, serve, "reader-update@example.com")

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
		t.Fatalf("expected non-member update document status 403, got %d body=%s", updateDocumentByViewerRec.Code, updateDocumentByViewerRec.Body.String())
	}

	seedSpaceMemberForAccess(t, database, spaceID, collaboratorUserID, "collaborator")
	updateDocumentByCollaboratorReq := httptest.NewRequest(http.MethodPut, "/api/docs/"+docID+"/visibility", bytes.NewReader(updateDocumentBody))
	updateDocumentByCollaboratorReq.Header.Set("Authorization", "Bearer "+collaboratorToken)
	updateDocumentByCollaboratorReq.Header.Set("Content-Type", "application/json")
	updateDocumentByCollaboratorRec := serve(updateDocumentByCollaboratorReq)
	if updateDocumentByCollaboratorRec.Code != http.StatusOK {
		t.Fatalf(
			"expected collaborator update document status 200, got %d body=%s",
			updateDocumentByCollaboratorRec.Code,
			updateDocumentByCollaboratorRec.Body.String(),
		)
	}

	seedSpaceMemberForAccess(t, database, spaceID, readerUserID, "reader")
	updateDocumentByReaderReq := httptest.NewRequest(http.MethodPut, "/api/docs/"+docID+"/visibility", bytes.NewReader([]byte(`{"visibility":"member"}`)))
	updateDocumentByReaderReq.Header.Set("Authorization", "Bearer "+readerToken)
	updateDocumentByReaderReq.Header.Set("Content-Type", "application/json")
	updateDocumentByReaderRec := serve(updateDocumentByReaderReq)
	if updateDocumentByReaderRec.Code != http.StatusForbidden {
		t.Fatalf("expected reader update document status 403, got %d body=%s", updateDocumentByReaderRec.Code, updateDocumentByReaderRec.Body.String())
	}

	updateDocumentByOwnerReq := httptest.NewRequest(http.MethodPut, "/api/docs/"+docID+"/visibility", bytes.NewReader([]byte(`{"visibility":"public"}`)))
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
	if persistedDoc.Visibility != "public" {
		t.Fatalf("expected persisted document visibility public, got %s", persistedDoc.Visibility)
	}
}

func TestRouter_PlatformAdminCanManageDocumentWithoutMembership(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, _ := registerAccessUser(t, serve, "owner-platform-admin-doc@example.com")
	platformAdminUserID, _, platformAdminToken := registerAccessUser(t, serve, "platform-admin-doc@example.com")
	grantPlatformAdminRoleForAccess(t, database, platformAdminUserID)

	spaceID := "01h1spaceplatformadmin0000001"
	nodeID := "01h1nodeplatformadmin00000002"
	docID := "01h1docplatformadmin000000003"
	seedSpaceAndDocumentForAccess(t, database, ownerUserID, spaceID, nodeID, docID, "member", "member")

	getDocumentReq := httptest.NewRequest(http.MethodGet, "/api/docs/"+docID, nil)
	getDocumentReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	getDocumentRec := serve(getDocumentReq)
	if getDocumentRec.Code != http.StatusOK {
		t.Fatalf(
			"expected platform admin read document status 200, got %d body=%s",
			getDocumentRec.Code,
			getDocumentRec.Body.String(),
		)
	}

	updateVisibilityReq := httptest.NewRequest(
		http.MethodPut,
		"/api/docs/"+docID+"/visibility",
		bytes.NewReader([]byte(`{"visibility":"public"}`)),
	)
	updateVisibilityReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	updateVisibilityReq.Header.Set("Content-Type", "application/json")
	updateVisibilityRec := serve(updateVisibilityReq)
	if updateVisibilityRec.Code != http.StatusOK {
		t.Fatalf(
			"expected platform admin update document visibility status 200, got %d body=%s",
			updateVisibilityRec.Code,
			updateVisibilityRec.Body.String(),
		)
	}

	saveDocumentReq := httptest.NewRequest(
		http.MethodPut,
		"/api/docs/"+docID,
		bytes.NewReader([]byte(`{"contentMd":"# updated by platform admin","baseVersion":1}`)),
	)
	saveDocumentReq.Header.Set("Authorization", "Bearer "+platformAdminToken)
	saveDocumentReq.Header.Set("Content-Type", "application/json")
	saveDocumentRec := serve(saveDocumentReq)
	if saveDocumentRec.Code != http.StatusOK {
		t.Fatalf(
			"expected platform admin save document status 200, got %d body=%s",
			saveDocumentRec.Code,
			saveDocumentRec.Body.String(),
		)
	}

	savePayload := decodeJSONResultData[struct {
		Document struct {
			ID      string `json:"id"`
			Version int    `json:"version"`
		} `json:"document"`
	}](t, saveDocumentRec.Body.Bytes())
	if savePayload.Document.ID != docID {
		t.Fatalf("expected saved document id %s, got %s", docID, savePayload.Document.ID)
	}
	if savePayload.Document.Version != 2 {
		t.Fatalf("expected saved document version 2, got %d", savePayload.Document.Version)
	}

	var persistedDoc struct {
		Visibility      string `gorm:"column:visibility"`
		ContentMD       string `gorm:"column:content_md"`
		Version         int    `gorm:"column:version"`
		UpdatedByUserID string `gorm:"column:updated_by_user_id"`
	}
	if err := database.ORM.Table("documents").
		Select("visibility", "content_md", "version", "updated_by_user_id").
		Where("document_id = ?", docID).
		Take(&persistedDoc).Error; err != nil {
		t.Fatalf("query persisted document failed: %v", err)
	}
	if persistedDoc.Visibility != "public" {
		t.Fatalf("expected persisted document visibility public, got %s", persistedDoc.Visibility)
	}
	if persistedDoc.ContentMD != "# updated by platform admin" {
		t.Fatalf("expected persisted document content updated by platform admin, got %q", persistedDoc.ContentMD)
	}
	if persistedDoc.Version != 2 {
		t.Fatalf("expected persisted document version 2, got %d", persistedDoc.Version)
	}
	if persistedDoc.UpdatedByUserID != platformAdminUserID {
		t.Fatalf("expected updated_by_user_id %s, got %s", platformAdminUserID, persistedDoc.UpdatedByUserID)
	}
}

func TestRouter_CreateNode_DocumentVisibilityInheritsSpace(t *testing.T) {
	testCases := []struct {
		name            string
		spaceVisibility string
	}{
		{name: "PublicSpace", spaceVisibility: "public"},
		{name: "AuthenticatedSpace", spaceVisibility: "authenticated"},
		{name: "MemberSpace", spaceVisibility: "member"},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			database, serve := setupAuthTestRouter(t)
			defer func() {
				_ = database.Close()
			}()

			ownerUserID, _, ownerToken := registerAccessUser(t, serve, "owner-create-node-"+testCase.name+"@example.com")
			spaceID := "01h1createnodespace" + strings.ToLower(testCase.name) + "0000001"
			seedSpaceForWorkspaceCreateNode(t, database, ownerUserID, spaceID, testCase.spaceVisibility)

			body := []byte(`{"parentId":null,"type":"doc","title":"Inherited Visibility Doc"}`)
			req := httptest.NewRequest(http.MethodPost, "/api/spaces/"+spaceID+"/nodes", bytes.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+ownerToken)
			req.Header.Set("Content-Type", "application/json")
			rec := serve(req)
			if rec.Code != http.StatusOK {
				t.Fatalf("expected create node status 200, got %d body=%s", rec.Code, rec.Body.String())
			}

			payload := decodeJSONResultData[struct {
				NodeID string `json:"nodeId"`
				DocID  string `json:"docId"`
			}](t, rec.Body.Bytes())
			if payload.DocID == "" {
				t.Fatalf("expected created doc id in response, body=%s", rec.Body.String())
			}

			var persistedDoc struct {
				Visibility string `gorm:"column:visibility"`
			}
			if err := database.ORM.Table("documents").
				Select("visibility").
				Where("document_id = ?", payload.DocID).
				Take(&persistedDoc).Error; err != nil {
				t.Fatalf("query created document visibility failed: %v", err)
			}
			if persistedDoc.Visibility != testCase.spaceVisibility {
				t.Fatalf(
					"expected created document visibility %s, got %s",
					testCase.spaceVisibility,
					persistedDoc.Visibility,
				)
			}
		})
	}
}

func TestRouter_ReaderPage_UnauthorizedRendersForbiddenInsteadOfRedirect(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, _ := registerAccessUser(t, serve, "owner-reader-page@example.com")
	spaceID := "01h1readerpageauth000000000001"
	nodeID := "01h1readerpagenode000000000002"
	docID := "01h1readerpagedoc0000000000003"
	seedSpaceAndDocumentForAccess(t, database, ownerUserID, spaceID, nodeID, docID, "authenticated", "authenticated")

	pageReq := httptest.NewRequest(http.MethodGet, "/r/"+spaceID+"/"+docID, nil)
	pageRec := serve(pageReq)
	if pageRec.Code != http.StatusForbidden {
		t.Fatalf("expected reader page unauthorized status 403, got %d body=%s", pageRec.Code, pageRec.Body.String())
	}
	if location := pageRec.Header().Get("Location"); location != "" {
		t.Fatalf("expected no redirect location, got %q", location)
	}
	if !strings.Contains(pageRec.Body.String(), "无权限访问") {
		t.Fatalf("expected forbidden body contains 无权限访问, body=%s", pageRec.Body.String())
	}

	spaceReq := httptest.NewRequest(http.MethodGet, "/r/"+spaceID, nil)
	spaceRec := serve(spaceReq)
	if spaceRec.Code != http.StatusForbidden {
		t.Fatalf("expected reader space unauthorized status 403, got %d body=%s", spaceRec.Code, spaceRec.Body.String())
	}
	if location := spaceRec.Header().Get("Location"); location != "" {
		t.Fatalf("expected no redirect location for reader space, got %q", location)
	}
}

func TestRouter_UpdateDocumentIdentifier_AccessControlAndConflict(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "owner-doc-identifier@example.com")
	collaboratorUserID, _, collaboratorToken := registerAccessUser(
		t,
		serve,
		"collaborator-doc-identifier@example.com",
	)
	readerUserID, _, readerToken := registerAccessUser(t, serve, "reader-doc-identifier@example.com")
	_, _, outsiderToken := registerAccessUser(t, serve, "outsider-doc-identifier@example.com")

	spaceID := "01h1spaceidentifier00000000001"
	firstNodeID := "01h1nodeidentifier000000000001"
	firstDocID := "01h1docidentifier000000000001"
	secondNodeID := "01h1nodeidentifier000000000002"
	secondDocID := "01h1docidentifier000000000002"

	seedSpaceAndDocumentForAccess(t, database, ownerUserID, spaceID, firstNodeID, firstDocID, "member", "member")
	seedSpaceMemberForAccess(t, database, spaceID, collaboratorUserID, "collaborator")
	seedSpaceMemberForAccess(t, database, spaceID, readerUserID, "reader")

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := database.ORM.Table("nodes").Create(map[string]any{
		"node_id":        secondNodeID,
		"space_id":       spaceID,
		"parent_node_id": nil,
		"type":           "doc",
		"title":          "Conflict Doc",
		"sort":           2,
		"created_at":     now,
		"updated_at":     now,
	}).Error; err != nil {
		t.Fatalf("insert second node failed: %v", err)
	}
	if err := database.ORM.Table("documents").Create(map[string]any{
		"document_id":        secondDocID,
		"node_id":            secondNodeID,
		"theme_id":           "default",
		"visibility":         "member",
		"title":              "Conflict Doc",
		"content_md":         "# conflict",
		"version":            1,
		"updated_by_user_id": ownerUserID,
		"created_at":         now,
		"updated_at":         now,
	}).Error; err != nil {
		t.Fatalf("insert second document failed: %v", err)
	}

	updateBody := []byte(`{"identifier":"quick-start"}`)

	outsiderReq := httptest.NewRequest(http.MethodPatch, "/api/docs/"+firstDocID+"/identifier", bytes.NewReader(updateBody))
	outsiderReq.Header.Set("Authorization", "Bearer "+outsiderToken)
	outsiderReq.Header.Set("Content-Type", "application/json")
	outsiderRec := serve(outsiderReq)
	if outsiderRec.Code != http.StatusForbidden {
		t.Fatalf("expected outsider patch status 403, got %d body=%s", outsiderRec.Code, outsiderRec.Body.String())
	}

	readerReq := httptest.NewRequest(http.MethodPatch, "/api/docs/"+firstDocID+"/identifier", bytes.NewReader(updateBody))
	readerReq.Header.Set("Authorization", "Bearer "+readerToken)
	readerReq.Header.Set("Content-Type", "application/json")
	readerRec := serve(readerReq)
	if readerRec.Code != http.StatusForbidden {
		t.Fatalf("expected reader patch status 403, got %d body=%s", readerRec.Code, readerRec.Body.String())
	}

	collaboratorReq := httptest.NewRequest(http.MethodPatch, "/api/docs/"+firstDocID+"/identifier", bytes.NewReader(updateBody))
	collaboratorReq.Header.Set("Authorization", "Bearer "+collaboratorToken)
	collaboratorReq.Header.Set("Content-Type", "application/json")
	collaboratorRec := serve(collaboratorReq)
	if collaboratorRec.Code != http.StatusOK {
		t.Fatalf("expected collaborator patch status 200, got %d body=%s", collaboratorRec.Code, collaboratorRec.Body.String())
	}

	updatePayload := decodeJSONResultData[struct {
		DocumentID string  `json:"documentId"`
		Identifier *string `json:"identifier"`
		ReaderURL  string  `json:"readerUrl"`
	}](t, collaboratorRec.Body.Bytes())
	if updatePayload.DocumentID != firstDocID {
		t.Fatalf("expected document id %s, got %s", firstDocID, updatePayload.DocumentID)
	}
	if updatePayload.Identifier == nil || *updatePayload.Identifier != "quick-start" {
		t.Fatalf("expected identifier quick-start, got %#v", updatePayload.Identifier)
	}
	if !strings.HasSuffix(updatePayload.ReaderURL, "/"+spaceID+"/quick-start") {
		t.Fatalf("expected reader url suffix /%s/quick-start, got %s", spaceID, updatePayload.ReaderURL)
	}

	conflictReq := httptest.NewRequest(
		http.MethodPatch,
		"/api/docs/"+secondDocID+"/identifier",
		bytes.NewReader([]byte(`{"identifier":"quick-start"}`)),
	)
	conflictReq.Header.Set("Authorization", "Bearer "+ownerToken)
	conflictReq.Header.Set("Content-Type", "application/json")
	conflictRec := serve(conflictReq)
	if conflictRec.Code != http.StatusOK {
		t.Fatalf("expected conflict http status 200, got %d body=%s", conflictRec.Code, conflictRec.Body.String())
	}
	if decodeJSONResultCode(t, conflictRec.Body.Bytes()) != response.ResolveErrorCode(response.CodeDocumentIdentifierConflict) {
		t.Fatalf(
			"expected conflict business code %d, got %d body=%s",
			response.ResolveErrorCode(response.CodeDocumentIdentifierConflict),
			decodeJSONResultCode(t, conflictRec.Body.Bytes()),
			conflictRec.Body.String(),
		)
	}

	invalidReq := httptest.NewRequest(
		http.MethodPatch,
		"/api/docs/"+firstDocID+"/identifier",
		bytes.NewReader([]byte(`{"identifier":"api"}`)),
	)
	invalidReq.Header.Set("Authorization", "Bearer "+ownerToken)
	invalidReq.Header.Set("Content-Type", "application/json")
	invalidRec := serve(invalidReq)
	if invalidRec.Code != http.StatusOK {
		t.Fatalf("expected reserved identifier http status 200, got %d body=%s", invalidRec.Code, invalidRec.Body.String())
	}
	if decodeJSONResultCode(t, invalidRec.Body.Bytes()) != response.ResolveErrorCode(response.CodeDocumentIdentifierReserved) {
		t.Fatalf(
			"expected reserved business code %d, got %d body=%s",
			response.ResolveErrorCode(response.CodeDocumentIdentifierReserved),
			decodeJSONResultCode(t, invalidRec.Body.Bytes()),
			invalidRec.Body.String(),
		)
	}

	clearReq := httptest.NewRequest(
		http.MethodPatch,
		"/api/docs/"+firstDocID+"/identifier",
		bytes.NewReader([]byte(`{"identifier":""}`)),
	)
	clearReq.Header.Set("Authorization", "Bearer "+ownerToken)
	clearReq.Header.Set("Content-Type", "application/json")
	clearRec := serve(clearReq)
	if clearRec.Code != http.StatusOK {
		t.Fatalf("expected clear identifier status 200, got %d body=%s", clearRec.Code, clearRec.Body.String())
	}

	var persistedNode struct {
		ReaderSlug *string `gorm:"column:reader_slug"`
	}
	if err := database.ORM.Table("nodes").
		Select("reader_slug").
		Where("node_id = ?", firstNodeID).
		Take(&persistedNode).Error; err != nil {
		t.Fatalf("query persisted reader_slug failed: %v", err)
	}
	if persistedNode.ReaderSlug != nil && strings.TrimSpace(*persistedNode.ReaderSlug) != "" {
		t.Fatalf("expected reader_slug cleared, got %q", strings.TrimSpace(*persistedNode.ReaderSlug))
	}

	dottedReq := httptest.NewRequest(
		http.MethodPatch,
		"/api/docs/"+firstDocID+"/identifier",
		bytes.NewReader([]byte(`{"identifier":"read-1.md"}`)),
	)
	dottedReq.Header.Set("Authorization", "Bearer "+ownerToken)
	dottedReq.Header.Set("Content-Type", "application/json")
	dottedRec := serve(dottedReq)
	if dottedRec.Code != http.StatusOK {
		t.Fatalf("expected dotted identifier status 200, got %d body=%s", dottedRec.Code, dottedRec.Body.String())
	}
	dottedPayload := decodeJSONResultData[struct {
		Identifier *string `json:"identifier"`
	}](t, dottedRec.Body.Bytes())
	if dottedPayload.Identifier == nil || *dottedPayload.Identifier != "read-1.md" {
		t.Fatalf("expected dotted identifier read-1.md, got %#v", dottedPayload.Identifier)
	}
}

func TestRouter_ReaderPage_DocumentIDRedirectsToSlugCanonical(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, _ := registerAccessUser(t, serve, "owner-reader-slug@example.com")
	spaceID := "01h1readerslugspace0000000001"
	nodeID := "01h1readerslugnode00000000001"
	docID := "01h1readerslugdoc000000000001"
	readerSlug := "quick-start"

	seedSpaceAndDocumentForAccess(t, database, ownerUserID, spaceID, nodeID, docID, "public", "public")
	if err := database.ORM.Table("nodes").Where("node_id = ?", nodeID).Update("reader_slug", readerSlug).Error; err != nil {
		t.Fatalf("update reader slug failed: %v", err)
	}

	legacyURLReq := httptest.NewRequest(http.MethodGet, "/r/"+spaceID+"/"+docID, nil)
	legacyURLRec := serve(legacyURLReq)
	if legacyURLRec.Code != http.StatusSeeOther {
		t.Fatalf("expected legacy doc id redirect status 303, got %d body=%s", legacyURLRec.Code, legacyURLRec.Body.String())
	}
	expectedLocation := "/r/" + spaceID + "/" + readerSlug
	if location := legacyURLRec.Header().Get("Location"); location != expectedLocation {
		t.Fatalf("expected redirect location %q, got %q", expectedLocation, location)
	}

	slugURLReq := httptest.NewRequest(http.MethodGet, "/r/"+spaceID+"/"+readerSlug, nil)
	slugURLRec := serve(slugURLReq)
	if slugURLRec.Code != http.StatusOK {
		t.Fatalf("expected slug url status 200, got %d body=%s", slugURLRec.Code, slugURLRec.Body.String())
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
	if rec.Code != http.StatusOK {
		t.Fatalf("register failed, status=%d body=%s", rec.Code, rec.Body.String())
	}

	payload := decodeJSONResultData[struct {
		User struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"user"`
		Token string `json:"token"`
	}](t, rec.Body.Bytes())
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

func seedSpaceForWorkspaceCreateNode(
	t *testing.T,
	database *storage.Database,
	ownerUserID string,
	spaceID string,
	spaceVisibility string,
) {
	t.Helper()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := database.ORM.Table("spaces").Create(map[string]any{
		"space_id":      spaceID,
		"name":          "Workspace Create Node Space",
		"owner_user_id": ownerUserID,
		"visibility":    spaceVisibility,
		"status":        "active",
		"created_at":    now,
		"updated_at":    now,
	}).Error; err != nil {
		t.Fatalf("insert space for create node test failed: %v", err)
	}
}

func seedSpaceMemberForAccess(
	t *testing.T,
	database *storage.Database,
	spaceID string,
	userID string,
	role string,
) {
	t.Helper()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := database.ORM.Table("space_members").Create(map[string]any{
		"space_id":   spaceID,
		"user_id":    userID,
		"role":       role,
		"created_at": now,
		"updated_at": now,
	}).Error; err != nil {
		t.Fatalf("insert space member failed: %v", err)
	}
}

func grantPlatformAdminRoleForAccess(
	t *testing.T,
	database *storage.Database,
	userID string,
) {
	t.Helper()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := database.ORM.Table("user_admin_roles").Create(map[string]any{
		"user_id":    userID,
		"role":       "platform_admin",
		"created_at": now,
		"updated_at": now,
	}).Error; err != nil {
		t.Fatalf("grant platform admin role failed: %v", err)
	}
}
