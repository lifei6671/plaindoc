package provider

import (
	"context"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBleveProvider_SearchAndPurgeBySpace(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:test-bleve-provider?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := prepareBleveProviderTestSchema(db); err != nil {
		t.Fatalf("prepare schema failed: %v", err)
	}
	if err := seedBleveProviderTestData(db); err != nil {
		t.Fatalf("seed data failed: %v", err)
	}

	provider := NewBleveProvider(BleveProviderOptions{
		DB:        db,
		IndexPath: filepath.Join(t.TempDir(), "bleve-index"),
	})
	ctx := context.Background()
	if err := provider.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema failed: %v", err)
	}

	now := time.Now().UTC().Unix()
	if err := provider.Upsert(ctx, []IndexRecord{
		{
			SpaceID:         "space-1",
			DocID:           "doc-1",
			NodeID:          "node-1",
			Title:           "检索命中文档",
			BodyPlain:       "这是公开文档",
			Terms:           "检 索 命 中 文 档",
			TitleTerms:      "检 索 命 中 文 档",
			VisibilityScope: string(models.VisibilityPublic),
			MinRole:         1,
			UpdatedAtUnix:   now,
			SpaceStatus:     "active",
			DocStatus:       "active",
		},
		{
			SpaceID:         "space-1",
			DocID:           "doc-2",
			NodeID:          "node-2",
			Title:           "成员命中文档",
			BodyPlain:       "这是成员文档",
			Terms:           "成 员 命 中 文 档",
			TitleTerms:      "成 员 命 中 文 档",
			VisibilityScope: string(models.VisibilityMember),
			MinRole:         1,
			UpdatedAtUnix:   now,
			SpaceStatus:     "active",
			DocStatus:       "active",
		},
	}); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}

	anonymousResult, err := provider.Search(ctx, SearchRequest{
		Query: "命 中",
		Page:  1,
	})
	if err != nil {
		t.Fatalf("anonymous search failed: %v", err)
	}
	if anonymousResult.Total != 1 {
		t.Fatalf("expected anonymous total=1, got=%d", anonymousResult.Total)
	}
	if len(anonymousResult.Hits) != 1 || anonymousResult.Hits[0].DocID != "doc-1" {
		t.Fatalf("expected anonymous hit doc-1, got=%+v", anonymousResult.Hits)
	}

	memberResult, err := provider.Search(ctx, SearchRequest{
		Query:       "命 中",
		Page:        1,
		ActorUserID: "member-1",
	})
	if err != nil {
		t.Fatalf("member search failed: %v", err)
	}
	if memberResult.Total != 2 {
		t.Fatalf("expected member total=2, got=%d", memberResult.Total)
	}

	if err := provider.PurgeBySpace(ctx, "space-1"); err != nil {
		t.Fatalf("purge by space failed: %v", err)
	}
	afterPurgeResult, err := provider.Search(ctx, SearchRequest{
		Query:       "命 中",
		Page:        1,
		ActorUserID: "member-1",
	})
	if err != nil {
		t.Fatalf("search after purge failed: %v", err)
	}
	if afterPurgeResult.Total != 0 {
		t.Fatalf("expected total=0 after purge, got=%d", afterPurgeResult.Total)
	}
}

