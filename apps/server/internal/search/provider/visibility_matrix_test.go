package provider

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type visibilityMatrixCase struct {
	name        string
	actorUserID string
	expectedDoc []string
}

func TestDatabaseProvider_SearchMatchesVisibilityMatrix(t *testing.T) {
	db := openVisibilityMatrixTestDB(t, "file:test-database-provider-visibility-matrix?mode=memory&cache=shared")
	seedVisibilityMatrixTestData(t, db)

	provider := NewDatabaseProvider(db)
	runVisibilityMatrixAssertions(t, "database", func(ctx context.Context, actorUserID string) (SearchResponse, error) {
		return provider.Search(ctx, SearchRequest{
			Query:       "matrix hit",
			Page:        1,
			PageSize:    100,
			ActorUserID: actorUserID,
		})
	})
}

func TestBleveProvider_SearchMatchesVisibilityMatrix(t *testing.T) {
	db := openVisibilityMatrixTestDB(t, "file:test-bleve-provider-visibility-matrix?mode=memory&cache=shared")
	seedVisibilityMatrixTestData(t, db)

	provider := NewBleveProvider(BleveProviderOptions{
		DB:        db,
		IndexPath: filepath.Join(t.TempDir(), "bleve-index"),
	})
	ctx := context.Background()
	if err := provider.EnsureSchema(ctx); err != nil {
		t.Fatalf("bleve ensure schema failed: %v", err)
	}

	now := time.Now().UTC().Unix()
	if err := provider.Upsert(ctx, buildVisibilityMatrixIndexRecords(now)); err != nil {
		t.Fatalf("bleve upsert failed: %v", err)
	}

	runVisibilityMatrixAssertions(t, "bleve", func(ctx context.Context, actorUserID string) (SearchResponse, error) {
		return provider.Search(ctx, SearchRequest{
			Query:       "matrix hit",
			Page:        1,
			PageSize:    100,
			ActorUserID: actorUserID,
		})
	})
}

func runVisibilityMatrixAssertions(
	t *testing.T,
	providerName string,
	search func(ctx context.Context, actorUserID string) (SearchResponse, error),
) {
	t.Helper()

	cases := []visibilityMatrixCase{
		{
			name:        "anonymous",
			actorUserID: "",
			expectedDoc: []string{
				"doc-public-public",
			},
		},
		{
			name:        "logged-in-non-member",
			actorUserID: "user-1",
			expectedDoc: []string{
				"doc-public-public",
				"doc-public-authenticated",
				"doc-authenticated-public",
				"doc-authenticated-authenticated",
			},
		},
		{
			name:        "logged-in-member",
			actorUserID: "member-1",
			expectedDoc: []string{
				"doc-public-public",
				"doc-public-authenticated",
				"doc-authenticated-public",
				"doc-authenticated-authenticated",
				"doc-member-public",
				"doc-member-authenticated",
				"doc-member-member",
			},
		},
		{
			name:        "space-owner",
			actorUserID: "owner-authenticated",
			expectedDoc: []string{
				"doc-public-public",
				"doc-public-authenticated",
				"doc-authenticated-public",
				"doc-authenticated-authenticated",
				"doc-authenticated-member",
			},
		},
	}

	for _, item := range cases {
		item := item
		t.Run(item.name, func(t *testing.T) {
			result, err := search(context.Background(), item.actorUserID)
			if err != nil {
				t.Fatalf("%s search failed: %v", providerName, err)
			}
			assertSearchDocSet(t, result, item.expectedDoc)
		})
	}
}

func assertSearchDocSet(t *testing.T, result SearchResponse, expectedDocIDs []string) {
	t.Helper()

	gotDocSet := make(map[string]struct{}, len(result.Hits))
	for _, hit := range result.Hits {
		docID := strings.TrimSpace(hit.DocID)
		if docID == "" {
			continue
		}
		gotDocSet[docID] = struct{}{}
	}

	if int(result.Total) != len(expectedDocIDs) {
		t.Fatalf("expected total=%d, got=%d", len(expectedDocIDs), result.Total)
	}
	if len(gotDocSet) != len(expectedDocIDs) {
		t.Fatalf("expected %d hits, got %d (hits=%+v)", len(expectedDocIDs), len(gotDocSet), result.Hits)
	}
	for _, docID := range expectedDocIDs {
		if _, exists := gotDocSet[docID]; !exists {
			t.Fatalf("expected doc %q in hits, got=%+v", docID, result.Hits)
		}
	}
}

func openVisibilityMatrixTestDB(t *testing.T, dsn string) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := prepareVisibilityMatrixTestSchema(db); err != nil {
		t.Fatalf("prepare schema failed: %v", err)
	}
	return db
}

func prepareVisibilityMatrixTestSchema(db *gorm.DB) error {
	if err := db.Exec(`
CREATE TABLE spaces (
  space_id TEXT PRIMARY KEY,
  owner_user_id TEXT,
  visibility TEXT,
  status TEXT,
  deleted_at TEXT NULL,
  name TEXT
)`).Error; err != nil {
		return err
	}
	if err := db.Exec(`
CREATE TABLE nodes (
  node_id TEXT PRIMARY KEY,
  space_id TEXT
)`).Error; err != nil {
		return err
	}
	if err := db.Exec(`
CREATE TABLE documents (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  document_id TEXT,
  node_id TEXT,
  title TEXT,
  content_md TEXT,
  visibility TEXT,
  status TEXT,
  deleted_at TEXT NULL,
  updated_at TEXT
)`).Error; err != nil {
		return err
	}
	if err := db.Exec(`
CREATE TABLE space_members (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  space_id TEXT,
  user_id TEXT
)`).Error; err != nil {
		return err
	}
	return nil
}

