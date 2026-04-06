package repository

import (
	"context"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
)

func TestGormDocumentAttachmentRepository_ListForAdmin_MapsAttachmentFields(t *testing.T) {
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-document-attachment-repository-list?mode=memory&cache=shared",
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

	now := time.Now().UTC().Round(time.Second)
	ownerUserID := "01kowneratt0000000000000000"
	creatorUserID := "01kcreator0000000000000000"
	spaceID := "01kspaceatt0000000000000000"
	nodeID := "01knodeatt00000000000000000"
	documentID := "01kdocatt000000000000000000"
	attachmentID := "01kattach00000000000000000"
	blobID := "01kblob00000000000000000000"

	for _, user := range []*models.User{
		{
			UserID:       ownerUserID,
			Email:        "owner@example.com",
			PasswordHash: "hashed",
			Name:         "Owner",
			Status:       models.EntityStatusActive,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			UserID:       creatorUserID,
			Email:        "creator@example.com",
			PasswordHash: "hashed",
			Name:         "Creator",
			Status:       models.EntityStatusActive,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	} {
		if err := database.ORM.WithContext(ctx).Create(user).Error; err != nil {
			t.Fatalf("seed user failed: %v", err)
		}
	}

	if err := database.ORM.WithContext(ctx).Create(&models.Space{
		SpaceID:     spaceID,
		Name:        "测试空间",
		Description: "desc",
		CategoryID:  "01jmf4v2x7m7f1m6qv5kh0t2mn",
		Category:    "未分类",
		OwnerUserID: ownerUserID,
		Visibility:  models.VisibilityMember,
		Status:      models.EntityStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}).Error; err != nil {
		t.Fatalf("seed space failed: %v", err)
	}

	readerSlug := "attachment-doc"
	if err := database.ORM.WithContext(ctx).Create(&models.Node{
		NodeID:          nodeID,
		SpaceID:         spaceID,
		ReaderSlug:      &readerSlug,
		Type:            models.NodeTypeDoc,
		Title:           "节点标题",
		Sort:            1,
		CreatedByUserID: &ownerUserID,
		UpdatedByUserID: &ownerUserID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}).Error; err != nil {
		t.Fatalf("seed node failed: %v", err)
	}

	if err := database.ORM.WithContext(ctx).Create(&models.Document{
		DocumentID:      documentID,
		NodeID:          nodeID,
		ThemeID:         "default",
		Visibility:      models.VisibilityMember,
		Status:          models.EntityStatusActive,
		Title:           "附件文档",
		ContentMD:       "content",
		Version:         1,
		CreatedByUserID: &creatorUserID,
		UpdatedByUserID: &creatorUserID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}).Error; err != nil {
		t.Fatalf("seed document failed: %v", err)
	}

	if err := database.ORM.WithContext(ctx).Create(&models.DocumentAttachment{
		AttachmentID:    attachmentID,
		BlobID:          blobID,
		DocumentID:      documentID,
		SpaceID:         spaceID,
		StorageProvider: "local",
		FileName:        "example.pdf",
		ObjectKey:       "attachments/example.pdf",
		ObjectURL:       "/uploads/example.pdf",
		MimeType:        "application/pdf",
		SizeBytes:       1234,
		ContentHashAlgo: "sha256",
		ContentHash:     "hash",
		PreviewKind:     "pdf",
		Status:          models.EntityStatusActive,
		CreatedByUserID: &creatorUserID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}).Error; err != nil {
		t.Fatalf("seed attachment failed: %v", err)
	}

	repo := NewGormDocumentAttachmentRepository(database.ORM)
	records, total, err := repo.ListForAdmin(ctx, ListAdminDocumentAttachmentsParams{
		RestrictToScopes: false,
		Limit:            20,
		Offset:           0,
	})
	if err != nil {
		t.Fatalf("list attachments failed: %v", err)
	}

	if total != 1 {
		t.Fatalf("unexpected total: got=%d want=1", total)
	}
	if len(records) != 1 {
		t.Fatalf("unexpected records length: got=%d want=1", len(records))
	}

	record := records[0]
	if record.Attachment.AttachmentID != attachmentID {
		t.Fatalf("unexpected attachment_id: got=%q want=%q", record.Attachment.AttachmentID, attachmentID)
	}
	if record.Attachment.DocumentID != documentID {
		t.Fatalf("unexpected document_id: got=%q want=%q", record.Attachment.DocumentID, documentID)
	}
	if record.Attachment.SpaceID != spaceID {
		t.Fatalf("unexpected space_id: got=%q want=%q", record.Attachment.SpaceID, spaceID)
	}
	if record.DocumentTitle != "附件文档" {
		t.Fatalf("unexpected document title: got=%q", record.DocumentTitle)
	}
	if record.SpaceName != "测试空间" {
		t.Fatalf("unexpected space name: got=%q", record.SpaceName)
	}
}
