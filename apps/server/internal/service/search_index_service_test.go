package service

import (
	"context"
	"errors"
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

func TestSearchIndexService_Status(t *testing.T) {
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-search-index-status?mode=memory&cache=shared",
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
		IndexPath: t.TempDir() + "/bleve-status",
	})
	indexService := NewSearchIndexService(
		database.ORM,
		searchConfigService,
		bleveProvider,
	)

	statusBeforeRebuild, err := indexService.Status(context.Background())
	if err != nil {
		t.Fatalf("status before rebuild failed: %v", err)
	}
	if !statusBeforeRebuild.Enabled {
		t.Fatal("expected enabled=true before rebuild")
	}
	if statusBeforeRebuild.ActiveProvider != "bleve" {
		t.Fatalf("expected active provider bleve, got=%q", statusBeforeRebuild.ActiveProvider)
	}
	if statusBeforeRebuild.EffectiveProvider != "bleve" {
		t.Fatalf("expected effective provider bleve, got=%q", statusBeforeRebuild.EffectiveProvider)
	}
	if !statusBeforeRebuild.SupportsDocCount {
		t.Fatal("expected supportsDocCount=true before rebuild")
	}
	if !statusBeforeRebuild.ProviderHealthy {
		t.Fatalf("expected provider healthy before rebuild, message=%q", statusBeforeRebuild.ProviderMessage)
	}

	if _, err := indexService.RebuildActiveProvider(context.Background()); err != nil {
		t.Fatalf("rebuild active provider failed: %v", err)
	}

	statusAfterRebuild, err := indexService.Status(context.Background())
	if err != nil {
		t.Fatalf("status after rebuild failed: %v", err)
	}
	if statusAfterRebuild.LastRebuildAt == nil {
		t.Fatal("expected last rebuild time after rebuild")
	}
	if statusAfterRebuild.LastRebuildSource != "manual" {
		t.Fatalf("expected last rebuild source manual, got=%q", statusAfterRebuild.LastRebuildSource)
	}
	if statusAfterRebuild.LastRebuildIndexedDocuments != 1 {
		t.Fatalf("expected last rebuild indexed docs 1, got=%d", statusAfterRebuild.LastRebuildIndexedDocuments)
	}
	if statusAfterRebuild.IndexedDocuments != 1 {
		t.Fatalf("expected indexed documents 1 after rebuild, got=%d", statusAfterRebuild.IndexedDocuments)
	}
}

