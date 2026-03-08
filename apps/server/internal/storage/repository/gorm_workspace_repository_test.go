package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/oklog/ulid/v2"
	"gorm.io/gorm/clause"
)

func TestGormWorkspaceRepository_SaveOfficeDocumentPersistsRevision(t *testing.T) {
	database := openWorkspaceRepositoryTestDB(t, "file:test-workspace-save-office-document?mode=memory&cache=shared")
	defer func() {
		_ = database.Close()
	}()

	ctx := context.Background()
	now := time.Now().UTC()
	fixture := seedWorkspaceRepositoryOfficeFixture(t, database, now)
	repo := NewGormWorkspaceRepository(database.ORM)

	nextBlobID := "repo-workspace-blob-2"
	if err := database.ORM.WithContext(ctx).Create(&models.DocumentAttachmentBlob{
		BlobID:          nextBlobID,
		StorageProvider: "local",
		ObjectKey:       "uploads/blob-2.docx",
		ObjectURL:       "/uploads/blob-2.docx",
		MimeType:        fixture.MimeType,
		SizeBytes:       2048,
		ContentHashAlgo: "sha256",
		ContentHash:     "blob-hash-2",
		CreatedAt:       now.Add(2 * time.Minute),
		UpdatedAt:       now.Add(2 * time.Minute),
	}).Error; err != nil {
		t.Fatalf("seed next blob failed: %v", err)
	}

	touchedAt := now.Add(3 * time.Minute)
	saved, err := repo.SaveOfficeDocument(ctx, WorkspaceSaveOfficeDocumentParams{
		DocumentID:         fixture.DocumentID,
		BaseContentVersion: 1,
		NextVersion:        2,
		NextContentVersion: 2,
		SourceBlobID:       nextBlobID,
		SourceFileName:     "workspace-office-v2.docx",
		SourceMimeType:     fixture.MimeType,
		ActorUserID:        fixture.UserID,
		NodeID:             fixture.NodeID,
		SpaceID:            fixture.SpaceID,
		TouchedAt:          touchedAt,
		FileRevision: &models.DocumentFileRevision{
			DocumentFileRevisionID: stringsToLowerULID(),
			DocumentID:             fixture.DocumentID,
			BlobID:                 nextBlobID,
			FileName:               "workspace-office-v2.docx",
			MimeType:               fixture.MimeType,
			Version:                2,
			BaseVersion:            1,
			EditorUserID:           workspaceRepoStringPointer(fixture.UserID),
			Source:                 models.RevisionSourceRemote,
			CreatedAt:              touchedAt,
		},
	})
	if err != nil {
		t.Fatalf("save office document failed: %v", err)
	}
	if !saved {
		t.Fatal("expected save office document return saved=true")
	}

	var document struct {
		Version         int     `gorm:"column:version"`
		ContentVersion  int     `gorm:"column:content_version"`
		SourceBlobID    string  `gorm:"column:source_blob_id"`
		SourceFileName  string  `gorm:"column:source_file_name"`
		SourceMimeType  string  `gorm:"column:source_mime_type"`
		UpdatedByUserID *string `gorm:"column:updated_by_user_id"`
	}
	if err := database.ORM.WithContext(ctx).
		Table("documents").
		Select("version", "content_version", "source_blob_id", "source_file_name", "source_mime_type", "updated_by_user_id", "updated_at").
		Where("document_id = ?", fixture.DocumentID).
		Take(&document).Error; err != nil {
		t.Fatalf("query saved document failed: %v", err)
	}
	if document.Version != 2 || document.ContentVersion != 2 {
		t.Fatalf("expected version/contentVersion 2/2, got %d/%d", document.Version, document.ContentVersion)
	}
	if document.SourceBlobID != nextBlobID || document.SourceFileName != "workspace-office-v2.docx" || document.SourceMimeType != fixture.MimeType {
		t.Fatalf("unexpected source payload after save: %+v", document)
	}
	if document.UpdatedByUserID == nil || *document.UpdatedByUserID != fixture.UserID {
		t.Fatalf("expected updated_by_user_id %q, got %+v", fixture.UserID, document.UpdatedByUserID)
	}

	var revision struct {
		BlobID      string `gorm:"column:blob_id"`
		Version     int    `gorm:"column:version"`
		BaseVersion int    `gorm:"column:base_version"`
	}
	if err := database.ORM.WithContext(ctx).
		Table("document_file_revisions").
		Select("blob_id", "version", "base_version").
		Where("document_id = ?", fixture.DocumentID).
		Order("version DESC").
		Take(&revision).Error; err != nil {
		t.Fatalf("query saved file revision failed: %v", err)
	}
	if revision.BlobID != nextBlobID || revision.Version != 2 || revision.BaseVersion != 1 {
		t.Fatalf("unexpected saved file revision: %+v", revision)
	}
}

