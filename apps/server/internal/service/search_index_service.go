package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	searchcfg "github.com/lifei6671/plaindoc/apps/server/internal/search"
	searchanalyzer "github.com/lifei6671/plaindoc/apps/server/internal/search/analyzer"
	searchprovider "github.com/lifei6671/plaindoc/apps/server/internal/search/provider"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

const defaultSearchIndexRebuildBatchSize = 200
const defaultSearchIndexAsyncQueueSize = 1024
const defaultSearchIndexAsyncTaskTimeout = 45 * time.Second

const (
	searchIndexRebuildSourceBootstrap  = "bootstrap"
	searchIndexRebuildSourceManual     = "manual"
	searchIndexRebuildSourceSyncDoc    = "sync_document"
	searchIndexRebuildSourceDeleteDoc  = "delete_document"
	searchIndexRebuildSourceSyncSpace  = "sync_space"
	searchIndexRebuildSourcePurgeSpace = "purge_space"
)

var (
	// ErrSearchIndexRebuildInProgress 表示当前已有索引重建任务在执行。
	ErrSearchIndexRebuildInProgress = errors.New("search index rebuild is already in progress")
	// ErrSearchIndexTaskQueueFull 表示异步索引任务队列已满。
	ErrSearchIndexTaskQueueFull = errors.New("search index task queue is full")
)

// SearchIndexBootstrapResult 表示索引启动阶段结果。
type SearchIndexBootstrapResult struct {
	Provider         searchcfg.ProviderName
	Rebuilt          bool
	IndexedDocuments int
}

// SearchIndexRebuildResult 表示全量重建结果。
type SearchIndexRebuildResult struct {
	Provider         searchcfg.ProviderName
	IndexedDocuments int
}

// SearchIndexStatusResult 表示当前索引运行状态。
type SearchIndexStatusResult struct {
	Enabled                     bool
	ActiveProvider              searchcfg.ProviderName
	EffectiveProvider           searchcfg.ProviderName
	FallbackPolicy              searchcfg.FallbackPolicy
	ActiveAnalyzer              searchcfg.AnalyzerName
	RebuildInProgress           bool
	ProviderHealthy             bool
	ProviderMessage             string
	SupportsDocCount            bool
	IndexedDocuments            int64
	LastRebuildAt               *time.Time
	LastRebuildSource           string
	LastRebuildIndexedDocuments int
}

// SearchIndexService 负责检索引擎索引创建与全量重建。
type SearchIndexService struct {
	db                    *gorm.DB
	searchConfigService   *SearchConfigService
	providers             map[searchcfg.ProviderName]searchprovider.Provider
	searchIndexSourceRepo repository.SearchIndexSourceRepository
	searchIndexJobRepo    repository.SearchIndexJobRepository

	asyncTaskQueue      chan searchIndexAsyncTask
	asyncTaskWorkerOnce sync.Once

	runtimeMu                   sync.RWMutex
	rebuildInProgress           bool
	lastRebuildAt               time.Time
	lastRebuildSource           string
	lastRebuildIndexedDocuments int
}

type searchIndexAsyncTaskType string

const (
	searchIndexAsyncTaskTypeSyncDocument   searchIndexAsyncTaskType = "sync_document"
	searchIndexAsyncTaskTypeDeleteDocument searchIndexAsyncTaskType = "delete_document"
)

type searchIndexAsyncTask struct {
	taskType   searchIndexAsyncTaskType
	documentID string
}

// NewSearchIndexService 创建索引服务。
func NewSearchIndexService(
	db *gorm.DB,
	searchConfigService *SearchConfigService,
	providers ...searchprovider.Provider,
) *SearchIndexService {
	providerMap := make(map[searchcfg.ProviderName]searchprovider.Provider, len(providers))
	for _, item := range providers {
		if item == nil {
			continue
		}
		name := searchcfg.ProviderName(strings.ToLower(strings.TrimSpace(item.Name())))
		if name == "" {
			continue
		}
		providerMap[name] = item
	}
	return &SearchIndexService{
		db:                    db,
		searchConfigService:   searchConfigService,
		providers:             providerMap,
		searchIndexSourceRepo: repository.NewGormSearchIndexSourceRepository(db),
		asyncTaskQueue:        make(chan searchIndexAsyncTask, defaultSearchIndexAsyncQueueSize),
	}
}

