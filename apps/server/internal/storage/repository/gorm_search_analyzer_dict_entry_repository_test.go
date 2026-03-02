package repository

import (
	"context"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
)

func TestGormSearchAnalyzerDictEntryRepository_ListActiveByAnalyzer(t *testing.T) {
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-search-analyzer-dict-entry-repository?mode=memory&cache=shared",
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
	weight200 := 200
	if err := database.ORM.WithContext(ctx).Create(&models.SearchAnalyzerDictEntry{
		Analyzer:  "jieba",
		Term:      "微服务架构",
		Weight:    &weight200,
		Tag:       "n",
		Status:    models.SearchAnalyzerDictEntryStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed active jieba dict entry failed: %v", err)
	}
	if err := database.ORM.WithContext(ctx).Create(&models.SearchAnalyzerDictEntry{
		Analyzer:  "jieba",
		Term:      "逻辑删除词条",
		Status:    models.SearchAnalyzerDictEntryStatusDeleted,
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed deleted jieba dict entry failed: %v", err)
	}
	if err := database.ORM.WithContext(ctx).Create(&models.SearchAnalyzerDictEntry{
		Analyzer:  "simple",
		Term:      "simple词条",
		Status:    models.SearchAnalyzerDictEntryStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed simple dict entry failed: %v", err)
	}

	repo := NewGormSearchAnalyzerDictEntryRepository(database.ORM)
	items, err := repo.ListActiveByAnalyzer(ctx, "jieba")
	if err != nil {
		t.Fatalf("list active by analyzer failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 active jieba entry, got %d", len(items))
	}

	item := items[0]
	if item.Analyzer != "jieba" {
		t.Fatalf("expected analyzer jieba, got %q", item.Analyzer)
	}
	if item.Term != "微服务架构" {
		t.Fatalf("expected term 微服务架构, got %q", item.Term)
	}
	if item.Weight == nil || *item.Weight != 200 {
		t.Fatalf("expected weight 200, got %+v", item.Weight)
	}
	if item.Tag != "n" {
		t.Fatalf("expected tag n, got %q", item.Tag)
	}
	if item.Status != models.SearchAnalyzerDictEntryStatusActive {
		t.Fatalf("expected status active, got %q", item.Status)
	}
	if item.CreatedAt.IsZero() || item.UpdatedAt.IsZero() {
		t.Fatalf("expected non-zero timestamps, got created=%v updated=%v", item.CreatedAt, item.UpdatedAt)
	}
}

func TestGormSearchAnalyzerDictEntryRepository_ListActiveByAnalyzer_EmptyAnalyzer(t *testing.T) {
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-search-analyzer-dict-entry-repository-empty?mode=memory&cache=shared",
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

	repo := NewGormSearchAnalyzerDictEntryRepository(database.ORM)
	items, err := repo.ListActiveByAnalyzer(ctx, "   ")
	if err != nil {
		t.Fatalf("list active by analyzer failed: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty entries, got %d", len(items))
	}
}
