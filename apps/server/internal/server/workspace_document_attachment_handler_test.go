package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
)

func TestRouter_DeleteDocumentAttachment_PhysicalDeleteFailureKeepsBlobForCompensation(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "attachment-delete-owner@example.com")
	spaceID := "01kattachmentdeletespace0000001"
	nodeID := "01kattachmentdeletenode00000001"
	documentID := "01kattachmentdeletedoc00000001"
	attachmentID := "01kattachmentdeleteatt0000001"
	blobID := "01kattachmentdeleteblob0000001"
	now := time.Now().UTC()

	seedSpaceAndDocumentForAccess(t, database, ownerUserID, spaceID, nodeID, documentID, "member", "member")

	if err := database.ORM.Table("file_blobs").Create(map[string]any{
		"blob_id":           blobID,
		"storage_provider":  "local",
		"object_key":        "../invalid/path/for-delete",
		"object_url":        "/uploads/../invalid/path/for-delete",
		"mime_type":         "application/pdf",
		"size_bytes":        1024,
		"content_hash_algo": "sha256",
		"content_hash":      "hash-delete-failure",
		"created_at":        now,
		"updated_at":        now,
	}).Error; err != nil {
		t.Fatalf("seed file blob failed: %v", err)
	}
	if err := database.ORM.Table("document_attachments").Create(map[string]any{
		"attachment_id":      attachmentID,
		"blob_id":            blobID,
		"document_id":        documentID,
		"space_id":           spaceID,
		"storage_provider":   "local",
		"file_name":          "manual.pdf",
		"object_key":         "../invalid/path/for-delete",
		"object_url":         "/uploads/../invalid/path/for-delete",
		"mime_type":          "application/pdf",
		"size_bytes":         1024,
		"content_hash_algo":  "sha256",
		"content_hash":       "hash-delete-failure",
		"preview_kind":       "pdf",
		"status":             "active",
		"created_by_user_id": ownerUserID,
		"created_at":         now,
		"updated_at":         now,
	}).Error; err != nil {
		t.Fatalf("seed document attachment failed: %v", err)
	}

	deleteReq := httptest.NewRequest(
		http.MethodDelete,
		"/api/docs/"+documentID+"/attachments/"+attachmentID+"?physicalDelete=true",
		nil,
	)
	deleteReq.Header.Set("Authorization", "Bearer "+ownerToken)
	deleteRec := serve(deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected delete attachment status 200, got %d body=%s", deleteRec.Code, deleteRec.Body.String())
	}
	if decodeJSONResultCode(t, deleteRec.Body.Bytes()) != response.ResolveErrorCode(response.CodeInternalError) {
		t.Fatalf(
			"expected internal error code %d, got %d body=%s",
			response.ResolveErrorCode(response.CodeInternalError),
			decodeJSONResultCode(t, deleteRec.Body.Bytes()),
			deleteRec.Body.String(),
		)
	}

	var attachmentCount int64
	if err := database.ORM.Table("document_attachments").Where("attachment_id = ?", attachmentID).Count(&attachmentCount).Error; err != nil {
		t.Fatalf("count attachment failed: %v", err)
	}
	if attachmentCount != 0 {
		t.Fatalf("expected attachment row hard-deleted, got count=%d", attachmentCount)
	}

	var blobCount int64
	if err := database.ORM.Table("file_blobs").Where("blob_id = ?", blobID).Count(&blobCount).Error; err != nil {
		t.Fatalf("count blob failed: %v", err)
	}
	if blobCount != 1 {
		t.Fatalf("expected blob row kept for compensation, got count=%d", blobCount)
	}
}

func TestRouter_CreateDocumentAttachmentAccessLink_InvalidPurpose(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "attachment-access-link-owner@example.com")
	spaceID := "01kattachmentaccessspace000001"
	nodeID := "01kattachmentaccessnode0000001"
	documentID := "01kattachmentaccessdoc00000001"
	attachmentID := "01kattachmentaccessatt0000001"
	blobID := "01kattachmentaccessblob0000001"
	now := time.Now().UTC()

	seedSpaceAndDocumentForAccess(t, database, ownerUserID, spaceID, nodeID, documentID, "member", "member")

	if err := database.ORM.Table("file_blobs").Create(map[string]any{
		"blob_id":           blobID,
		"storage_provider":  "local",
		"object_key":        "images/test/access/manual.pdf",
		"object_url":        "/uploads/images/test/access/manual.pdf",
		"mime_type":         "application/pdf",
		"size_bytes":        2048,
		"content_hash_algo": "sha256",
		"content_hash":      "hash-access-link",
		"created_at":        now,
		"updated_at":        now,
	}).Error; err != nil {
		t.Fatalf("seed file blob failed: %v", err)
	}
	if err := database.ORM.Table("document_attachments").Create(map[string]any{
		"attachment_id":      attachmentID,
		"blob_id":            blobID,
		"document_id":        documentID,
		"space_id":           spaceID,
		"storage_provider":   "local",
		"file_name":          "manual.pdf",
		"object_key":         "images/test/access/manual.pdf",
		"object_url":         "/uploads/images/test/access/manual.pdf",
		"mime_type":          "application/pdf",
		"size_bytes":         2048,
		"content_hash_algo":  "sha256",
		"content_hash":       "hash-access-link",
		"preview_kind":       "pdf",
		"status":             "active",
		"created_by_user_id": ownerUserID,
		"created_at":         now,
		"updated_at":         now,
	}).Error; err != nil {
		t.Fatalf("seed document attachment failed: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/docs/"+documentID+"/attachments/"+attachmentID+"/access-link?purpose=invalid-purpose",
		nil,
	)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	rec := serve(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected create access link status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if decodeJSONResultCode(t, rec.Body.Bytes()) != response.ResolveErrorCode(response.CodeInvalidRequest) {
		t.Fatalf(
			"expected invalid request code %d, got %d body=%s",
			response.ResolveErrorCode(response.CodeInvalidRequest),
			decodeJSONResultCode(t, rec.Body.Bytes()),
			rec.Body.String(),
		)
	}
}
