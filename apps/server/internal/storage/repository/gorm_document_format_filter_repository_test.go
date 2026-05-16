package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
)

func TestGormSearchIndexSourceRepository_FiltersNonMarkdownDocuments(t *testing.T) {
	database := openDocumentFormatFilterRepositoryTestDB(
		t,
		"file:test-search-index-source-format-filter?mode=memory&cache=shared",
	)
	defer func() {
		_ = database.Close()
	}()

	ctx := context.Background()
	now := time.Now().UTC()
	seedDocumentFormatFilterUser(t, database, "repo-owner-1", now)
	seedDocumentFormatFilterSpace(t, database, "repo-space-1", "repo-owner-1", models.VisibilityPublic, now)
	seedDocumentFormatFilterDocument(
		t,
		database,
		documentFormatFilterSeedDocument{
			SpaceID:    "repo-space-1",
			NodeID:     "repo-node-md-1",
			DocumentID: "repo-doc-md-1",
			Title:      "Markdown Guide",
			ContentMD:  "# markdown body",
			Format:     models.DocumentFormatMarkdown,
			Visibility: models.VisibilityPublic,
			Now:        now,
		},
	)
	seedDocumentFormatFilterDocument(
		t,
		database,
		documentFormatFilterSeedDocument{
			SpaceID:    "repo-space-1",
			NodeID:     "repo-node-office-1",
			DocumentID: "repo-doc-office-1",
			Title:      "Office Budget",
			ContentMD:  "# office placeholder",
			Format:     models.DocumentFormatDOCX,
			Visibility: models.VisibilityPublic,
			Now:        now,
		},
	)

	repo := NewGormSearchIndexSourceRepository(database.ORM)

	rows, err := repo.ListActiveDocuments(ctx, ListSearchIndexSourceDocumentsParams{Limit: 10})
	if err != nil {
		t.Fatalf("list active documents failed: %v", err)
	}
	if len(rows) != 1 || rows[0].DocumentID != "repo-doc-md-1" {
		t.Fatalf("expected only markdown document indexed, got %+v", rows)
	}

	spaceRows, err := repo.ListActiveDocumentsBySpaceID(ctx, ListSearchIndexSourceDocumentsBySpaceParams{
		SpaceID: "repo-space-1",
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("list active documents by space failed: %v", err)
	}
	if len(spaceRows) != 1 || spaceRows[0].DocumentID != "repo-doc-md-1" {
		t.Fatalf("expected only markdown document indexed by space, got %+v", spaceRows)
	}

	_, err = repo.GetActiveDocumentByDocumentID(ctx, "repo-doc-office-1")
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected office document excluded from index source lookup, got err=%v", err)
	}
}

func TestGormHomeSearchRepository_FiltersNonMarkdownMetadata(t *testing.T) {
	database := openDocumentFormatFilterRepositoryTestDB(
		t,
		"file:test-home-search-format-filter?mode=memory&cache=shared",
	)
	defer func() {
		_ = database.Close()
	}()

	ctx := context.Background()
	now := time.Now().UTC()
	seedDocumentFormatFilterUser(t, database, "repo-owner-2", now)
	seedDocumentFormatFilterSpace(t, database, "repo-space-2", "repo-owner-2", models.VisibilityPublic, now)
	seedDocumentFormatFilterDocument(
		t,
		database,
		documentFormatFilterSeedDocument{
			SpaceID:    "repo-space-2",
			NodeID:     "repo-node-md-2",
			DocumentID: "repo-doc-md-2",
			Title:      "Markdown Landing",
			ContentMD:  "# markdown",
			Format:     models.DocumentFormatMarkdown,
			Visibility: models.VisibilityPublic,
			ReaderSlug: stringPointer("landing"),
			Now:        now,
		},
	)
	seedDocumentFormatFilterDocument(
		t,
		database,
		documentFormatFilterSeedDocument{
			SpaceID:    "repo-space-2",
			NodeID:     "repo-node-office-2",
			DocumentID: "repo-doc-office-2",
			Title:      "Office Sheet",
			ContentMD:  "# office",
			Format:     models.DocumentFormatXLSX,
			Visibility: models.VisibilityPublic,
			Now:        now,
		},
	)

	repo := NewGormHomeSearchRepository(database.ORM)

	rows, err := repo.ListActiveDocumentMetadataByDocumentIDs(ctx, []string{
		"repo-doc-md-2",
		"repo-doc-office-2",
	})
	if err != nil {
		t.Fatalf("list active document metadata failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected only markdown metadata row, got %+v", rows)
	}
	if rows[0].DocumentID != "repo-doc-md-2" || rows[0].DocumentRouteKey != "landing" {
		t.Fatalf("unexpected metadata row: %+v", rows[0])
	}
}

type documentFormatFilterSeedDocument struct {
	SpaceID    string
	NodeID     string
	DocumentID string
	Title      string
	ContentMD  string
	Format     models.DocumentFormat
	Visibility models.Visibility
	ReaderSlug *string
	Now        time.Time
}

func openDocumentFormatFilterRepositoryTestDB(t *testing.T, dsn string) *storage.Database {
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

func seedDocumentFormatFilterUser(t *testing.T, database *storage.Database, userID string, now time.Time) {
	t.Helper()

	if err := database.ORM.Create(&models.User{
		UserID:       userID,
		Email:        userID + "@example.com",
		PasswordHash: "hash",
		Name:         userID,
		Status:       models.EntityStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}).Error; err != nil {
		t.Fatalf("seed user failed: %v", err)
	}
}

func seedDocumentFormatFilterSpace(
	t *testing.T,
	database *storage.Database,
	spaceID string,
	ownerUserID string,
	visibility models.Visibility,
	now time.Time,
) {
	t.Helper()

	if err := database.ORM.Create(&models.Space{
		SpaceID:     spaceID,
		Name:        spaceID,
		OwnerUserID: ownerUserID,
		Visibility:  visibility,
		Status:      models.EntityStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}).Error; err != nil {
		t.Fatalf("seed space failed: %v", err)
	}
}

func seedDocumentFormatFilterDocument(
	t *testing.T,
	database *storage.Database,
	input documentFormatFilterSeedDocument,
) {
	t.Helper()

	if err := database.ORM.Create(&models.Node{
		NodeID:     input.NodeID,
		SpaceID:    input.SpaceID,
		Type:       models.NodeTypeDoc,
		Title:      input.Title,
		ReaderSlug: input.ReaderSlug,
		Sort:       1,
		CreatedAt:  input.Now,
		UpdatedAt:  input.Now,
	}).Error; err != nil {
		t.Fatalf("seed node failed: %v", err)
	}

	if err := database.ORM.Create(&models.Document{
		DocumentID: input.DocumentID,
		NodeID:     input.NodeID,
		ThemeID:    "default",
		Visibility: input.Visibility,
		Status:     models.EntityStatusActive,
		Title:      input.Title,
		Format:     input.Format,
		ContentMD:  input.ContentMD,
		Version:    1,
		CreatedAt:  input.Now,
		UpdatedAt:  input.Now,
	}).Error; err != nil {
		t.Fatalf("seed document failed: %v", err)
	}
}

func stringPointer(value string) *string {
	return &value
}