// Bootstrap 创建索引结构，并在空索引时执行一次全量重建。
func (s *SearchIndexService) Bootstrap(ctx context.Context) (SearchIndexBootstrapResult, error) {
	if s == nil || s.db == nil || s.searchConfigService == nil {
		return SearchIndexBootstrapResult{}, errors.New("search index service dependencies are nil")
	}
	if ctx == nil {
		return SearchIndexBootstrapResult{}, errors.New("search index bootstrap context is nil")
	}

	snapshot, err := s.searchConfigService.Refresh(ctx)
	if err != nil {
		return SearchIndexBootstrapResult{}, err
	}
	if !snapshot.Config.Enabled {
		return SearchIndexBootstrapResult{}, nil
	}

	providerName, providerInstance, err := s.resolveProvider(snapshot.Config)
	if err != nil {
		return SearchIndexBootstrapResult{}, err
	}
	if err := providerInstance.EnsureSchema(ctx); err != nil {
		return SearchIndexBootstrapResult{}, err
	}

	if statsProvider, ok := providerInstance.(searchprovider.IndexStatsProvider); ok {
		docCount, countErr := statsProvider.DocCount(ctx)
		if countErr != nil {
			return SearchIndexBootstrapResult{}, countErr
		}
		if docCount > 0 {
			return SearchIndexBootstrapResult{
				Provider:         providerName,
				Rebuilt:          false,
				IndexedDocuments: int(docCount),
			}, nil
		}
	}

	rebuildResult, err := s.rebuildWithSnapshot(
		ctx,
		snapshot,
		providerName,
		providerInstance,
		searchIndexRebuildSourceBootstrap,
	)
	if err != nil {
		return SearchIndexBootstrapResult{}, err
	}
	return SearchIndexBootstrapResult{
		Provider:         rebuildResult.Provider,
		Rebuilt:          true,
		IndexedDocuments: rebuildResult.IndexedDocuments,
	}, nil
}

// RebuildActiveProvider 触发当前 active provider 全量重建。
func (s *SearchIndexService) RebuildActiveProvider(ctx context.Context) (SearchIndexRebuildResult, error) {
	if s == nil || s.db == nil || s.searchConfigService == nil {
		return SearchIndexRebuildResult{}, errors.New("search index service dependencies are nil")
	}
	if ctx == nil {
		return SearchIndexRebuildResult{}, errors.New("search index rebuild context is nil")
	}

	snapshot, err := s.searchConfigService.Refresh(ctx)
	if err != nil {
		return SearchIndexRebuildResult{}, err
	}
	if !snapshot.Config.Enabled {
		return SearchIndexRebuildResult{}, ErrSearchDisabled
	}
	providerName, providerInstance, err := s.resolveProvider(snapshot.Config)
	if err != nil {
		return SearchIndexRebuildResult{}, err
	}
	return s.rebuildWithSnapshot(
		ctx,
		snapshot,
		providerName,
		providerInstance,
		searchIndexRebuildSourceManual,
	)
}

// EnqueueSyncDocumentByID 异步入队按文档 ID 增量同步索引任务。
func (s *SearchIndexService) EnqueueSyncDocumentByID(documentID string) error {
	if s == nil || s.db == nil || s.searchConfigService == nil {
		return errors.New("search index service dependencies are nil")
	}

	normalizedDocumentID := strings.TrimSpace(documentID)
	if normalizedDocumentID == "" {
		return nil
	}
	if s.searchIndexJobRepo != nil {
		params, err := repository.BuildSearchIndexDocUpsertJob(normalizedDocumentID)
		if err != nil {
			return err
		}
		return s.searchIndexJobRepo.Enqueue(context.Background(), params)
	}
	return s.enqueueTask(searchIndexAsyncTask{
		taskType:   searchIndexAsyncTaskTypeSyncDocument,
		documentID: normalizedDocumentID,
	})
}