func seedVisibilityMatrixTestData(t *testing.T, db *gorm.DB) {
	t.Helper()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := db.Exec(`
INSERT INTO spaces(space_id, owner_user_id, visibility, status, deleted_at, name) VALUES
  ('space-public', 'owner-public', 'public', 'active', NULL, 'Space Public'),
  ('space-authenticated', 'owner-authenticated', 'authenticated', 'active', NULL, 'Space Authenticated'),
  ('space-member', 'owner-member', 'member', 'active', NULL, 'Space Member')
`).Error; err != nil {
		t.Fatalf("seed spaces failed: %v", err)
	}

	documents := []struct {
		DocID      string
		NodeID     string
		SpaceID    string
		Visibility string
	}{
		{DocID: "doc-public-public", NodeID: "node-public-public", SpaceID: "space-public", Visibility: "public"},
		{DocID: "doc-public-authenticated", NodeID: "node-public-authenticated", SpaceID: "space-public", Visibility: "authenticated"},
		{DocID: "doc-public-member", NodeID: "node-public-member", SpaceID: "space-public", Visibility: "member"},
		{DocID: "doc-authenticated-public", NodeID: "node-authenticated-public", SpaceID: "space-authenticated", Visibility: "public"},
		{DocID: "doc-authenticated-authenticated", NodeID: "node-authenticated-authenticated", SpaceID: "space-authenticated", Visibility: "authenticated"},
		{DocID: "doc-authenticated-member", NodeID: "node-authenticated-member", SpaceID: "space-authenticated", Visibility: "member"},
		{DocID: "doc-member-public", NodeID: "node-member-public", SpaceID: "space-member", Visibility: "public"},
		{DocID: "doc-member-authenticated", NodeID: "node-member-authenticated", SpaceID: "space-member", Visibility: "authenticated"},
		{DocID: "doc-member-member", NodeID: "node-member-member", SpaceID: "space-member", Visibility: "member"},
	}

	for _, item := range documents {
		if err := db.Exec(
			`INSERT INTO nodes(node_id, space_id) VALUES (?, ?)`,
			item.NodeID,
			item.SpaceID,
		).Error; err != nil {
			t.Fatalf("seed node failed (%s): %v", item.NodeID, err)
		}
		if err := db.Exec(
			`INSERT INTO documents(document_id, node_id, title, content_md, visibility, status, deleted_at, updated_at) VALUES (?, ?, ?, ?, ?, 'active', NULL, ?)`,
			item.DocID,
			item.NodeID,
			"matrix hit "+item.DocID,
			"matrix hit body "+item.DocID,
			item.Visibility,
			now,
		).Error; err != nil {
			t.Fatalf("seed document failed (%s): %v", item.DocID, err)
		}
	}

	if err := db.Exec(
		`INSERT INTO space_members(space_id, user_id) VALUES (?, ?)`,
		"space-member",
		"member-1",
	).Error; err != nil {
		t.Fatalf("seed space member failed: %v", err)
	}
}

func buildVisibilityMatrixIndexRecords(updatedAtUnix int64) []IndexRecord {
	return []IndexRecord{
		buildVisibilityIndexRecord("space-public", "doc-public-public", "node-public-public", updatedAtUnix),
		buildVisibilityIndexRecord("space-public", "doc-public-authenticated", "node-public-authenticated", updatedAtUnix),
		buildVisibilityIndexRecord("space-public", "doc-public-member", "node-public-member", updatedAtUnix),
		buildVisibilityIndexRecord("space-authenticated", "doc-authenticated-public", "node-authenticated-public", updatedAtUnix),
		buildVisibilityIndexRecord("space-authenticated", "doc-authenticated-authenticated", "node-authenticated-authenticated", updatedAtUnix),
		buildVisibilityIndexRecord("space-authenticated", "doc-authenticated-member", "node-authenticated-member", updatedAtUnix),
		buildVisibilityIndexRecord("space-member", "doc-member-public", "node-member-public", updatedAtUnix),
		buildVisibilityIndexRecord("space-member", "doc-member-authenticated", "node-member-authenticated", updatedAtUnix),
		buildVisibilityIndexRecord("space-member", "doc-member-member", "node-member-member", updatedAtUnix),
	}
}

func buildVisibilityIndexRecord(spaceID string, docID string, nodeID string, updatedAtUnix int64) IndexRecord {
	return IndexRecord{
		SpaceID:       spaceID,
		DocID:         docID,
		NodeID:        nodeID,
		Title:         "matrix hit " + docID,
		BodyPlain:     "matrix hit body " + docID,
		Terms:         "matrix hit",
		TitleTerms:    "matrix hit",
		UpdatedAtUnix: updatedAtUnix,
		SpaceStatus:   "active",
		DocStatus:     "active",
	}
}