func TestBleveProvider_SearchEnforcesMinRoleForMemberDocsInSingleSpace(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:test-bleve-provider-min-role?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := prepareBleveProviderTestSchema(db); err != nil {
		t.Fatalf("prepare schema failed: %v", err)
	}
	if err := seedBleveProviderTestData(db); err != nil {
		t.Fatalf("seed data failed: %v", err)
	}

	provider := NewBleveProvider(BleveProviderOptions{
		DB:        db,
		IndexPath: filepath.Join(t.TempDir(), "bleve-index-min-role"),
	})
	ctx := context.Background()
	if err := provider.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema failed: %v", err)
	}

	now := time.Now().UTC().Unix()
	if err := provider.Upsert(ctx, []IndexRecord{
		{
			SpaceID:         "space-1",
			DocID:           "doc-min-role-1",
			NodeID:          "node-min-role-1",
			Title:           "成员权限文档1",
			BodyPlain:       "单空间 min role 文档 1",
			Terms:           "min role 文档",
			TitleTerms:      "min role 文档",
			VisibilityScope: string(models.VisibilityMember),
			MinRole:         1,
			UpdatedAtUnix:   now,
			SpaceStatus:     "active",
			DocStatus:       "active",
		},
		{
			SpaceID:         "space-1",
			DocID:           "doc-min-role-2",
			NodeID:          "node-min-role-2",
			Title:           "成员权限文档2",
			BodyPlain:       "单空间 min role 文档 2",
			Terms:           "min role 文档",
			TitleTerms:      "min role 文档",
			VisibilityScope: string(models.VisibilityMember),
			MinRole:         2,
			UpdatedAtUnix:   now,
			SpaceStatus:     "active",
			DocStatus:       "active",
		},
		{
			SpaceID:         "space-1",
			DocID:           "doc-min-role-3",
			NodeID:          "node-min-role-3",
			Title:           "成员权限文档3",
			BodyPlain:       "单空间 min role 文档 3",
			Terms:           "min role 文档",
			TitleTerms:      "min role 文档",
			VisibilityScope: string(models.VisibilityMember),
			MinRole:         3,
			UpdatedAtUnix:   now,
			SpaceStatus:     "active",
			DocStatus:       "active",
		},
	}); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}

	cases := []struct {
		name          string
		userRoleLevel int
		expectedDocID []string
	}{
		{
			name:          "reader-role-level-1",
			userRoleLevel: 1,
			expectedDocID: []string{"doc-min-role-1"},
		},
		{
			name:          "collaborator-role-level-2",
			userRoleLevel: 2,
			expectedDocID: []string{"doc-min-role-1", "doc-min-role-2"},
		},
		{
			name:          "owner-role-level-3",
			userRoleLevel: 3,
			expectedDocID: []string{"doc-min-role-1", "doc-min-role-2", "doc-min-role-3"},
		},
	}

	for _, item := range cases {
		item := item
		t.Run(item.name, func(t *testing.T) {
			result, searchErr := provider.Search(ctx, SearchRequest{
				SpaceID:         "space-1",
				Query:           "min role",
				Page:            1,
				PageSize:        20,
				ActorUserID:     "member-1",
				IsAuthenticated: true,
				UserRoleLevel:   item.userRoleLevel,
			})
			if searchErr != nil {
				t.Fatalf("search failed: %v", searchErr)
			}
			assertBleveSearchDocIDs(t, result, item.expectedDocID)
		})
	}
}

func TestBleveProvider_SearchEnforcesMinRoleForMemberDocsAcrossSpaces(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:test-bleve-provider-cross-space-min-role?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := prepareBleveProviderTestSchema(db); err != nil {
		t.Fatalf("prepare schema failed: %v", err)
	}
	if err := seedBleveProviderCrossSpaceMinRoleData(db); err != nil {
		t.Fatalf("seed cross space data failed: %v", err)
	}

	provider := NewBleveProvider(BleveProviderOptions{
		DB:        db,
		IndexPath: filepath.Join(t.TempDir(), "bleve-index-cross-space-min-role"),
	})
	ctx := context.Background()
	if err := provider.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema failed: %v", err)
	}

	now := time.Now().UTC().Unix()
	if err := provider.Upsert(ctx, []IndexRecord{
		{
			SpaceID:         "space-a",
			DocID:           "doc-space-a-min-role-1",
			NodeID:          "node-space-a-min-role-1",
			Title:           "跨空间权限文档 A1",
			BodyPlain:       "cross space min role A1",
			Terms:           "cross space min role",
			TitleTerms:      "cross space min role",
			VisibilityScope: string(models.VisibilityMember),
			MinRole:         1,
			UpdatedAtUnix:   now,
			SpaceStatus:     "active",
			DocStatus:       "active",
		},
		{
			SpaceID:         "space-a",
			DocID:           "doc-space-a-min-role-2",
			NodeID:          "node-space-a-min-role-2",
			Title:           "跨空间权限文档 A2",
			BodyPlain:       "cross space min role A2",
			Terms:           "cross space min role",
			TitleTerms:      "cross space min role",
			VisibilityScope: string(models.VisibilityMember),
			MinRole:         2,
			UpdatedAtUnix:   now,
			SpaceStatus:     "active",
			DocStatus:       "active",
		},
		{
			SpaceID:         "space-b",
			DocID:           "doc-space-b-min-role-1",
			NodeID:          "node-space-b-min-role-1",
			Title:           "跨空间权限文档 B1",
			BodyPlain:       "cross space min role B1",
			Terms:           "cross space min role",
			TitleTerms:      "cross space min role",
			VisibilityScope: string(models.VisibilityMember),
			MinRole:         1,
			UpdatedAtUnix:   now,
			SpaceStatus:     "active",
			DocStatus:       "active",
		},
		{
			SpaceID:         "space-b",
			DocID:           "doc-space-b-min-role-2",
			NodeID:          "node-space-b-min-role-2",
			Title:           "跨空间权限文档 B2",
			BodyPlain:       "cross space min role B2",
			Terms:           "cross space min role",
			TitleTerms:      "cross space min role",
			VisibilityScope: string(models.VisibilityMember),
			MinRole:         2,
			UpdatedAtUnix:   now,
			SpaceStatus:     "active",
			DocStatus:       "active",
		},
	}); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}

	result, searchErr := provider.Search(ctx, SearchRequest{
		Query:       "cross space min role",
		Page:        1,
		PageSize:    20,
		ActorUserID: "member-cross",
	})
	if searchErr != nil {
		t.Fatalf("search failed: %v", searchErr)
	}
	assertBleveSearchDocIDs(t, result, []string{
		"doc-space-a-min-role-1",
		"doc-space-b-min-role-1",
		"doc-space-b-min-role-2",
	})
}

