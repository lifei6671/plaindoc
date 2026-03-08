package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRouter_DeleteNode_RemovesSubtreeDocumentsAndDocumentResources(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "delete-node-owner@example.com")
	spaceID := "01kdeletenodespace0000000001"
	rootFolderNodeID := "01kdeletenoderootfolder00001"
	childFolderNodeID := "01kdeletenodechildfolder0001"
	childDocNodeID := "01kdeletenodechilddocnode001"
	childDocID := "01kdeletenodechilddoc000001"
	grandDocNodeID := "01kdeletenodegranddocnode01"
	grandDocID := "01kdeletenodegranddoc000001"
	now := time.Now().UTC().Format(time.RFC3339Nano)

	if err := database.ORM.Table("spaces").Create(map[string]any{
		"space_id":      spaceID,
		"name":          "Delete Node Space",
		"owner_user_id": ownerUserID,
		"visibility":    "member",
		"status":        "active",
		"created_at":    now,
		"updated_at":    now,
	}).Error; err != nil {
		t.Fatalf("insert space failed: %v", err)
	}
	for _, node := range []map[string]any{
		{
			"node_id":        rootFolderNodeID,
			"space_id":       spaceID,
			"parent_node_id": nil,
			"type":           "folder",
			"title":          "Root Folder",
			"sort":           1,
			"created_at":     now,
			"updated_at":     now,
		},
		{
			"node_id":        childFolderNodeID,
			"space_id":       spaceID,
			"parent_node_id": rootFolderNodeID,
			"type":           "folder",
			"title":          "Child Folder",
			"sort":           1,
			"created_at":     now,
			"updated_at":     now,
		},
		{
			"node_id":        childDocNodeID,
			"space_id":       spaceID,
			"parent_node_id": rootFolderNodeID,
			"type":           "doc",
			"title":          "Child Doc",
			"sort":           2,
			"created_at":     now,
			"updated_at":     now,
		},
		{
			"node_id":        grandDocNodeID,
			"space_id":       spaceID,
			"parent_node_id": childFolderNodeID,
			"type":           "doc",
			"title":          "Grand Doc",
			"sort":           1,
			"created_at":     now,
			"updated_at":     now,
		},
	} {
		if err := database.ORM.Table("nodes").Create(node).Error; err != nil {
			t.Fatalf("insert node failed: %v", err)
		}
	}
	for _, document := range []map[string]any{
		{
			"document_id":        childDocID,
			"node_id":            childDocNodeID,
			"theme_id":           "default",
			"visibility":         "member",
			"status":             "active",
			"title":              "Child Doc",
			"content_md":         "# Child",
			"version":            1,
			"updated_by_user_id": ownerUserID,
			"created_at":         now,
			"updated_at":         now,
		},
		{
			"document_id":        grandDocID,
			"node_id":            grandDocNodeID,
			"theme_id":           "default",
			"visibility":         "member",
			"status":             "active",
			"title":              "Grand Doc",
			"content_md":         "# Grand",
			"version":            1,
			"updated_by_user_id": ownerUserID,
			"created_at":         now,
			"updated_at":         now,
		},
	} {
		if err := database.ORM.Table("documents").Create(document).Error; err != nil {
			t.Fatalf("insert document failed: %v", err)
		}
	}

	childAttachmentBlobID := "01kdeletenodechildblob00001"
	grandImageBlobID := "01kdeletenodegrandblob00001"
	for _, blob := range []map[string]any{
		{
			"blob_id":           childAttachmentBlobID,
			"storage_provider":  "local",
			"object_key":        "../invalid/delete-node/attachment.pdf",
			"object_url":        "/uploads/../invalid/delete-node/attachment.pdf",
			"mime_type":         "application/pdf",
			"size_bytes":        256,
			"content_hash_algo": "sha256",
			"content_hash":      "delete-node-attachment-hash",
			"created_at":        now,
			"updated_at":        now,
		},
		{
			"blob_id":           grandImageBlobID,
			"storage_provider":  "local",
			"object_key":        "../invalid/delete-node/image.png",
			"object_url":        "/uploads/../invalid/delete-node/image.png",
			"mime_type":         "image/png",
			"size_bytes":        128,
			"content_hash_algo": "sha256",
			"content_hash":      "delete-node-image-hash",
			"created_at":        now,
			"updated_at":        now,
		},
	} {
		if err := database.ORM.Table("file_blobs").Create(blob).Error; err != nil {
			t.Fatalf("insert blob failed: %v", err)
		}
	}
	if err := database.ORM.Table("document_attachments").Create(map[string]any{
		"attachment_id":      "01kdeletenodeattachment0001",
		"blob_id":            childAttachmentBlobID,
		"document_id":        childDocID,
		"space_id":           spaceID,
		"storage_provider":   "local",
		"file_name":          "manual.pdf",
		"object_key":         "../invalid/delete-node/attachment.pdf",
		"object_url":         "/uploads/../invalid/delete-node/attachment.pdf",
		"mime_type":          "application/pdf",
		"size_bytes":         256,
		"content_hash_algo":  "sha256",
		"content_hash":       "delete-node-attachment-hash",
		"preview_kind":       "pdf",
		"status":             "active",
		"created_by_user_id": ownerUserID,
		"created_at":         now,
		"updated_at":         now,
	}).Error; err != nil {
		t.Fatalf("insert attachment failed: %v", err)
	}
	if err := database.ORM.Table("document_image_assets").Create(map[string]any{
		"image_asset_id":     "01kdeletenodeimageasset0001",
		"document_id":        grandDocID,
		"space_id":           spaceID,
		"blob_id":            grandImageBlobID,
		"storage_provider":   "local",
		"object_key":         "../invalid/delete-node/image.png",
		"object_url":         "/uploads/../invalid/delete-node/image.png",
		"status":             "active",
		"last_referenced_at": now,
		"created_at":         now,
		"updated_at":         now,
	}).Error; err != nil {
		t.Fatalf("insert image asset failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/nodes/"+rootFolderNodeID, nil)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	rec := serve(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected delete node status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	assertTableCount := func(table string, expected int64, query string, args ...any) {
		t.Helper()
		var count int64
		if err := database.ORM.Table(table).Where(query, args...).Count(&count).Error; err != nil {
			t.Fatalf("count %s failed: %v", table, err)
		}
		if count != expected {
			t.Fatalf("expected %s count %d, got %d", table, expected, count)
		}
	}

	assertTableCount("nodes", 0, "node_id IN ?", []string{
		rootFolderNodeID,
		childFolderNodeID,
		childDocNodeID,
		grandDocNodeID,
	})
	assertTableCount("documents", 0, "document_id IN ?", []string{childDocID, grandDocID})
	assertTableCount("document_attachments", 0, "document_id IN ?", []string{childDocID, grandDocID})
	assertTableCount("document_image_assets", 0, "document_id IN ?", []string{childDocID, grandDocID})
}
