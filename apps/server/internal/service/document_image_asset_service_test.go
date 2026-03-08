package service

import (
	"context"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage"
)

func TestDocumentImageAssetService_SyncDocumentImageAssets_SQLiteTextTimestampCompatibility(t *testing.T) {
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-document-image-asset-sync-time-scan?mode=memory&cache=shared",
	})
	if err != nil {
		t.Fatalf("open database failed: %v", err)
	}
	defer func() {
		_ = database.Close()
	}()

	ctx := context.Background()
	if err := storage.MigrateUp(ctx, database.ORM, storage.DriverSQLite); err != nil {
		t.Fatalf("migrate up failed: %v", err)
	}

	now := time.Now().UTC()
	userID := "01kdocumentimageassetuser00000001"
	spaceID := "01kdocumentimageassetspace000001"
	nodeID := "01kdocumentimageassetnode0000001"
	documentID := "01kdocumentimageassetdoc00000001"
	blobID := "01kdocumentimageassetblob000001"
	objectKey := "images/space-a/doc-a/sync-time.png"

	if err := database.ORM.WithContext(ctx).Table("users").Create(map[string]any{
		"user_id":       userID,
		"email":         "doc-image-sync-time@example.com",
		"password_hash": "hashed-password",
		"name":          "Doc Image Sync Time",
		"created_at":    now,
		"updated_at":    now,
	}).Error; err != nil {
		t.Fatalf("seed user failed: %v", err)
	}

	if err := database.ORM.WithContext(ctx).Table("spaces").Create(map[string]any{
		"space_id":      spaceID,
		"name":          "Space For Image Sync",
		"owner_user_id": userID,
		"visibility":    "member",
		"description":   "",
		"cover_url":     "",
		"created_at":    now,
		"updated_at":    now,
		"status":        "active",
	}).Error; err != nil {
		t.Fatalf("seed space failed: %v", err)
	}

	if err := database.ORM.WithContext(ctx).Table("nodes").Create(map[string]any{
		"node_id":            nodeID,
		"space_id":           spaceID,
		"parent_node_id":     nil,
		"type":               "doc",
		"title":              "Document Node",
		"sort":               1,
		"created_by_user_id": userID,
		"updated_by_user_id": userID,
		"created_at":         now,
		"updated_at":         now,
	}).Error; err != nil {
		t.Fatalf("seed node failed: %v", err)
	}

	if err := database.ORM.WithContext(ctx).Table("documents").Create(map[string]any{
		"document_id":        documentID,
		"node_id":            nodeID,
		"theme_id":           "default",
		"title":              "Document For Image Sync",
		"content_md":         "",
		"version":            1,
		"updated_by_user_id": userID,
		"created_by_user_id": userID,
		"created_at":         now,
		"updated_at":         now,
		"status":             "active",
	}).Error; err != nil {
		t.Fatalf("seed document failed: %v", err)
	}
	if err := database.ORM.WithContext(ctx).Table("file_blobs").Create(map[string]any{
		"blob_id":           blobID,
		"storage_provider":  "local",
		"object_key":        objectKey,
		"object_url":        "/uploads/" + objectKey,
		"mime_type":         "image/png",
		"size_bytes":        7,
		"content_hash_algo": "sha256",
		"content_hash":      "hash-sync-time-image",
		"created_at":        now,
		"updated_at":        now,
	}).Error; err != nil {
		t.Fatalf("seed file blob failed: %v", err)
	}

	legacyTimestamp := now.Add(-2 * time.Hour).Format("2006-01-02 15:04:05")
	if err := database.ORM.WithContext(ctx).Table("document_image_assets").Create(map[string]any{
		"image_asset_id":     "01kdocumentimageassetexisting0001",
		"document_id":        documentID,
		"space_id":           spaceID,
		"storage_provider":   "local",
		"object_key":         objectKey,
		"object_url":         "/uploads/" + objectKey,
		"status":             "active",
		"pending_cleanup_at": nil,
		"deleted_at":         nil,
		"last_referenced_at": legacyTimestamp,
		"created_at":         legacyTimestamp,
		"updated_at":         legacyTimestamp,
	}).Error; err != nil {
		t.Fatalf("seed document image asset failed: %v", err)
	}

	service := NewDocumentImageAssetService(database.ORM, nil)
	if err := service.SyncDocumentImageAssets(ctx, SyncDocumentImageAssetsInput{
		DocumentID:   documentID,
		SpaceID:      spaceID,
		ContentMD:    "![sync-time](/uploads/" + objectKey + ")",
		ReferencedAt: now,
	}); err != nil {
		t.Fatalf("sync document image assets failed: %v", err)
	}

	var rowCount int64
	if err := database.ORM.WithContext(ctx).
		Table("document_image_assets").
		Where("document_id = ? AND storage_provider = ? AND object_key = ?", documentID, "local", objectKey).
		Count(&rowCount).Error; err != nil {
		t.Fatalf("count document image assets failed: %v", err)
	}
	if rowCount != 1 {
		t.Fatalf("expected one document image asset row, got %d", rowCount)
	}

	type statusRow struct {
		Status string  `gorm:"column:status"`
		BlobID *string `gorm:"column:blob_id"`
	}
	var status statusRow
	if err := database.ORM.WithContext(ctx).
		Table("document_image_assets").
		Select("status", "blob_id").
		Where("document_id = ? AND storage_provider = ? AND object_key = ?", documentID, "local", objectKey).
		Take(&status).Error; err != nil {
		t.Fatalf("query document image asset status failed: %v", err)
	}
	if status.Status != "active" {
		t.Fatalf("expected document image asset status active, got %q", status.Status)
	}
	if status.BlobID == nil || *status.BlobID != blobID {
		t.Fatalf("expected document image asset blob_id %q, got %+v", blobID, status.BlobID)
	}
}