func prepareBleveProviderTestSchema(db *gorm.DB) error {
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
  user_id TEXT,
  role TEXT
)`).Error; err != nil {
		return err
	}
	return nil
}

func seedBleveProviderTestData(db *gorm.DB) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := db.Exec(`
INSERT INTO spaces(space_id, owner_user_id, visibility, status, deleted_at, name) VALUES
  ('space-1', 'owner-1', 'public', 'active', NULL, '公开空间')
`).Error; err != nil {
		return err
	}
	if err := db.Exec(`
INSERT INTO nodes(node_id, space_id) VALUES
  ('node-1', 'space-1'),
  ('node-2', 'space-1'),
  ('node-min-role-1', 'space-1'),
  ('node-min-role-2', 'space-1'),
  ('node-min-role-3', 'space-1')
`).Error; err != nil {
		return err
	}
	if err := db.Exec(`
INSERT INTO documents(document_id, node_id, title, content_md, visibility, status, deleted_at, updated_at) VALUES
  ('doc-1', 'node-1', '检索命中文档', '这是公开文档', 'public', 'active', NULL, ?),
  ('doc-2', 'node-2', '成员命中文档', '这是成员文档', 'member', 'active', NULL, ?)
`, now, now).Error; err != nil {
		return err
	}
	if err := db.Exec(`
INSERT INTO space_members(space_id, user_id, role) VALUES
  ('space-1', 'member-1', 'reader')
`).Error; err != nil {
		return err
	}
	return nil
}

func seedBleveProviderCrossSpaceMinRoleData(db *gorm.DB) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := db.Exec(`
INSERT INTO spaces(space_id, owner_user_id, visibility, status, deleted_at, name) VALUES
  ('space-a', 'owner-a', 'member', 'active', NULL, 'Space A'),
  ('space-b', 'owner-b', 'member', 'active', NULL, 'Space B')
`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
INSERT INTO nodes(node_id, space_id) VALUES
  ('node-space-a-min-role-1', 'space-a'),
  ('node-space-a-min-role-2', 'space-a'),
  ('node-space-b-min-role-1', 'space-b'),
  ('node-space-b-min-role-2', 'space-b')
`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
INSERT INTO documents(document_id, node_id, title, content_md, visibility, status, deleted_at, updated_at) VALUES
  ('doc-space-a-min-role-1', 'node-space-a-min-role-1', '跨空间权限文档 A1', 'cross space min role A1', 'member', 'active', NULL, ?),
  ('doc-space-a-min-role-2', 'node-space-a-min-role-2', '跨空间权限文档 A2', 'cross space min role A2', 'member', 'active', NULL, ?),
  ('doc-space-b-min-role-1', 'node-space-b-min-role-1', '跨空间权限文档 B1', 'cross space min role B1', 'member', 'active', NULL, ?),
  ('doc-space-b-min-role-2', 'node-space-b-min-role-2', '跨空间权限文档 B2', 'cross space min role B2', 'member', 'active', NULL, ?)
`, now, now, now, now).Error; err != nil {
		return err
	}

	if err := db.Exec(`
INSERT INTO space_members(space_id, user_id, role) VALUES
  ('space-a', 'member-cross', 'reader'),
  ('space-b', 'member-cross', 'collaborator')
`).Error; err != nil {
		return err
	}
	return nil
}

func assertBleveSearchDocIDs(t *testing.T, result SearchResponse, expectedDocIDs []string) {
	t.Helper()

	expected := append([]string(nil), expectedDocIDs...)
	sort.Strings(expected)

	actual := make([]string, 0, len(result.Hits))
	for _, item := range result.Hits {
		actual = append(actual, item.DocID)
	}
	sort.Strings(actual)

	if int(result.Total) != len(expected) {
		t.Fatalf("expected total=%d, got=%d", len(expected), result.Total)
	}
	if len(actual) != len(expected) {
		t.Fatalf("expected hits=%v, got=%v", expected, actual)
	}
	for index := range expected {
		if actual[index] != expected[index] {
			t.Fatalf("expected hits=%v, got=%v", expected, actual)
		}
	}
}