// EnqueueDeleteDocumentByID 异步入队按文档 ID 删除索引任务。
func (s *SearchIndexService) EnqueueDeleteDocumentByID(documentID string) error {
	if s == nil || s.db == nil || s.searchConfigService == nil {
		return errors.New("search index service dependencies are nil")
	}

	normalizedDocumentID := strings.TrimSpace(documentID)
	if normalizedDocumentID == "" {
		return nil
	}
	if s.searchIndexJobRepo != nil {
		params, err := repository.BuildSearchIndexDocDeleteJob(normalizedDocumentID)
		if err != nil {
			return err
		}
		return s.searchIndexJobRepo.Enqueue(context.Background(), params)
	}
	return s.enqueueTask(searchIndexAsyncTask{
		taskType:   searchIndexAsyncTaskTypeDeleteDocument,
		documentID: normalizedDocumentID,
	})
}

// SetSearchIndexJobRepository 注入检索任务仓储，用于 outbox 入队。
func (s *SearchIndexService) SetSearchIndexJobRepository(
	jobRepo repository.SearchIndexJobRepository,
) {
	if s == nil {
		return
	}
	s.searchIndexJobRepo = jobRepo
}

// SetSearchIndexSourceRepository 注入索引源仓储，用于查询可索引文档快照。
func (s *SearchIndexService) SetSearchIndexSourceRepository(
	searchIndexSourceRepo repository.SearchIndexSourceRepository,
) {
	if s == nil {
		return
	}
	s.searchIndexSourceRepo = searchIndexSourceRepo
}

// SyncDocumentByID 按文档 ID 增量同步索引（存在则 upsert，不存在或不可索引则删除）。
func (s *SearchIndexService) SyncDocumentByID(ctx context.Context, documentID string) error {
	if s == nil || s.db == nil || s.searchConfigService == nil {
		return errors.New("search index service dependencies are nil")
	}
	if ctx == nil {
		return errors.New("search index sync context is nil")
	}

	normalizedDocumentID := strings.TrimSpace(documentID)
	if normalizedDocumentID == "" {
		return nil
	}

	snapshot, err := s.searchConfigService.Refresh(ctx)
	if err != nil {
		return err
	}
	if !snapshot.Config.Enabled {
		return nil
	}

	providerName, providerInstance, err := s.resolveProvider(snapshot.Config)
	if err != nil {
		return err
	}
	if providerName == searchcfg.ProviderDatabase {
		return nil
	}
	if err := providerInstance.EnsureSchema(ctx); err != nil {
		return err
	}

	row, err := s.loadActiveDocumentForSync(ctx, normalizedDocumentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := providerInstance.Delete(ctx, []string{normalizedDocumentID}); err != nil {
				return err
			}
			s.setRebuildRuntimeState(searchIndexRebuildSourceDeleteDoc, 0)
			return nil
		}
		return err
	}

	record, err := buildSearchIndexRecord(ctx, snapshot, row)
	if err != nil {
		return err
	}
	if err := providerInstance.Upsert(ctx, []searchprovider.IndexRecord{record}); err != nil {
		return err
	}
	s.setRebuildRuntimeState(searchIndexRebuildSourceSyncDoc, 1)
	return nil
}

