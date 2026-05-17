package repository

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	gormlogger "gorm.io/gorm/logger"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
)

func TestGormWorkspaceRepository_ListRevisionSummariesPaginatesMarkdownAndOffice(t *testing.T) {
	database := openWorkspaceRepositoryTestDB(t, "file:test-workspace-list-revision-summaries?mode=memory&cache=shared")
	defer func() {
		_ = database.Close()
	}()

	ctx := context.Background()
	now := time.Now().UTC()
	fixture := seedWorkspaceRepositoryRevisionFixture(t, database, now)
	repo := NewGormWorkspaceRepository(database.ORM)

	markdownSummaries, err := repo.ListRevisionSummariesByDocumentID(ctx, WorkspaceListRevisionSummariesParams{
		DocumentID: fixture.MarkdownDocumentID,
		Limit:      2,
		Offset:     1,
	})
	if err != nil {
		t.Fatalf("list markdown revision summaries failed: %v", err)
	}
	if len(markdownSummaries) != 2 {
		t.Fatalf("expected two paginated markdown summaries, got %d", len(markdownSummaries))
	}
	if markdownSummaries[0].Version != 2 || markdownSummaries[1].Version != 1 {
		t.Fatalf("expected markdown summaries versions 2,1 after offset, got %d,%d", markdownSummaries[0].Version, markdownSummaries[1].Version)
	}
	if markdownSummaries[0].Format != models.DocumentFormatMarkdown {
		t.Fatalf("expected markdown summary format, got %q", markdownSummaries[0].Format)
	}
	if markdownSummaries[0].EditorUserID == nil || *markdownSummaries[0].EditorUserID != fixture.EditorUserID {
		t.Fatalf("expected editor user id %q, got %+v", fixture.EditorUserID, markdownSummaries[0].EditorUserID)
	}
	if markdownSummaries[0].EditorUserName == nil || *markdownSummaries[0].EditorUserName != "Revision Editor" {
		t.Fatalf("expected editor user name Revision Editor, got %+v", markdownSummaries[0].EditorUserName)
	}

	officeSummaries, err := repo.ListRevisionSummariesByDocumentID(ctx, WorkspaceListRevisionSummariesParams{
		DocumentID: fixture.OfficeDocumentID,
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("list office revision summaries failed: %v", err)
	}
	if len(officeSummaries) != 2 {
		t.Fatalf("expected two office summaries, got %d", len(officeSummaries))
	}
	if officeSummaries[0].Version != 2 || officeSummaries[1].Version != 1 {
		t.Fatalf("expected office summaries versions 2,1, got %d,%d", officeSummaries[0].Version, officeSummaries[1].Version)
	}
	if officeSummaries[0].Format != models.DocumentFormatDOCX {
		t.Fatalf("expected office summary format docx, got %q", officeSummaries[0].Format)
	}
	if officeSummaries[0].FileName == nil || *officeSummaries[0].FileName != "office-v2.docx" {
		t.Fatalf("expected office file name, got %+v", officeSummaries[0].FileName)
	}
	if officeSummaries[0].MimeType == nil || *officeSummaries[0].MimeType != fixture.OfficeMimeType {
		t.Fatalf("expected office mime type, got %+v", officeSummaries[0].MimeType)
	}
	if officeSummaries[1].EditorUserID != nil || officeSummaries[1].EditorUserName != nil {
		t.Fatalf("missing editor should stay nil for frontend unknown/system display, got %+v/%+v", officeSummaries[1].EditorUserID, officeSummaries[1].EditorUserName)
	}
}

func TestGormWorkspaceRepository_ListRevisionSummariesBoundsSourceQueries(t *testing.T) {
	database := openWorkspaceRepositoryTestDB(t, "file:test-workspace-list-revision-summaries-bounds?mode=memory&cache=shared")
	defer func() {
		_ = database.Close()
	}()

	ctx := context.Background()
	now := time.Now().UTC()
	fixture := seedWorkspaceRepositoryRevisionFixture(t, database, now)
	sqlLogger := &sqlCaptureLogger{Interface: gormlogger.Default.LogMode(gormlogger.Silent)}
	repo := NewGormWorkspaceRepository(database.ORM.Session(&gorm.Session{
		Logger: sqlLogger,
	}))

	if _, err := repo.ListRevisionSummariesByDocumentID(ctx, WorkspaceListRevisionSummariesParams{
		DocumentID: fixture.MarkdownDocumentID,
		Limit:      2,
		Offset:     1,
	}); err != nil {
		t.Fatalf("list bounded revision summaries failed: %v", err)
	}

	var revisionSummarySelects int
	var boundedSelects int
	for _, statement := range sqlLogger.statements {
		normalized := normalizeCapturedSQL(statement)
		if strings.Contains(normalized, "from document_revisions") ||
			strings.Contains(normalized, "from document_file_revisions") {
			revisionSummarySelects++
			if strings.Contains(normalized, "limit 3") {
				boundedSelects++
			}
		}
	}
	if revisionSummarySelects == 0 {
		t.Fatalf("expected revision summary SQL statements, got %v", sqlLogger.statements)
	}
	if boundedSelects != revisionSummarySelects {
		t.Fatalf("expected all revision summary source queries to be bounded by offset+limit, statements=%v", sqlLogger.statements)
	}
}

func TestGormWorkspaceRepository_GetRevisionDetailByIDReturnsMarkdownAndOffice(t *testing.T) {
	database := openWorkspaceRepositoryTestDB(t, "file:test-workspace-get-revision-detail?mode=memory&cache=shared")
	defer func() {
		_ = database.Close()
	}()

	ctx := context.Background()
	now := time.Now().UTC()
	fixture := seedWorkspaceRepositoryRevisionFixture(t, database, now)
	repo := NewGormWorkspaceRepository(database.ORM)

	markdownDetail, err := repo.GetRevisionDetailByID(ctx, fixture.MarkdownDocumentID, fixture.MarkdownRevisionID)
	if err != nil {
		t.Fatalf("get markdown revision detail failed: %v", err)
	}
	if markdownDetail.Format != models.DocumentFormatMarkdown {
		t.Fatalf("expected markdown detail format, got %q", markdownDetail.Format)
	}
	if markdownDetail.ContentMD == nil || *markdownDetail.ContentMD != "# Markdown v3" {
		t.Fatalf("expected markdown content body, got %+v", markdownDetail.ContentMD)
	}
	if markdownDetail.BlobID != nil || markdownDetail.FileName != nil || markdownDetail.MimeType != nil {
		t.Fatalf("markdown detail must not include office file metadata, got blob=%+v file=%+v mime=%+v", markdownDetail.BlobID, markdownDetail.FileName, markdownDetail.MimeType)
	}

	officeDetail, err := repo.GetRevisionDetailByID(ctx, fixture.OfficeDocumentID, fixture.OfficeRevisionID)
	if err != nil {
		t.Fatalf("get office revision detail failed: %v", err)
	}
	if officeDetail.Format != models.DocumentFormatDOCX {
		t.Fatalf("expected office detail format docx, got %q", officeDetail.Format)
	}
	if officeDetail.ContentMD != nil {
		t.Fatalf("office detail must not include markdown content, got %+v", officeDetail.ContentMD)
	}
	if officeDetail.BlobID == nil || *officeDetail.BlobID != fixture.OfficeBlobID {
		t.Fatalf("expected office blob id %q, got %+v", fixture.OfficeBlobID, officeDetail.BlobID)
	}
	if officeDetail.FileName == nil || *officeDetail.FileName != "office-v2.docx" {
		t.Fatalf("expected office file name, got %+v", officeDetail.FileName)
	}

	_, err = repo.GetRevisionDetailByID(ctx, fixture.OfficeDocumentID, fixture.MarkdownRevisionID)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected not found for revision from another document, got %v", err)
	}
}

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
			EditorUserID:           new(fixture.UserID),
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
		Model(&models.Document{}).
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
		Model(&models.DocumentFileRevision{}).
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
		EditorUserID:           new(fixture.UserID),
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
			EditorUserID:           new(fixture.UserID),
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
		Model(&models.Document{}).
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
		Model(&models.DocumentFileRevision{}).
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