func TestGormWorkspaceRepository_SaveOfficeDocumentRollsBackOnRevisionConflict(t *testing.T) {
	database := openWorkspaceRepositoryTestDB(t, "file:test-workspace-save-office-document-rollback?mode=memory&cache=shared")
	defer func() {
		_ = database.Close()
	}()

	ctx := context.Background()
	now := time.Now().UTC()
	fixture := seedWorkspaceRepositoryOfficeFixture(t, database, now)
	repo := NewGormWorkspaceRepository(database.ORM)

	nextBlobID := "repo-workspace-blob-rollback"
	if err := database.ORM.WithContext(ctx).Create(&models.DocumentAttachmentBlob{
		BlobID:          nextBlobID,
		StorageProvider: "local",
		ObjectKey:       "uploads/blob-rollback.docx",
		ObjectURL:       "/uploads/blob-rollback.docx",
		MimeType:        fixture.MimeType,
		SizeBytes:       4096,
		ContentHashAlgo: "sha256",
		ContentHash:     "blob-hash-rollback",
		CreatedAt:       now.Add(2 * time.Minute),
		UpdatedAt:       now.Add(2 * time.Minute),
	}).Error; err != nil {
		t.Fatalf("seed rollback blob failed: %v", err)
	}
	if err := database.ORM.WithContext(ctx).Create(&models.DocumentFileRevision{
		DocumentFileRevisionID: stringsToLowerULID(),
		DocumentID:             fixture.DocumentID,
		BlobID:                 nextBlobID,
		FileName:               "conflict.docx",
		MimeType:               fixture.MimeType,
		Version:                2,
		BaseVersion:            1,
		EditorUserID:           workspaceRepoStringPointer(fixture.UserID),
		Source:                 models.RevisionSourceRemote,
		CreatedAt:              now.Add(2 * time.Minute),
	}).Error; err != nil {
		t.Fatalf("seed conflicting file revision failed: %v", err)
	}

	saved, err := repo.SaveOfficeDocument(ctx, WorkspaceSaveOfficeDocumentParams{
		DocumentID:         fixture.DocumentID,
		BaseContentVersion: 1,
		NextVersion:        2,
		NextContentVersion: 2,
		SourceBlobID:       nextBlobID,
		SourceFileName:     "workspace-office-conflict.docx",
		SourceMimeType:     fixture.MimeType,
		ActorUserID:        fixture.UserID,
		NodeID:             fixture.NodeID,
		SpaceID:            fixture.SpaceID,
		TouchedAt:          now.Add(3 * time.Minute),
		FileRevision: &models.DocumentFileRevision{
			DocumentFileRevisionID: stringsToLowerULID(),
			DocumentID:             fixture.DocumentID,
			BlobID:                 nextBlobID,
			FileName:               "workspace-office-conflict.docx",
			MimeType:               fixture.MimeType,
			Version:                2,
			BaseVersion:            1,
			EditorUserID:           workspaceRepoStringPointer(fixture.UserID),
			Source:                 models.RevisionSourceRemote,
			CreatedAt:              now.Add(3 * time.Minute),
		},
	})
	if err == nil {
		t.Fatal("expected revision conflict error, got nil")
	}
	if saved {
		t.Fatal("expected saved=false when revision insert conflicts")
	}

	var document struct {
		Version        int    `gorm:"column:version"`
		ContentVersion int    `gorm:"column:content_version"`
		SourceBlobID   string `gorm:"column:source_blob_id"`
		SourceFileName string `gorm:"column:source_file_name"`
	}
	if err := database.ORM.WithContext(ctx).
		Table("documents").
		Select("version", "content_version", "source_blob_id", "source_file_name").
		Where("document_id = ?", fixture.DocumentID).
		Take(&document).Error; err != nil {
		t.Fatalf("query rolled back document failed: %v", err)
	}
	if document.Version != 1 || document.ContentVersion != 1 {
		t.Fatalf("expected rolled back version/contentVersion 1/1, got %d/%d", document.Version, document.ContentVersion)
	}
	if document.SourceBlobID != fixture.InitialBlobID || document.SourceFileName != "workspace-office.docx" {
		t.Fatalf("expected rolled back source blob payload, got %+v", document)
	}

	var revisionCount int64
	if err := database.ORM.WithContext(ctx).
		Table("document_file_revisions").
		Where("document_id = ?", fixture.DocumentID).
		Count(&revisionCount).Error; err != nil {
		t.Fatalf("count rolled back file revisions failed: %v", err)
	}
	if revisionCount != 2 {
		t.Fatalf("expected only seeded file revisions remain, got %d", revisionCount)
	}
}

type workspaceRepositoryOfficeFixture struct {
	UserID        string
	SpaceID       string
	NodeID        string
	DocumentID    string
	InitialBlobID string
	MimeType      string
}