// DeleteDocumentByID 按文档 ID 删除索引记录。
func (s *SearchIndexService) DeleteDocumentByID(ctx context.Context, documentID string) error {
	if s == nil || s.db == nil || s.searchConfigService == nil {
		return errors.New("search index service dependencies are nil")
	}
	if ctx == nil {
		return errors.New("search index delete context is nil")
	}

	normalizedDocumentID := strings.TrimSpace(documentID)
	if normalizedDocumentID == "" {
		return nil
	}

	snapshot, err := s.searchConfigService.Refresh(ctx)
	if err != nil {
		return err
	}
	if !snapshot.Config.Enabled {
		return nil
	}

	providerName, providerInstance, err := s.resolveProvider(snapshot.Config)
	if err != nil {
		return err
	}
	if providerName == searchcfg.ProviderDatabase {
		return nil
	}
	if err := providerInstance.Delete(ctx, []string{normalizedDocumentID}); err != nil {
		return err
	}
	s.setRebuildRuntimeState(searchIndexRebuildSourceDeleteDoc, 0)
	return nil
}

// SyncSpaceByID 按空间增量重建索引：先清空该空间索引，再批量写入当前可索引文档。
func (s *SearchIndexService) SyncSpaceByID(ctx context.Context, spaceID string) error {
	if s == nil || s.db == nil || s.searchConfigService == nil {
		return errors.New("search index service dependencies are nil")
	}
	if ctx == nil {
		return errors.New("search index sync context is nil")
	}

	normalizedSpaceID := strings.TrimSpace(spaceID)
	if normalizedSpaceID == "" {
		return nil
	}

	snapshot, err := s.searchConfigService.Refresh(ctx)
	if err != nil {
		return err
	}
	if !snapshot.Config.Enabled {
		return nil
	}

	providerName, providerInstance, err := s.resolveProvider(snapshot.Config)
	if err != nil {
		return err
	}
	if providerName == searchcfg.ProviderDatabase {
		return nil
	}
	if err := providerInstance.EnsureSchema(ctx); err != nil {
		return err
	}
	if err := providerInstance.PurgeBySpace(ctx, normalizedSpaceID); err != nil {
		return err
	}

	offset := 0
	indexedDocuments := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		rows, err := s.loadActiveDocumentsForSpaceSync(
			ctx,
			normalizedSpaceID,
			defaultSearchIndexRebuildBatchSize,
			offset,
		)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			break
		}

		records := make([]searchprovider.IndexRecord, 0, len(rows))
		for _, row := range rows {
			record, err := buildSearchIndexRecord(ctx, snapshot, row)
			if err != nil {
				return err
			}
			records = append(records, record)
		}
		if err := providerInstance.Upsert(ctx, records); err != nil {
			return err
		}
		indexedDocuments += len(records)

		if len(rows) < defaultSearchIndexRebuildBatchSize {
			break
		}
		offset += len(rows)
	}
	s.setRebuildRuntimeState(searchIndexRebuildSourceSyncSpace, indexedDocuments)
	return nil
}

// PurgeSpaceByID 删除目标空间在索引中的全部文档。
func (s *SearchIndexService) PurgeSpaceByID(ctx context.Context, spaceID string) error {
	if s == nil || s.db == nil || s.searchConfigService == nil {
		return errors.New("search index service dependencies are nil")
	}
	if ctx == nil {
		return errors.New("search index purge context is nil")
	}

	normalizedSpaceID := strings.TrimSpace(spaceID)
	if normalizedSpaceID == "" {
		return nil
	}

	snapshot, err := s.searchConfigService.Refresh(ctx)
	if err != nil {
		return err
	}
	if !snapshot.Config.Enabled {
		return nil
	}

	providerName, providerInstance, err := s.resolveProvider(snapshot.Config)
	if err != nil {
		return err
	}
	if providerName == searchcfg.ProviderDatabase {
		return nil
	}
	if err := providerInstance.PurgeBySpace(ctx, normalizedSpaceID); err != nil {
		return err
	}
	s.setRebuildRuntimeState(searchIndexRebuildSourcePurgeSpace, 0)
	return nil
}