func TestSearchIndexService_Status_UpdatedAfterIncrementalSync(t *testing.T) {
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-search-index-status-sync-updates?mode=memory&cache=shared",
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
		IndexPath: t.TempDir() + "/bleve-sync-status",
	})
	indexService := NewSearchIndexService(
		database.ORM,
		searchConfigService,
		bleveProvider,
	)

	if err := indexService.SyncDocumentByID(context.Background(), "search-doc-1"); err != nil {
		t.Fatalf("sync document by id failed: %v", err)
	}
	statusAfterSyncDoc, err := indexService.Status(context.Background())
	if err != nil {
		t.Fatalf("status after sync document failed: %v", err)
	}
	if statusAfterSyncDoc.LastRebuildAt == nil {
		t.Fatal("expected last rebuild at after sync document")
	}
	if statusAfterSyncDoc.LastRebuildSource != searchIndexRebuildSourceSyncDoc {
		t.Fatalf(
			"expected source=%q after sync document, got=%q",
			searchIndexRebuildSourceSyncDoc,
			statusAfterSyncDoc.LastRebuildSource,
		)
	}
	if statusAfterSyncDoc.LastRebuildIndexedDocuments != 1 {
		t.Fatalf(
			"expected last rebuild indexed docs 1 after sync document, got=%d",
			statusAfterSyncDoc.LastRebuildIndexedDocuments,
		)
	}

	if err := indexService.DeleteDocumentByID(context.Background(), "search-doc-1"); err != nil {
		t.Fatalf("delete document by id failed: %v", err)
	}
	statusAfterDeleteDoc, err := indexService.Status(context.Background())
	if err != nil {
		t.Fatalf("status after delete document failed: %v", err)
	}
	if statusAfterDeleteDoc.LastRebuildSource != searchIndexRebuildSourceDeleteDoc {
		t.Fatalf(
			"expected source=%q after delete document, got=%q",
			searchIndexRebuildSourceDeleteDoc,
			statusAfterDeleteDoc.LastRebuildSource,
		)
	}
	if statusAfterDeleteDoc.LastRebuildIndexedDocuments != 0 {
		t.Fatalf(
			"expected last rebuild indexed docs 0 after delete document, got=%d",
			statusAfterDeleteDoc.LastRebuildIndexedDocuments,
		)
	}
	if statusAfterDeleteDoc.IndexedDocuments != 0 {
		t.Fatalf("expected indexed documents 0 after delete document, got=%d", statusAfterDeleteDoc.IndexedDocuments)
	}

	if err := indexService.SyncSpaceByID(context.Background(), "search-space-1"); err != nil {
		t.Fatalf("sync space by id failed: %v", err)
	}
	statusAfterSyncSpace, err := indexService.Status(context.Background())
	if err != nil {
		t.Fatalf("status after sync space failed: %v", err)
	}
	if statusAfterSyncSpace.LastRebuildSource != searchIndexRebuildSourceSyncSpace {
		t.Fatalf(
			"expected source=%q after sync space, got=%q",
			searchIndexRebuildSourceSyncSpace,
			statusAfterSyncSpace.LastRebuildSource,
		)
	}
	if statusAfterSyncSpace.LastRebuildIndexedDocuments != 1 {
		t.Fatalf(
			"expected last rebuild indexed docs 1 after sync space, got=%d",
			statusAfterSyncSpace.LastRebuildIndexedDocuments,
		)
	}
	if statusAfterSyncSpace.IndexedDocuments != 1 {
		t.Fatalf("expected indexed documents 1 after sync space, got=%d", statusAfterSyncSpace.IndexedDocuments)
	}

	if err := indexService.PurgeSpaceByID(context.Background(), "search-space-1"); err != nil {
		t.Fatalf("purge space by id failed: %v", err)
	}
	statusAfterPurgeSpace, err := indexService.Status(context.Background())
	if err != nil {
		t.Fatalf("status after purge space failed: %v", err)
	}
	if statusAfterPurgeSpace.LastRebuildSource != searchIndexRebuildSourcePurgeSpace {
		t.Fatalf(
			"expected source=%q after purge space, got=%q",
			searchIndexRebuildSourcePurgeSpace,
			statusAfterPurgeSpace.LastRebuildSource,
		)
	}
	if statusAfterPurgeSpace.LastRebuildIndexedDocuments != 0 {
		t.Fatalf(
			"expected last rebuild indexed docs 0 after purge space, got=%d",
			statusAfterPurgeSpace.LastRebuildIndexedDocuments,
		)
	}
	if statusAfterPurgeSpace.IndexedDocuments != 0 {
		t.Fatalf("expected indexed documents 0 after purge space, got=%d", statusAfterPurgeSpace.IndexedDocuments)
	}
}

