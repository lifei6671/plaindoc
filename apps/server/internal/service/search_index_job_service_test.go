package service

import (
	"context"
	"errors"
	"testing"

	searchprovider "github.com/lifei6671/plaindoc/apps/server/internal/search/provider"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
)

func TestSearchIndexJobService_RunOnce_Success(t *testing.T) {
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-search-index-job-service-success?mode=memory&cache=shared",
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
	if err := seedSearchIndexServiceFixture(database.ORM); err != nil {
		t.Fatalf("seed fixture failed: %v", err)
	}

	systemConfigRepo := repository.NewGormSystemConfigRepository(database.ORM)
	searchConfigService := NewSearchConfigService(systemConfigRepo, SearchConfigServiceOptions{})
	provider := &recordingSearchIndexProvider{}
	indexService := NewSearchIndexService(database.ORM, searchConfigService, provider)
	jobRepo := repository.NewGormSearchIndexJobRepository(database.ORM)
	jobService := NewSearchIndexJobService(jobRepo, indexService)

	jobParams, err := repository.BuildSearchIndexDocUpsertJob("search-doc-1")
	if err != nil {
		t.Fatalf("build search index job params failed: %v", err)
	}
	if err := jobRepo.Enqueue(ctx, jobParams); err != nil {
		t.Fatalf("enqueue job failed: %v", err)
	}

	runResult, err := jobService.RunOnce(ctx)
	if err != nil {
		t.Fatalf("run once failed: %v", err)
	}
	if runResult.Claimed != 1 || runResult.Succeeded != 1 || runResult.Retried != 0 {
		t.Fatalf("unexpected run result: %+v", runResult)
	}
	if provider.upsertCalls != 1 {
		t.Fatalf("expected provider upsert calls=1, got=%d", provider.upsertCalls)
	}
}

func TestSearchIndexJobService_RunOnce_RetryOnFailure(t *testing.T) {
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-search-index-job-service-retry?mode=memory&cache=shared",
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
	if err := seedSearchIndexServiceFixture(database.ORM); err != nil {
		t.Fatalf("seed fixture failed: %v", err)
	}

	systemConfigRepo := repository.NewGormSystemConfigRepository(database.ORM)
	searchConfigService := NewSearchConfigService(systemConfigRepo, SearchConfigServiceOptions{})
	provider := &recordingSearchIndexProvider{
		failUpsert: true,
	}
	indexService := NewSearchIndexService(database.ORM, searchConfigService, provider)
	jobRepo := repository.NewGormSearchIndexJobRepository(database.ORM)
	jobService := NewSearchIndexJobService(jobRepo, indexService)

	jobParams, err := repository.BuildSearchIndexDocUpsertJob("search-doc-1")
	if err != nil {
		t.Fatalf("build search index job params failed: %v", err)
	}
	if err := jobRepo.Enqueue(ctx, jobParams); err != nil {
		t.Fatalf("enqueue job failed: %v", err)
	}

	runResult, err := jobService.RunOnce(ctx)
	if err != nil {
		t.Fatalf("run once failed: %v", err)
	}
	if runResult.Claimed != 1 || runResult.Succeeded != 0 || runResult.Retried != 1 {
		t.Fatalf("unexpected run result: %+v", runResult)
	}

	type jobRow struct {
		Status     string `gorm:"column:status"`
		RetryCount int    `gorm:"column:retry_count"`
	}
	var row jobRow
	if err := database.ORM.WithContext(ctx).
		Table("search_index_jobs").
		Select("status", "retry_count").
		Where("dedupe_key = ?", "doc:search-doc-1").
		Take(&row).Error; err != nil {
		t.Fatalf("query search index job failed: %v", err)
	}
	if row.Status != models.SearchIndexJobStatusFailed {
		t.Fatalf("expected status failed, got %q", row.Status)
	}
	if row.RetryCount != 1 {
		t.Fatalf("expected retry_count=1, got %d", row.RetryCount)
	}
}

type recordingSearchIndexProvider struct {
	upsertCalls int
	deleteCalls int
	purgeCalls  int
	failUpsert  bool
}

func (p *recordingSearchIndexProvider) Name() string {
	return "bleve"
}

func (p *recordingSearchIndexProvider) Health(ctx context.Context) error {
	return nil
}

func (p *recordingSearchIndexProvider) Verify(ctx context.Context, config map[string]any) error {
	return nil
}

func (p *recordingSearchIndexProvider) EnsureSchema(ctx context.Context) error {
	return nil
}

func (p *recordingSearchIndexProvider) Upsert(
	ctx context.Context,
	records []searchprovider.IndexRecord,
) error {
	p.upsertCalls += 1
	if p.failUpsert {
		return errors.New("mock upsert failed")
	}
	return nil
}

func (p *recordingSearchIndexProvider) Delete(ctx context.Context, docIDs []string) error {
	p.deleteCalls += 1
	return nil
}

func (p *recordingSearchIndexProvider) PurgeBySpace(ctx context.Context, spaceID string) error {
	p.purgeCalls += 1
	return nil
}

func (p *recordingSearchIndexProvider) Search(
	ctx context.Context,
	request searchprovider.SearchRequest,
) (searchprovider.SearchResponse, error) {
	return searchprovider.SearchResponse{}, nil
}

func (p *recordingSearchIndexProvider) Capabilities() searchprovider.Capabilities {
	return searchprovider.Capabilities{}
}