func (s *SearchIndexService) rebuildWithSnapshot(
	ctx context.Context,
	snapshot SearchRuntimeSnapshot,
	providerName searchcfg.ProviderName,
	providerInstance searchprovider.Provider,
	source string,
) (SearchIndexRebuildResult, error) {
	if !s.tryBeginRebuild() {
		return SearchIndexRebuildResult{}, ErrSearchIndexRebuildInProgress
	}
	defer s.finishRebuild()

	if providerInstance == nil {
		return SearchIndexRebuildResult{}, ErrSearchProviderUnavailable
	}
	if resettableProvider, ok := providerInstance.(searchprovider.ResettableProvider); ok {
		if err := resettableProvider.Reset(ctx); err != nil {
			return SearchIndexRebuildResult{}, err
		}
	}
	if err := providerInstance.EnsureSchema(ctx); err != nil {
		return SearchIndexRebuildResult{}, err
	}

	indexedDocuments := 0
	offset := 0
	for {
		if err := ctx.Err(); err != nil {
			return SearchIndexRebuildResult{}, err
		}

		rows, err := s.loadActiveDocumentsForRebuild(
			ctx,
			defaultSearchIndexRebuildBatchSize,
			offset,
		)
		if err != nil {
			return SearchIndexRebuildResult{}, err
		}
		if len(rows) == 0 {
			break
		}

		records := make([]searchprovider.IndexRecord, 0, len(rows))
		for _, row := range rows {
			indexRecord, err := buildSearchIndexRecord(ctx, snapshot, row)
			if err != nil {
				return SearchIndexRebuildResult{}, err
			}
			records = append(records, indexRecord)
		}
		if err := providerInstance.Upsert(ctx, records); err != nil {
			return SearchIndexRebuildResult{}, err
		}

		indexedDocuments += len(records)
		if len(rows) < defaultSearchIndexRebuildBatchSize {
			break
		}
		offset += len(rows)
	}

	result := SearchIndexRebuildResult{
		Provider:         providerName,
		IndexedDocuments: indexedDocuments,
	}
	s.setRebuildRuntimeState(source, indexedDocuments)
	return result, nil
}

// Status 返回当前检索索引状态（用于后台展示）。
func (s *SearchIndexService) Status(ctx context.Context) (SearchIndexStatusResult, error) {
	if s == nil || s.db == nil || s.searchConfigService == nil {
		return SearchIndexStatusResult{}, errors.New("search index service dependencies are nil")
	}
	if ctx == nil {
		return SearchIndexStatusResult{}, errors.New("search index status context is nil")
	}

	snapshot, err := s.searchConfigService.Refresh(ctx)
	if err != nil {
		return SearchIndexStatusResult{}, err
	}

	lastRebuildAt, lastRebuildSource, lastRebuildIndexedDocuments, rebuildInProgress := s.readRebuildRuntimeState()
	result := SearchIndexStatusResult{
		Enabled:                     snapshot.Config.Enabled,
		ActiveProvider:              snapshot.Config.ActiveProvider,
		FallbackPolicy:              snapshot.Config.FallbackPolicy,
		ActiveAnalyzer:              snapshot.Config.Analysis.ActiveAnalyzer,
		RebuildInProgress:           rebuildInProgress,
		ProviderHealthy:             false,
		ProviderMessage:             "",
		SupportsDocCount:            false,
		IndexedDocuments:            0,
		LastRebuildSource:           lastRebuildSource,
		LastRebuildIndexedDocuments: lastRebuildIndexedDocuments,
	}
	if !lastRebuildAt.IsZero() {
		last := lastRebuildAt
		result.LastRebuildAt = &last
	}
	if !snapshot.Config.Enabled {
		return result, nil
	}

	providerName, providerInstance, resolveErr := s.resolveProvider(snapshot.Config)
	if resolveErr != nil {
		result.ProviderMessage = resolveErr.Error()
		return result, nil
	}
	result.EffectiveProvider = providerName

	if healthErr := providerInstance.Health(ctx); healthErr != nil {
		result.ProviderMessage = healthErr.Error()
	} else {
		result.ProviderHealthy = true
	}

	if statsProvider, ok := providerInstance.(searchprovider.IndexStatsProvider); ok {
		result.SupportsDocCount = true
		docCount, countErr := statsProvider.DocCount(ctx)
		if countErr != nil {
			if result.ProviderMessage == "" {
				result.ProviderMessage = countErr.Error()
			}
			result.ProviderHealthy = false
		} else {
			result.IndexedDocuments = int64(docCount)
		}
	}
	return result, nil
}