func TestSearchIndexService_Status_RebuildInProgress(t *testing.T) {
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-search-index-status-rebuilding?mode=memory&cache=shared",
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
	if err := database.ORM.Table("system_configs").
		Where("config_key = ?", "search").
		Update("config_value_json", `{"enabled":true,"activeProvider":"database","fallbackPolicy":"degrade_to_database","analysis":{"activeAnalyzer":"simple","analyzers":{"simple":{"enabled":true},"jieba":{"enabled":false,"mode":"search","hmm":true,"stopwordsEnabled":false,"dictSource":"db","dictVersion":"default"}}}}`).
		Error; err != nil {
		t.Fatalf("update search config failed: %v", err)
	}

	systemConfigRepo := repository.NewGormSystemConfigRepository(database.ORM)
	searchConfigService := NewSearchConfigService(systemConfigRepo, SearchConfigServiceOptions{})
	blockingProvider := newBlockingSearchIndexProvider()
	indexService := NewSearchIndexService(
		database.ORM,
		searchConfigService,
		blockingProvider,
	)

	errChan := make(chan error, 1)
	go func() {
		_, rebuildErr := indexService.RebuildActiveProvider(context.Background())
		errChan <- rebuildErr
	}()

	select {
	case <-blockingProvider.started:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for rebuild upsert start")
	}

	statusWhileRunning, err := indexService.Status(context.Background())
	if err != nil {
		t.Fatalf("status while rebuild running failed: %v", err)
	}
	if !statusWhileRunning.RebuildInProgress {
		t.Fatal("expected rebuildInProgress=true while rebuild is running")
	}

	close(blockingProvider.release)
	select {
	case rebuildErr := <-errChan:
		if rebuildErr != nil {
			t.Fatalf("rebuild failed: %v", rebuildErr)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for rebuild complete")
	}

	statusAfterRunning, err := indexService.Status(context.Background())
	if err != nil {
		t.Fatalf("status after rebuild failed: %v", err)
	}
	if statusAfterRunning.RebuildInProgress {
		t.Fatal("expected rebuildInProgress=false after rebuild")
	}
}

func TestSearchIndexService_RebuildActiveProvider_RejectsConcurrentRebuild(t *testing.T) {
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-search-index-concurrent-rebuild?mode=memory&cache=shared",
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
	if err := database.ORM.Table("system_configs").
		Where("config_key = ?", "search").
		Update("config_value_json", `{"enabled":true,"activeProvider":"database","fallbackPolicy":"degrade_to_database","analysis":{"activeAnalyzer":"simple","analyzers":{"simple":{"enabled":true},"jieba":{"enabled":false,"mode":"search","hmm":true,"stopwordsEnabled":false,"dictSource":"db","dictVersion":"default"}}}}`).
		Error; err != nil {
		t.Fatalf("update search config failed: %v", err)
	}

	systemConfigRepo := repository.NewGormSystemConfigRepository(database.ORM)
	searchConfigService := NewSearchConfigService(systemConfigRepo, SearchConfigServiceOptions{})
	blockingProvider := newBlockingSearchIndexProvider()
	indexService := NewSearchIndexService(
		database.ORM,
		searchConfigService,
		blockingProvider,
	)

	errChan := make(chan error, 1)
	go func() {
		_, rebuildErr := indexService.RebuildActiveProvider(context.Background())
		errChan <- rebuildErr
	}()

	select {
	case <-blockingProvider.started:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for first rebuild start")
	}

	if _, concurrentErr := indexService.RebuildActiveProvider(context.Background()); !errors.Is(
		concurrentErr,
		ErrSearchIndexRebuildInProgress,
	) {
		t.Fatalf("expected ErrSearchIndexRebuildInProgress, got %v", concurrentErr)
	}

	close(blockingProvider.release)
	select {
	case rebuildErr := <-errChan:
		if rebuildErr != nil {
			t.Fatalf("first rebuild failed: %v", rebuildErr)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for first rebuild complete")
	}
}

func TestSearchIndexService_EnqueueSyncDocumentByID_NonBlocking(t *testing.T) {
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-search-index-enqueue-sync-doc?mode=memory&cache=shared",
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
	provider := newBlockingIncrementalSearchIndexProvider()
	indexService := NewSearchIndexService(
		database.ORM,
		searchConfigService,
		provider,
	)

	startAt := time.Now()
	if err := indexService.EnqueueSyncDocumentByID("search-doc-1"); err != nil {
		t.Fatalf("enqueue sync document failed: %v", err)
	}
	if elapsed := time.Since(startAt); elapsed > 200*time.Millisecond {
		t.Fatalf("expected enqueue call to be non-blocking, elapsed=%s", elapsed)
	}

	select {
	case <-provider.upsertStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for async sync-document task start")
	}

	close(provider.upsertRelease)

	waitForSearchIndexStatus(t, 3*time.Second, 50*time.Millisecond, func(status SearchIndexStatusResult) bool {
		return status.LastRebuildSource == searchIndexRebuildSourceSyncDoc
	}, func(status SearchIndexStatusResult) string {
		return "last source=" + status.LastRebuildSource
	}, indexService)
}

func TestSearchIndexService_EnqueueDeleteDocumentByID_NonBlocking(t *testing.T) {
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-search-index-enqueue-delete-doc?mode=memory&cache=shared",
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
	provider := newBlockingIncrementalSearchIndexProvider()
	indexService := NewSearchIndexService(
		database.ORM,
		searchConfigService,
		provider,
	)

	startAt := time.Now()
	if err := indexService.EnqueueDeleteDocumentByID("search-doc-1"); err != nil {
		t.Fatalf("enqueue delete document failed: %v", err)
	}
	if elapsed := time.Since(startAt); elapsed > 200*time.Millisecond {
		t.Fatalf("expected enqueue call to be non-blocking, elapsed=%s", elapsed)
	}

	select {
	case <-provider.deleteStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for async delete-document task start")
	}

	close(provider.deleteRelease)

	waitForSearchIndexStatus(t, 3*time.Second, 50*time.Millisecond, func(status SearchIndexStatusResult) bool {
		return status.LastRebuildSource == searchIndexRebuildSourceDeleteDoc
	}, func(status SearchIndexStatusResult) string {
		return "last source=" + status.LastRebuildSource
	}, indexService)
}

type blockingSearchIndexProvider struct {
	started chan struct{}
	release chan struct{}
}

func newBlockingSearchIndexProvider() *blockingSearchIndexProvider {
	return &blockingSearchIndexProvider{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (p *blockingSearchIndexProvider) Name() string {
	return "database"
}

func (p *blockingSearchIndexProvider) Health(ctx context.Context) error {
	return nil
}

func (p *blockingSearchIndexProvider) Verify(ctx context.Context, config map[string]any) error {
	return nil
}

func (p *blockingSearchIndexProvider) EnsureSchema(ctx context.Context) error {
	return nil
}

func (p *blockingSearchIndexProvider) Upsert(
	ctx context.Context,
	records []searchprovider.IndexRecord,
) error {
	select {
	case <-p.started:
	default:
		close(p.started)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.release:
		return nil
	}
}

func (p *blockingSearchIndexProvider) Delete(ctx context.Context, docIDs []string) error {
	return nil
}

func (p *blockingSearchIndexProvider) PurgeBySpace(ctx context.Context, spaceID string) error {
	return nil
}

func (p *blockingSearchIndexProvider) Search(
	ctx context.Context,
	request searchprovider.SearchRequest,
) (searchprovider.SearchResponse, error) {
	return searchprovider.SearchResponse{}, nil
}

func (p *blockingSearchIndexProvider) Capabilities() searchprovider.Capabilities {
	return searchprovider.Capabilities{}
}

type blockingIncrementalSearchIndexProvider struct {
	upsertStarted chan struct{}
	upsertRelease chan struct{}
	deleteStarted chan struct{}
	deleteRelease chan struct{}
}

func newBlockingIncrementalSearchIndexProvider() *blockingIncrementalSearchIndexProvider {
	return &blockingIncrementalSearchIndexProvider{
		upsertStarted: make(chan struct{}),
		upsertRelease: make(chan struct{}),
		deleteStarted: make(chan struct{}),
		deleteRelease: make(chan struct{}),
	}
}

func (p *blockingIncrementalSearchIndexProvider) Name() string {
	return "bleve"
}

func (p *blockingIncrementalSearchIndexProvider) Health(ctx context.Context) error {
	return nil
}

func (p *blockingIncrementalSearchIndexProvider) Verify(ctx context.Context, config map[string]any) error {
	return nil
}

func (p *blockingIncrementalSearchIndexProvider) EnsureSchema(ctx context.Context) error {
	return nil
}

func (p *blockingIncrementalSearchIndexProvider) Upsert(
	ctx context.Context,
	records []searchprovider.IndexRecord,
) error {
	select {
	case <-p.upsertStarted:
	default:
		close(p.upsertStarted)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.upsertRelease:
		return nil
	}
}

func (p *blockingIncrementalSearchIndexProvider) Delete(ctx context.Context, docIDs []string) error {
	select {
	case <-p.deleteStarted:
	default:
		close(p.deleteStarted)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.deleteRelease:
		return nil
	}
}

func (p *blockingIncrementalSearchIndexProvider) PurgeBySpace(ctx context.Context, spaceID string) error {
	return nil
}

func (p *blockingIncrementalSearchIndexProvider) Search(
	ctx context.Context,
	request searchprovider.SearchRequest,
) (searchprovider.SearchResponse, error) {
	return searchprovider.SearchResponse{}, nil
}

func (p *blockingIncrementalSearchIndexProvider) Capabilities() searchprovider.Capabilities {
	return searchprovider.Capabilities{}
}

func waitForSearchIndexStatus(
	t *testing.T,
	timeout time.Duration,
	interval time.Duration,
	condition func(status SearchIndexStatusResult) bool,
	describe func(status SearchIndexStatusResult) string,
	indexService *SearchIndexService,
) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	lastStatus, statusErr := indexService.Status(context.Background())
	if statusErr != nil {
		t.Fatalf("read search index status failed: %v", statusErr)
	}
	for {
		if condition(lastStatus) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting status condition: %s", describe(lastStatus))
		}
		time.Sleep(interval)
		lastStatus, statusErr = indexService.Status(context.Background())
		if statusErr != nil {
			t.Fatalf("read search index status failed: %v", statusErr)
		}
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
