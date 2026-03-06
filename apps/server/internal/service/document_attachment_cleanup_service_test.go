package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
)

func TestDocumentAttachmentCleanupService_CleanupDeletedDocumentAttachments(t *testing.T) {
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-document-attachment-cleanup?mode=memory&cache=shared",
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
	userID := "01kattachmentcleanupuser0000001"
	spaceID := "01kattachmentcleanupspace000001"
	deletedNodeID := "01kattachmentcleanupnode0000001"
	activeNodeID := "01kattachmentcleanupnode0000002"
	activeOfficeNodeID := "01kattachmentcleanupnode0000003"
	deletedDocID := "01kattachmentcleanupdoc00000001"
	activeDocID := "01kattachmentcleanupdoc00000002"
	activeOfficeDocID := "01kattachmentcleanupdoc00000003"

	if err := database.ORM.WithContext(ctx).Table("users").Create(map[string]any{
		"user_id":       userID,
		"email":         "attachment-cleanup@example.com",
		"password_hash": "hashed-password",
		"name":          "Attachment Cleanup",
		"created_at":    now,
		"updated_at":    now,
	}).Error; err != nil {
		t.Fatalf("seed user failed: %v", err)
	}
	if err := database.ORM.WithContext(ctx).Table("spaces").Create(map[string]any{
		"space_id":      spaceID,
		"name":          "Attachment Cleanup Space",
		"owner_user_id": userID,
		"visibility":    "member",
		"status":        "active",
		"created_at":    now,
		"updated_at":    now,
	}).Error; err != nil {
		t.Fatalf("seed space failed: %v", err)
	}
	if err := database.ORM.WithContext(ctx).Table("nodes").Create([]map[string]any{
		{
			"node_id":            deletedNodeID,
			"space_id":           spaceID,
			"parent_node_id":     nil,
			"type":               "doc",
			"title":              "Deleted Node",
			"sort":               1,
			"created_by_user_id": userID,
			"updated_by_user_id": userID,
			"created_at":         now,
			"updated_at":         now,
		},
		{
			"node_id":            activeNodeID,
			"space_id":           spaceID,
			"parent_node_id":     nil,
			"type":               "doc",
			"title":              "Active Node",
			"sort":               2,
			"created_by_user_id": userID,
			"updated_by_user_id": userID,
			"created_at":         now,
			"updated_at":         now,
		},
		{
			"node_id":            activeOfficeNodeID,
			"space_id":           spaceID,
			"parent_node_id":     nil,
			"type":               "doc",
			"title":              "Active Office Node",
			"sort":               3,
			"created_by_user_id": userID,
			"updated_by_user_id": userID,
			"created_at":         now,
			"updated_at":         now,
		},
	}).Error; err != nil {
		t.Fatalf("seed nodes failed: %v", err)
	}
	officeBlobID := "01kattachmentcleanupblob000004"
	if err := database.ORM.WithContext(ctx).Table("documents").Create([]map[string]any{
		{
			"document_id":        deletedDocID,
			"node_id":            deletedNodeID,
			"theme_id":           "default",
			"visibility":         "member",
			"title":              "Deleted Doc",
			"content_md":         "",
			"version":            1,
			"status":             "deleted",
			"deleted_at":         now.Add(-2 * time.Hour),
			"updated_by_user_id": userID,
			"created_at":         now.Add(-3 * time.Hour),
			"updated_at":         now.Add(-2 * time.Hour),
		},
		{
			"document_id":        activeDocID,
			"node_id":            activeNodeID,
			"theme_id":           "default",
			"visibility":         "member",
			"title":              "Active Doc",
			"content_md":         "",
			"version":            1,
			"status":             "active",
			"deleted_at":         nil,
			"updated_by_user_id": userID,
			"created_at":         now.Add(-2 * time.Hour),
			"updated_at":         now.Add(-1 * time.Hour),
		},
	}).Error; err != nil {
		t.Fatalf("seed documents failed: %v", err)
	}

	deletedBlobID := "01kattachmentcleanupblob000001"
	activeBlobID := "01kattachmentcleanupblob000002"
	orphanBlobID := "01kattachmentcleanupblob000003"
	officeObjectKey := "images/cleanup/active-office.docx"
	deletedObjectKey := "images/cleanup/deleted-file.txt"
	activeObjectKey := "images/cleanup/active-file.txt"
	orphanObjectKey := "images/cleanup/orphan-file.txt"

	deletedBlobPath := createAttachmentCleanupLocalFile(t, deletedObjectKey, "deleted")
	activeBlobPath := createAttachmentCleanupLocalFile(t, activeObjectKey, "active")
	orphanBlobPath := createAttachmentCleanupLocalFile(t, orphanObjectKey, "orphan")
	officeBlobPath := createAttachmentCleanupLocalFile(t, officeObjectKey, "office")

	if err := database.ORM.WithContext(ctx).Table("file_blobs").Create([]map[string]any{
		{
			"blob_id":           deletedBlobID,
			"storage_provider":  "local",
			"object_key":        deletedObjectKey,
			"object_url":        "/uploads/" + deletedObjectKey,
			"mime_type":         "text/plain",
			"size_bytes":        7,
			"content_hash_algo": "sha256",
			"content_hash":      "hash-deleted",
			"deleted_at":        nil,
			"created_at":        now.Add(-2 * time.Hour),
			"updated_at":        now.Add(-2 * time.Hour),
		},
		{
			"blob_id":           activeBlobID,
			"storage_provider":  "local",
			"object_key":        activeObjectKey,
			"object_url":        "/uploads/" + activeObjectKey,
			"mime_type":         "text/plain",
			"size_bytes":        6,
			"content_hash_algo": "sha256",
			"content_hash":      "hash-active",
			"deleted_at":        nil,
			"created_at":        now.Add(-2 * time.Hour),
			"updated_at":        now.Add(-2 * time.Hour),
		},
		{
			"blob_id":           orphanBlobID,
			"storage_provider":  "local",
			"object_key":        orphanObjectKey,
			"object_url":        "/uploads/" + orphanObjectKey,
			"mime_type":         "text/plain",
			"size_bytes":        6,
			"content_hash_algo": "sha256",
			"content_hash":      "hash-orphan",
			"deleted_at":        nil,
			"created_at":        now.Add(-2 * time.Hour),
			"updated_at":        now.Add(-2 * time.Hour),
		},
		{
			"blob_id":           officeBlobID,
			"storage_provider":  "local",
			"object_key":        officeObjectKey,
			"object_url":        "/uploads/" + officeObjectKey,
			"mime_type":         "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			"size_bytes":        6,
			"content_hash_algo": "sha256",
			"content_hash":      "hash-office",
			"deleted_at":        nil,
			"created_at":        now.Add(-2 * time.Hour),
			"updated_at":        now.Add(-2 * time.Hour),
		},
	}).Error; err != nil {
		t.Fatalf("seed file blobs failed: %v", err)
	}
	if err := database.ORM.WithContext(ctx).Table("documents").Create(map[string]any{
		"document_id":        activeOfficeDocID,
		"node_id":            activeOfficeNodeID,
		"theme_id":           "default",
		"visibility":         "member",
		"title":              "Active Office Doc",
		"format":             "docx",
		"content_md":         "",
		"version":            1,
		"source_blob_id":     officeBlobID,
		"source_file_name":   "active-office.docx",
		"source_mime_type":   "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"content_version":    1,
		"status":             "active",
		"deleted_at":         nil,
		"updated_by_user_id": userID,
		"created_at":         now.Add(-2 * time.Hour),
		"updated_at":         now.Add(-45 * time.Minute),
	}).Error; err != nil {
		t.Fatalf("seed office document failed: %v", err)
	}

	if err := database.ORM.WithContext(ctx).Table("document_attachments").Create([]map[string]any{
		{
			"attachment_id":      "01kattachmentcleanupatt000001",
			"blob_id":            deletedBlobID,
			"document_id":        deletedDocID,
			"space_id":           spaceID,
			"storage_provider":   "local",
			"file_name":          "deleted-file.txt",
			"object_key":         deletedObjectKey,
			"object_url":         "/uploads/" + deletedObjectKey,
			"mime_type":          "text/plain",
			"size_bytes":         7,
			"content_hash_algo":  "sha256",
			"content_hash":       "hash-deleted",
			"preview_kind":       "text",
			"status":             "active",
			"deleted_at":         nil,
			"created_by_user_id": userID,
			"created_at":         now.Add(-90 * time.Minute),
			"updated_at":         now.Add(-90 * time.Minute),
		},
		{
			"attachment_id":      "01kattachmentcleanupatt000002",
			"blob_id":            activeBlobID,
			"document_id":        activeDocID,
			"space_id":           spaceID,
			"storage_provider":   "local",
			"file_name":          "active-file.txt",
			"object_key":         activeObjectKey,
			"object_url":         "/uploads/" + activeObjectKey,
			"mime_type":          "text/plain",
			"size_bytes":         6,
			"content_hash_algo":  "sha256",
			"content_hash":       "hash-active",
			"preview_kind":       "text",
			"status":             "active",
			"deleted_at":         nil,
			"created_by_user_id": userID,
			"created_at":         now.Add(-80 * time.Minute),
			"updated_at":         now.Add(-80 * time.Minute),
		},
	}).Error; err != nil {
		t.Fatalf("seed attachments failed: %v", err)
	}
	if err := database.ORM.WithContext(ctx).Table("document_file_revisions").Create(map[string]any{
		"document_file_revision_id": "01kattachmentcleanupfilerev00001",
		"document_id":               activeOfficeDocID,
		"blob_id":                   officeBlobID,
		"file_name":                 "active-office.docx",
		"mime_type":                 "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"version":                   1,
		"base_version":              0,
		"editor_user_id":            userID,
		"source":                    "remote",
		"created_at":                now.Add(-40 * time.Minute),
	}).Error; err != nil {
		t.Fatalf("seed office file revisions failed: %v", err)
	}

	cleanupService := NewDocumentAttachmentCleanupService(
		database.ORM,
		repository.NewGormDocumentAttachmentRepository(database.ORM),
		nil,
	)
	result, err := cleanupService.CleanupDeletedDocumentAttachments(ctx, 100)
	if err != nil {
		t.Fatalf("cleanup deleted document attachments failed: %v", err)
	}
	if result.DeletedAttachments != 1 {
		t.Fatalf("expected deleted attachments 1, got %d", result.DeletedAttachments)
	}
	if result.DeletedBlobs != 2 {
		t.Fatalf("expected deleted blobs 2, got %d", result.DeletedBlobs)
	}

	assertAttachmentCleanupCount(t, database, "document_attachments", "attachment_id", "01kattachmentcleanupatt000001", 0)
	assertAttachmentCleanupCount(t, database, "document_attachments", "attachment_id", "01kattachmentcleanupatt000002", 1)
	assertAttachmentCleanupCount(t, database, "file_blobs", "blob_id", deletedBlobID, 0)
	assertAttachmentCleanupCount(t, database, "file_blobs", "blob_id", orphanBlobID, 0)
	assertAttachmentCleanupCount(t, database, "file_blobs", "blob_id", activeBlobID, 1)
	assertAttachmentCleanupCount(t, database, "file_blobs", "blob_id", officeBlobID, 1)

	if _, err := os.Stat(deletedBlobPath); !isNotExistErr(err) {
		t.Fatalf("expected deleted blob file removed, stat err=%v", err)
	}
	if _, err := os.Stat(orphanBlobPath); !isNotExistErr(err) {
		t.Fatalf("expected orphan blob file removed, stat err=%v", err)
	}
	if _, err := os.Stat(activeBlobPath); err != nil {
		t.Fatalf("expected active blob file remains, stat err=%v", err)
	}
	if _, err := os.Stat(officeBlobPath); err != nil {
		t.Fatalf("expected office blob file remains, stat err=%v", err)
	}
}

func createAttachmentCleanupLocalFile(t *testing.T, objectKey string, content string) string {
	t.Helper()

	normalizedObjectKey := strings.TrimSpace(strings.TrimPrefix(objectKey, "/"))
	targetPath := filepath.Join("uploads", filepath.FromSlash(normalizedObjectKey))
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatalf("mkdir local file path failed: %v", err)
	}
	if err := os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write local file failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Remove(targetPath)
	})
	return targetPath
}

func assertAttachmentCleanupCount(
	t *testing.T,
	database *storage.Database,
	tableName string,
	columnName string,
	target string,
	expected int64,
) {
	t.Helper()

	var count int64
	if err := database.ORM.Table(tableName).Where(columnName+" = ?", target).Count(&count).Error; err != nil {
		t.Fatalf("count %s by %s failed: %v", tableName, columnName, err)
	}
	if count != expected {
		t.Fatalf("unexpected count for %s.%s=%s: got=%d want=%d", tableName, columnName, target, count, expected)
	}
}

func isNotExistErr(err error) bool {
	return os.IsNotExist(err)
}