func (s *SearchIndexService) setRebuildRuntimeState(source string, indexedDocuments int) {
	if s == nil {
		return
	}
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	s.lastRebuildAt = time.Now().UTC()
	s.lastRebuildSource = strings.TrimSpace(source)
	s.lastRebuildIndexedDocuments = indexedDocuments
}

func (s *SearchIndexService) readRebuildRuntimeState() (time.Time, string, int, bool) {
	if s == nil {
		return time.Time{}, "", 0, false
	}
	s.runtimeMu.RLock()
	defer s.runtimeMu.RUnlock()
	return s.lastRebuildAt, s.lastRebuildSource, s.lastRebuildIndexedDocuments, s.rebuildInProgress
}

func (s *SearchIndexService) tryBeginRebuild() bool {
	if s == nil {
		return false
	}
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	if s.rebuildInProgress {
		return false
	}
	s.rebuildInProgress = true
	return true
}

func (s *SearchIndexService) finishRebuild() {
	if s == nil {
		return
	}
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	s.rebuildInProgress = false
}

// IsRebuildInProgress 返回索引重建任务是否正在执行。
func (s *SearchIndexService) IsRebuildInProgress() bool {
	if s == nil {
		return false
	}
	s.runtimeMu.RLock()
	defer s.runtimeMu.RUnlock()
	return s.rebuildInProgress
}

func (s *SearchIndexService) enqueueTask(task searchIndexAsyncTask) error {
	if s == nil {
		return errors.New("search index service dependencies are nil")
	}
	if s.asyncTaskQueue == nil {
		return errors.New("search index task queue is nil")
	}

	s.asyncTaskWorkerOnce.Do(func() {
		go s.runAsyncTaskWorker()
	})

	select {
	case s.asyncTaskQueue <- task:
		return nil
	default:
		return ErrSearchIndexTaskQueueFull
	}
}

func (s *SearchIndexService) runAsyncTaskWorker() {
	if s == nil || s.asyncTaskQueue == nil {
		return
	}
	for task := range s.asyncTaskQueue {
		taskCtx, cancel := context.WithTimeout(context.Background(), defaultSearchIndexAsyncTaskTimeout)
		switch task.taskType {
		case searchIndexAsyncTaskTypeSyncDocument:
			_ = s.SyncDocumentByID(taskCtx, task.documentID)
		case searchIndexAsyncTaskTypeDeleteDocument:
			_ = s.DeleteDocumentByID(taskCtx, task.documentID)
		}
		cancel()
	}
}

func (s *SearchIndexService) resolveProvider(
	config searchcfg.Config,
) (searchcfg.ProviderName, searchprovider.Provider, error) {
	if s == nil {
		return "", nil, fmt.Errorf("%w: search index service is nil", ErrSearchProviderUnavailable)
	}

	activeProvider := config.ActiveProvider
	if providerInstance := s.providers[activeProvider]; providerInstance != nil {
		return activeProvider, providerInstance, nil
	}

	if config.FallbackPolicy == searchcfg.FallbackPolicyDegradeToDatabase &&
		activeProvider != searchcfg.ProviderDatabase {
		if fallbackProvider := s.providers[searchcfg.ProviderDatabase]; fallbackProvider != nil {
			return searchcfg.ProviderDatabase, fallbackProvider, nil
		}
	}

	return "", nil, fmt.Errorf(
		"%w: provider %q is not configured",
		ErrSearchProviderUnavailable,
		activeProvider,
	)
}

