package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	searchcfg "github.com/lifei6671/plaindoc/apps/server/internal/search"
	"github.com/lifei6671/plaindoc/apps/server/internal/search/analyzer"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

// SearchJiebaDictLoader 定义 Jieba 用户词典加载回调。
type SearchJiebaDictLoader func(ctx context.Context, config searchcfg.Config) ([]string, error)

// SearchConfigServiceOptions 定义 SearchConfigService 可选参数。
type SearchConfigServiceOptions struct {
	JiebaDictLoader SearchJiebaDictLoader
}

// SearchRuntimeSnapshot 表示当前检索配置运行时快照。
type SearchRuntimeSnapshot struct {
	Config         searchcfg.Config
	Registry       *analyzer.Registry
	ActiveAnalyzer analyzer.Provider
	SourceVersion  int
	LoadedAt       time.Time
}

// SearchConfigService 负责读取 system_configs.search 并解析分词运行时。
type SearchConfigService struct {
	systemConfigRepo repository.SystemConfigRepository
	jiebaDictLoader  SearchJiebaDictLoader

	mu          sync.RWMutex
	snapshot    SearchRuntimeSnapshot
	initialized bool
}

// NewSearchConfigService 创建检索配置服务。
func NewSearchConfigService(
	systemConfigRepo repository.SystemConfigRepository,
	options SearchConfigServiceOptions,
) *SearchConfigService {
	return &SearchConfigService{
		systemConfigRepo: systemConfigRepo,
		jiebaDictLoader:  options.JiebaDictLoader,
	}
}

// Current 返回当前内存快照；若尚未初始化则返回 false。
func (s *SearchConfigService) Current() (SearchRuntimeSnapshot, bool) {
	if s == nil {
		return SearchRuntimeSnapshot{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.initialized {
		return SearchRuntimeSnapshot{}, false
	}
	return s.snapshot, true
}

// Resolve 返回当前可用快照；首次调用时会从配置中心加载。
func (s *SearchConfigService) Resolve(ctx context.Context) (SearchRuntimeSnapshot, error) {
	if s == nil {
		return SearchRuntimeSnapshot{}, errors.New("search config service is nil")
	}
	if snapshot, ok := s.Current(); ok {
		return snapshot, nil
	}
	return s.Refresh(ctx)
}

// Refresh 重新加载 search 配置并刷新运行时分词器快照。
func (s *SearchConfigService) Refresh(ctx context.Context) (SearchRuntimeSnapshot, error) {
	if s == nil {
		return SearchRuntimeSnapshot{}, errors.New("search config service is nil")
	}

	configRecord, err := s.loadConfigRecord(ctx)
	if err != nil {
		return SearchRuntimeSnapshot{}, err
	}
	sourceVersion := 0
	if configRecord != nil {
		sourceVersion = configRecord.Version
	}

	s.mu.RLock()
	if s.initialized && s.snapshot.SourceVersion == sourceVersion {
		current := s.snapshot
		s.mu.RUnlock()
		return current, nil
	}
	s.mu.RUnlock()

	config, err := s.parseConfigFromRecord(configRecord)
	if err != nil {
		return SearchRuntimeSnapshot{}, err
	}

	dictEntries := []string{}
	if s.jiebaDictLoader != nil && config.Analysis.Analyzers.Jieba.Enabled {
		loadedEntries, loadErr := s.jiebaDictLoader(ctx, config)
		if loadErr != nil {
			return SearchRuntimeSnapshot{}, fmt.Errorf("load jieba dict entries: %w", loadErr)
		}
		dictEntries = append(dictEntries, loadedEntries...)
	}

	registry, err := searchcfg.BuildAnalyzerRegistry(config, searchcfg.AnalyzerResolverOptions{
		JiebaUserDictEntries: dictEntries,
	})
	if err != nil {
		return SearchRuntimeSnapshot{}, err
	}
	activeAnalyzer, err := searchcfg.ResolveActiveAnalyzer(config, registry)
	if err != nil {
		return SearchRuntimeSnapshot{}, err
	}

	nextSnapshot := SearchRuntimeSnapshot{
		Config:         config,
		Registry:       registry,
		ActiveAnalyzer: activeAnalyzer,
		SourceVersion:  sourceVersion,
		LoadedAt:       time.Now().UTC(),
	}

	s.mu.Lock()
	if s.initialized && s.snapshot.SourceVersion == sourceVersion {
		current := s.snapshot
		s.mu.Unlock()
		return current, nil
	}
	s.snapshot = nextSnapshot
	s.initialized = true
	s.mu.Unlock()

	return nextSnapshot, nil
}

func (s *SearchConfigService) loadConfigRecord(
	ctx context.Context,
) (*models.SystemConfig, error) {
	if s == nil || s.systemConfigRepo == nil {
		return nil, nil
	}

	configRecord, err := s.systemConfigRepo.GetByConfigKey(ctx, searchcfg.SystemConfigKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return configRecord, nil
}

func (s *SearchConfigService) parseConfigFromRecord(
	configRecord *models.SystemConfig,
) (searchcfg.Config, error) {
	defaultConfig := searchcfg.DefaultConfig()
	if configRecord == nil || strings.TrimSpace(configRecord.ConfigValueJSON) == "" {
		return defaultConfig, nil
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(configRecord.ConfigValueJSON), &payload); err != nil {
		return defaultConfig, err
	}
	if payload == nil {
		payload = map[string]any{}
	}

	if err := searchcfg.ValidateConfigPayload(payload); err != nil {
		return defaultConfig, err
	}
	normalized := searchcfg.NormalizeConfig(payload)
	return normalized, nil
}
