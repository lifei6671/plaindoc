package service

import (
	"context"
	"testing"
	"time"

	searchprovider "github.com/lifei6671/plaindoc/apps/server/internal/search/provider"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

func TestSearchIndexService_RebuildActiveProvider(t *testing.T) {
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-search-index-service?mode=memory&cache=shared",
	})
	if err != nil {
		t.Fatalf("open database failed: %v", err)
	}
	defer func() {
		_ = database.Close()
	}()

	if err := storage.MigrateUp(context.Background(), database.ORM, storage.DriverSQLite); err != nil {
		t.Fatalf("migrate up failed: %v", err)
	}
	if err := seedSearchIndexServiceFixture(database.ORM); err != nil {
		t.Fatalf("seed fixture failed: %v", err)
	}

	systemConfigRepo := repository.NewGormSystemConfigRepository(database.ORM)
	searchConfigService := NewSearchConfigService(systemConfigRepo, SearchConfigServiceOptions{})
	bleveProvider := searchprovider.NewBleveProvider(searchprovider.BleveProviderOptions{
		DB:        database.ORM,
		IndexPath: t.TempDir() + "/bleve",
	})
	indexService := NewSearchIndexService(
		database.ORM,
		searchConfigService,
		bleveProvider,
	)

	rebuildResult, err := indexService.RebuildActiveProvider(context.Background())
	if err != nil {
		t.Fatalf("rebuild active provider failed: %v", err)
	}
	if rebuildResult.Provider != "bleve" {
		t.Fatalf("expected provider=bleve, got=%q", rebuildResult.Provider)
	}
	if rebuildResult.IndexedDocuments != 1 {
		t.Fatalf("expected indexedDocuments=1, got=%d", rebuildResult.IndexedDocuments)
	}

	searchResult, err := bleveProvider.Search(context.Background(), searchprovider.SearchRequest{
		Query: "检 索",
		Page:  1,
	})
	if err != nil {
		t.Fatalf("search after rebuild failed: %v", err)
	}
	if searchResult.Total != 1 {
		t.Fatalf("expected total=1 after rebuild, got=%d", searchResult.Total)
	}
	if len(searchResult.Hits) != 1 || searchResult.Hits[0].DocID != "search-doc-1" {
		t.Fatalf("expected hit search-doc-1, got=%+v", searchResult.Hits)
	}
}

func seedSearchIndexServiceFixture(dbORM *gorm.DB) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)

	if err := dbORM.Table("users").Create(map[string]any{
		"user_id":       "search-owner-1",
		"email":         "search-owner@example.com",
		"password_hash": "hash",
		"name":          "Search Owner",
		"status":        "active",
		"created_at":    now,
		"updated_at":    now,
	}).Error; err != nil {
		return err
	}
	if err := dbORM.Table("spaces").Create(map[string]any{
		"space_id":      "search-space-1",
		"name":          "搜索空间",
		"owner_user_id": "search-owner-1",
		"visibility":    "public",
		"status":        "active",
		"created_at":    now,
		"updated_at":    now,
	}).Error; err != nil {
		return err
	}
	if err := dbORM.Table("nodes").Create(map[string]any{
		"node_id":        "search-node-1",
		"space_id":       "search-space-1",
		"parent_node_id": nil,
		"type":           "doc",
		"title":          "检索文档",
		"sort":           1,
		"created_at":     now,
		"updated_at":     now,
	}).Error; err != nil {
		return err
	}
	if err := dbORM.Table("documents").Create(map[string]any{
		"document_id": "search-doc-1",
		"node_id":     "search-node-1",
		"theme_id":    "default",
		"visibility":  "public",
		"status":      "active",
		"title":       "检索命中文档",
		"content_md":  "# 检索内容",
		"version":     1,
		"created_at":  now,
		"updated_at":  now,
	}).Error; err != nil {
		return err
	}
	if err := dbORM.Table("system_configs").Create(map[string]any{
		"config_key":         "search",
		"config_value_json":  `{"enabled":true,"activeProvider":"bleve","fallbackPolicy":"degrade_to_bleve","analysis":{"activeAnalyzer":"simple","analyzers":{"simple":{"enabled":true},"jieba":{"enabled":false,"mode":"search","hmm":true,"stopwordsEnabled":false,"dictSource":"db","dictVersion":"default"}}}}`,
		"version":            1,
		"updated_by_user_id": nil,
		"created_at":         now,
		"updated_at":         now,
	}).Error; err != nil {
		return err
	}
	return nil
}