func (s *SearchIndexService) loadActiveDocumentsForRebuild(
	ctx context.Context,
	limit int,
	offset int,
) ([]repository.SearchIndexSourceDocumentRecord, error) {
	if s == nil || s.searchIndexSourceRepo == nil {
		return nil, errors.New("search index source repository is nil")
	}
	if limit <= 0 {
		limit = defaultSearchIndexRebuildBatchSize
	}
	if offset < 0 {
		offset = 0
	}
	return s.searchIndexSourceRepo.ListActiveDocuments(
		ctx,
		repository.ListSearchIndexSourceDocumentsParams{
			Limit:  limit,
			Offset: offset,
		},
	)
}

func (s *SearchIndexService) loadActiveDocumentForSync(
	ctx context.Context,
	documentID string,
) (repository.SearchIndexSourceDocumentRecord, error) {
	if s == nil || s.searchIndexSourceRepo == nil {
		return repository.SearchIndexSourceDocumentRecord{}, errors.New("search index source repository is nil")
	}

	normalizedDocumentID := strings.TrimSpace(documentID)
	if normalizedDocumentID == "" {
		return repository.SearchIndexSourceDocumentRecord{}, gorm.ErrRecordNotFound
	}

	row, err := s.searchIndexSourceRepo.GetActiveDocumentByDocumentID(ctx, normalizedDocumentID)
	if err != nil {
		return repository.SearchIndexSourceDocumentRecord{}, err
	}
	if row == nil {
		return repository.SearchIndexSourceDocumentRecord{}, gorm.ErrRecordNotFound
	}
	return *row, nil
}

func (s *SearchIndexService) loadActiveDocumentsForSpaceSync(
	ctx context.Context,
	spaceID string,
	limit int,
	offset int,
) ([]repository.SearchIndexSourceDocumentRecord, error) {
	if s == nil || s.searchIndexSourceRepo == nil {
		return nil, errors.New("search index source repository is nil")
	}
	if limit <= 0 {
		limit = defaultSearchIndexRebuildBatchSize
	}
	if offset < 0 {
		offset = 0
	}

	normalizedSpaceID := strings.TrimSpace(spaceID)
	if normalizedSpaceID == "" {
		return []repository.SearchIndexSourceDocumentRecord{}, nil
	}

	return s.searchIndexSourceRepo.ListActiveDocumentsBySpaceID(
		ctx,
		repository.ListSearchIndexSourceDocumentsBySpaceParams{
			SpaceID: normalizedSpaceID,
			Limit:   limit,
			Offset:  offset,
		},
	)
}

func buildSearchIndexRecord(
	ctx context.Context,
	snapshot SearchRuntimeSnapshot,
	row repository.SearchIndexSourceDocumentRecord,
) (searchprovider.IndexRecord, error) {
	if snapshot.ActiveAnalyzer == nil {
		return searchprovider.IndexRecord{}, errors.New("active analyzer is nil")
	}

	spaceID := strings.TrimSpace(row.SpaceID)
	title := strings.TrimSpace(row.Title)
	bodyPlain := normalizeSearchIndexBody(row.Format, row.ContentMD)
	titleCompoundTokens := searchprovider.ExtractCompoundLiteralTokens(title)
	bodyCompoundTokens := searchprovider.ExtractCompoundLiteralTokens(bodyPlain)

	titleOutput, err := snapshot.ActiveAnalyzer.AnalyzeForIndex(ctx, searchanalyzer.AnalyzeInput{
		Text:    title,
		Mode:    searchanalyzer.ModeIndex,
		SpaceID: spaceID,
	})
	if err != nil {
		return searchprovider.IndexRecord{}, err
	}
	bodyOutput, err := snapshot.ActiveAnalyzer.AnalyzeForIndex(ctx, searchanalyzer.AnalyzeInput{
		Text:    bodyPlain,
		Mode:    searchanalyzer.ModeIndex,
		SpaceID: spaceID,
	})
	if err != nil {
		return searchprovider.IndexRecord{}, err
	}

	visibilityScope, minRole := resolveSearchVisibilityAndRole(
		row.SpaceVisibility,
		row.DocVisibility,
	)

	dictVersion := strings.TrimSpace(bodyOutput.DictVersion)
	if dictVersion == "" {
		dictVersion = strings.TrimSpace(titleOutput.DictVersion)
	}
	if dictVersion == "" {
		dictVersion = searchcfg.DefaultDictVersion
	}

	return searchprovider.IndexRecord{
		SpaceID:   spaceID,
		DocID:     strings.TrimSpace(row.DocumentID),
		NodeID:    strings.TrimSpace(row.NodeID),
		Title:     title,
		BodyPlain: bodyPlain,
		Terms: strings.Join(
			mergeSearchTokens(titleOutput.Tokens, bodyOutput.Tokens, titleCompoundTokens, bodyCompoundTokens),
			" ",
		),
		TitleTerms:      strings.Join(mergeSearchTokens(titleOutput.Tokens, titleCompoundTokens), " "),
		VisibilityScope: visibilityScope,
		MinRole:         minRole,
		UpdatedAtUnix:   parseSearchIndexUnix(row.UpdatedAt),
		IsDeleted:       false,
		SpaceStatus:     string(models.EntityStatusActive),
		DocStatus:       string(models.EntityStatusActive),
		AnalyzerName:    string(snapshot.Config.Analysis.ActiveAnalyzer),
		AnalyzerVersion: dictVersion,
	}, nil
}