func openWorkspaceRepositoryTestDB(t *testing.T, dsn string) *storage.Database {
	t.Helper()

	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    dsn,
	})
	if err != nil {
		t.Fatalf("open database failed: %v", err)
	}
	if err := storage.MigrateUp(context.Background(), database.ORM, storage.DriverSQLite); err != nil {
		t.Fatalf("migrate up failed: %v", err)
	}
	return database
}

func seedWorkspaceRepositoryOfficeFixture(
	t *testing.T,
	database *storage.Database,
	now time.Time,
) workspaceRepositoryOfficeFixture {
	t.Helper()

	fixture := workspaceRepositoryOfficeFixture{
		UserID:        "repo-workspace-user-1",
		SpaceID:       "repo-workspace-space-1",
		NodeID:        "repo-workspace-node-1",
		DocumentID:    "repo-workspace-doc-1",
		InitialBlobID: "repo-workspace-blob-1",
		MimeType:      "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	}

	if err := database.ORM.Create(&models.User{
		UserID:       fixture.UserID,
		Email:        "workspace-repo@example.com",
		PasswordHash: "hash",
		Name:         "workspace repo user",
		Status:       models.EntityStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}).Error; err != nil {
		t.Fatalf("seed user failed: %v", err)
	}
	if err := database.ORM.Create(&models.Space{
		SpaceID:     fixture.SpaceID,
		Name:        "workspace repo space",
		OwnerUserID: fixture.UserID,
		Visibility:  models.VisibilityMember,
		Status:      models.EntityStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}).Error; err != nil {
		t.Fatalf("seed space failed: %v", err)
	}
	if err := database.ORM.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.Theme{
		ThemeID:                "default",
		Name:                   "默认主题",
		Description:            "default",
		VariablesJSON:          "{}",
		SyntaxTheme:            "one-light",
		CodeBlockStyleJSON:     "{}",
		CodeBlockCodeStyleJSON: "{}",
		InlineCodeStyleJSON:    "{}",
		IsBuiltin:              true,
		IsEnabled:              true,
		CreatedAt:              now,
		UpdatedAt:              now,
	}).Error; err != nil {
		t.Fatalf("seed theme failed: %v", err)
	}
	if err := database.ORM.Create(&models.Node{
		NodeID:    fixture.NodeID,
		SpaceID:   fixture.SpaceID,
		Type:      models.NodeTypeDoc,
		Title:     "workspace office",
		Sort:      1,
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed node failed: %v", err)
	}
	if err := database.ORM.Create(&models.DocumentAttachmentBlob{
		BlobID:          fixture.InitialBlobID,
		StorageProvider: "local",
		ObjectKey:       "uploads/blob-1.docx",
		ObjectURL:       "/uploads/blob-1.docx",
		MimeType:        fixture.MimeType,
		SizeBytes:       1024,
		ContentHashAlgo: "sha256",
		ContentHash:     "blob-hash-1",
		CreatedAt:       now,
		UpdatedAt:       now,
	}).Error; err != nil {
		t.Fatalf("seed initial blob failed: %v", err)
	}
	if err := database.ORM.Create(&models.Document{
		DocumentID:      fixture.DocumentID,
		NodeID:          fixture.NodeID,
		ThemeID:         "default",
		Visibility:      models.VisibilityMember,
		Status:          models.EntityStatusActive,
		Title:           "workspace office",
		Format:          models.DocumentFormatDOCX,
		Version:         1,
		ContentVersion:  1,
		SourceBlobID:    workspaceRepoStringPointer(fixture.InitialBlobID),
		SourceFileName:  workspaceRepoStringPointer("workspace-office.docx"),
		SourceMimeType:  workspaceRepoStringPointer(fixture.MimeType),
		CreatedByUserID: workspaceRepoStringPointer(fixture.UserID),
		UpdatedByUserID: workspaceRepoStringPointer(fixture.UserID),
		CreatedAt:       now,
		UpdatedAt:       now,
	}).Error; err != nil {
		t.Fatalf("seed document failed: %v", err)
	}
	if err := database.ORM.Create(&models.DocumentFileRevision{
		DocumentFileRevisionID: stringsToLowerULID(),
		DocumentID:             fixture.DocumentID,
		BlobID:                 fixture.InitialBlobID,
		FileName:               "workspace-office.docx",
		MimeType:               fixture.MimeType,
		Version:                1,
		BaseVersion:            1,
		EditorUserID:           workspaceRepoStringPointer(fixture.UserID),
		Source:                 models.RevisionSourceLocal,
		CreatedAt:              now,
	}).Error; err != nil {
		t.Fatalf("seed initial file revision failed: %v", err)
	}

	return fixture
}

func stringsToLowerULID() string {
	return strings.ToLower(ulid.Make().String())
}

func workspaceRepoStringPointer(value string) *string {
	return &value
}
