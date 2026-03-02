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

	config, sourceVersion, err := s.loadConfig(ctx)
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
	s.snapshot = nextSnapshot
	s.initialized = true
	s.mu.Unlock()

	return nextSnapshot, nil
}

func (s *SearchConfigService) loadConfig(ctx context.Context) (searchcfg.Config, int, error) {
	defaultConfig := searchcfg.DefaultConfig()
	if s == nil || s.systemConfigRepo == nil {
		return defaultConfig, 0, nil
	}

	configRecord, err := s.systemConfigRepo.GetByConfigKey(ctx, searchcfg.SystemConfigKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return defaultConfig, 0, nil
		}
		return defaultConfig, 0, err
	}
	if configRecord == nil || strings.TrimSpace(configRecord.ConfigValueJSON) == "" {
		return defaultConfig, 0, nil
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(configRecord.ConfigValueJSON), &payload); err != nil {
		return defaultConfig, 0, err
	}
	if payload == nil {
		payload = map[string]any{}
	}

	if err := searchcfg.ValidateConfigPayload(payload); err != nil {
		return defaultConfig, 0, err
	}
	normalized := searchcfg.NormalizeConfig(payload)
	return normalized, configRecord.Version, nil
}