func normalizeSearchIndexBody(format models.DocumentFormat, content string) string {
	if models.IsOfficeDocumentFormat(format) {
		return strings.TrimSpace(searchanalyzer.NormalizeHTMLToPlainText(content))
	}
	return strings.TrimSpace(searchanalyzer.NormalizeMarkdownToPlainText(content))
}

func mergeSearchTokens(items ...[]string) []string {
	if len(items) == 0 {
		return []string{}
	}
	result := make([]string, 0, 32)
	seen := make(map[string]struct{}, 32)
	for _, group := range items {
		for _, token := range group {
			normalized := strings.TrimSpace(strings.ToLower(token))
			if normalized == "" {
				continue
			}
			if _, exists := seen[normalized]; exists {
				continue
			}
			seen[normalized] = struct{}{}
			result = append(result, normalized)
		}
	}
	return result
}

func parseSearchIndexUnix(value any) int64 {
	switch current := value.(type) {
	case nil:
		return time.Now().UTC().Unix()
	case time.Time:
		return current.Unix()
	case *time.Time:
		if current == nil {
			return time.Now().UTC().Unix()
		}
		return current.Unix()
	case int64:
		return current
	case int:
		return int64(current)
	case float64:
		return int64(current)
	case []byte:
		return parseSearchIndexUnixString(string(current))
	case string:
		return parseSearchIndexUnixString(current)
	default:
		return parseSearchIndexUnixString(fmt.Sprint(current))
	}
}

func parseSearchIndexUnixString(value string) int64 {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return time.Now().UTC().Unix()
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
		time.DateTime,
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, raw)
		if err == nil {
			return parsed.Unix()
		}
	}
	if epochSeconds, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return epochSeconds
	}
	if epochFloat, err := strconv.ParseFloat(raw, 64); err == nil {
		return int64(epochFloat)
	}
	return time.Now().UTC().Unix()
}

func resolveSearchVisibilityAndRole(
	spaceVisibility string,
	documentVisibility string,
) (string, int) {
	resolveRank := func(value string) int {
		switch models.Visibility(strings.ToLower(strings.TrimSpace(value))) {
		case models.VisibilityPublic:
			return 1
		case models.VisibilityAuthenticated:
			return 2
		case models.VisibilityMember:
			return 3
		default:
			return 3
		}
	}

	rank := resolveRank(spaceVisibility)
	documentRank := resolveRank(documentVisibility)
	if documentRank > rank {
		rank = documentRank
	}

	switch rank {
	case 1:
		return string(models.VisibilityPublic), 1
	case 2:
		return string(models.VisibilityAuthenticated), 1
	default:
		return string(models.VisibilityMember), 1
	}
}