type workspaceRepositoryRevisionFixture struct {
	OwnerUserID        string
	EditorUserID       string
	SpaceID            string
	MarkdownNodeID     string
	MarkdownDocumentID string
	MarkdownRevisionID string
	OfficeNodeID       string
	OfficeDocumentID   string
	OfficeRevisionID   string
	OfficeBlobID       string
	OfficeMimeType     string
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

func seedWorkspaceRepositoryRevisionFixture(
	t *testing.T,
	database *storage.Database,
	now time.Time,
) workspaceRepositoryRevisionFixture {
	t.Helper()

	fixture := workspaceRepositoryRevisionFixture{
		OwnerUserID:        "repo-revision-owner",
		EditorUserID:       "repo-revision-editor",
		SpaceID:            "repo-revision-space",
		MarkdownNodeID:     "repo-revision-markdown-node",
		MarkdownDocumentID: "repo-revision-markdown-doc",
		MarkdownRevisionID: "repo-revision-markdown-rev-3",
		OfficeNodeID:       "repo-revision-office-node",
		OfficeDocumentID:   "repo-revision-office-doc",
		OfficeRevisionID:   "repo-revision-office-rev-2",
		OfficeBlobID:       "repo-revision-office-blob-2",
		OfficeMimeType:     "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	}

	if err := database.ORM.Create([]models.User{
		{
			UserID:       fixture.OwnerUserID,
			Email:        "revision-owner@example.com",
			PasswordHash: "hash",
			Name:         "Revision Owner",
			Status:       models.EntityStatusActive,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			UserID:       fixture.EditorUserID,
			Email:        "revision-editor@example.com",
			PasswordHash: "hash",
			Name:         "Revision Editor",
			Status:       models.EntityStatusActive,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}).Error; err != nil {
		t.Fatalf("seed revision users failed: %v", err)
	}
	if err := database.ORM.Create(&models.Space{
		SpaceID:     fixture.SpaceID,
		Name:        "revision space",
		OwnerUserID: fixture.OwnerUserID,
		Visibility:  models.VisibilityMember,
		Status:      models.EntityStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}).Error; err != nil {
		t.Fatalf("seed revision space failed: %v", err)
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
		t.Fatalf("seed revision theme failed: %v", err)
	}
	if err := database.ORM.Create([]models.Node{
		{
			NodeID:    fixture.MarkdownNodeID,
			SpaceID:   fixture.SpaceID,
			Type:      models.NodeTypeDoc,
			Title:     "markdown revision doc",
			Sort:      1,
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			NodeID:    fixture.OfficeNodeID,
			SpaceID:   fixture.SpaceID,
			Type:      models.NodeTypeDoc,
			Title:     "office revision doc",
			Sort:      2,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}).Error; err != nil {
		t.Fatalf("seed revision nodes failed: %v", err)
	}
	if err := database.ORM.Create([]models.DocumentAttachmentBlob{
		{
			BlobID:          "repo-revision-office-blob-1",
			StorageProvider: "local",
			ObjectKey:       "uploads/revision-office-v1.docx",
			ObjectURL:       "/uploads/revision-office-v1.docx",
			MimeType:        fixture.OfficeMimeType,
			SizeBytes:       1024,
			ContentHashAlgo: "sha256",
			ContentHash:     "revision-office-blob-hash-1",
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		{
			BlobID:          fixture.OfficeBlobID,
			StorageProvider: "local",
			ObjectKey:       "uploads/revision-office-v2.docx",
			ObjectURL:       "/uploads/revision-office-v2.docx",
			MimeType:        fixture.OfficeMimeType,
			SizeBytes:       2048,
			ContentHashAlgo: "sha256",
			ContentHash:     "revision-office-blob-hash-2",
			CreatedAt:       now.Add(time.Minute),
			UpdatedAt:       now.Add(time.Minute),
		},
	}).Error; err != nil {
		t.Fatalf("seed revision blobs failed: %v", err)
	}
	if err := database.ORM.Create([]models.Document{
		{
			DocumentID:      fixture.MarkdownDocumentID,
			NodeID:          fixture.MarkdownNodeID,
			ThemeID:         "default",
			Visibility:      models.VisibilityMember,
			Status:          models.EntityStatusActive,
			Title:           "markdown revision doc",
			Format:          models.DocumentFormatMarkdown,
			ContentMD:       "# Markdown current",
			Version:         3,
			ContentVersion:  3,
			CreatedByUserID: new(fixture.OwnerUserID),
			UpdatedByUserID: new(fixture.EditorUserID),
			CreatedAt:       now,
			UpdatedAt:       now.Add(3 * time.Minute),
		},
		{
			DocumentID:      fixture.OfficeDocumentID,
			NodeID:          fixture.OfficeNodeID,
			ThemeID:         "default",
			Visibility:      models.VisibilityMember,
			Status:          models.EntityStatusActive,
			Title:           "office revision doc",
			Format:          models.DocumentFormatDOCX,
			Version:         2,
			ContentVersion:  2,
			SourceBlobID:    new(fixture.OfficeBlobID),
			SourceFileName:  new("office-v2.docx"),
			SourceMimeType:  new(fixture.OfficeMimeType),
			CreatedByUserID: new(fixture.OwnerUserID),
			UpdatedByUserID: new(fixture.EditorUserID),
			CreatedAt:       now,
			UpdatedAt:       now.Add(2 * time.Minute),
		},
	}).Error; err != nil {
		t.Fatalf("seed revision documents failed: %v", err)
	}
	if err := database.ORM.Create([]models.DocumentRevision{
		{
			DocumentRevisionID: "repo-revision-markdown-rev-1",
			DocumentID:         fixture.MarkdownDocumentID,
			Version:            1,
			ContentMD:          "# Markdown v1",
			BaseVersion:        0,
			EditorUserID:       new(fixture.OwnerUserID),
			Source:             models.RevisionSourceLocal,
			CreatedAt:          now,
		},
		{
			DocumentRevisionID: "repo-revision-markdown-rev-2",
			DocumentID:         fixture.MarkdownDocumentID,
			Version:            2,
			ContentMD:          "# Markdown v2",
			BaseVersion:        1,
			EditorUserID:       new(fixture.EditorUserID),
			Source:             models.RevisionSourceRemote,
			CreatedAt:          now.Add(time.Minute),
		},
		{
			DocumentRevisionID: fixture.MarkdownRevisionID,
			DocumentID:         fixture.MarkdownDocumentID,
			Version:            3,
			ContentMD:          "# Markdown v3",
			BaseVersion:        2,
			EditorUserID:       new(fixture.EditorUserID),
			Source:             models.RevisionSourceRemote,
			CreatedAt:          now.Add(2 * time.Minute),
		},
	}).Error; err != nil {
		t.Fatalf("seed markdown revisions failed: %v", err)
	}
	if err := database.ORM.Create([]models.DocumentFileRevision{
		{
			DocumentFileRevisionID: "repo-revision-office-rev-1",
			DocumentID:             fixture.OfficeDocumentID,
			BlobID:                 "repo-revision-office-blob-1",
			FileName:               "office-v1.docx",
			MimeType:               fixture.OfficeMimeType,
			Version:                1,
			BaseVersion:            0,
			EditorUserID:           nil,
			Source:                 models.RevisionSourceLocal,
			CreatedAt:              now,
		},
		{
			DocumentFileRevisionID: fixture.OfficeRevisionID,
			DocumentID:             fixture.OfficeDocumentID,
			BlobID:                 fixture.OfficeBlobID,
			FileName:               "office-v2.docx",
			MimeType:               fixture.OfficeMimeType,
			Version:                2,
			BaseVersion:            1,
			EditorUserID:           new(fixture.EditorUserID),
			Source:                 models.RevisionSourceRemote,
			CreatedAt:              now.Add(time.Minute),
		},
	}).Error; err != nil {
		t.Fatalf("seed office revisions failed: %v", err)
	}

	return fixture
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
		SourceBlobID:    new(fixture.InitialBlobID),
		SourceFileName:  new("workspace-office.docx"),
		SourceMimeType:  new(fixture.MimeType),
		CreatedByUserID: new(fixture.UserID),
		UpdatedByUserID: new(fixture.UserID),
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
		EditorUserID:           new(fixture.UserID),
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
