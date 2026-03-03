package provider

import (
	"context"
	"path/filepath"
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
  user_id TEXT
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
  ('node-2', 'space-1')
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
INSERT INTO space_members(space_id, user_id) VALUES
  ('space-1', 'member-1')
`).Error; err != nil {
		return err
	}
	return nil
}
