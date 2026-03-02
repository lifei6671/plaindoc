package service

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	searchcfg "github.com/lifei6671/plaindoc/apps/server/internal/search"
	"github.com/lifei6671/plaindoc/apps/server/internal/search/analyzer"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

type stubSystemConfigRepository struct {
	recordByKey map[string]*models.SystemConfig
	errByKey    map[string]error
}

func (r *stubSystemConfigRepository) List(ctx context.Context) ([]models.SystemConfig, error) {
	return nil, errors.New("not implemented")
}

func (r *stubSystemConfigRepository) GetByConfigKey(
	ctx context.Context,
	configKey string,
) (*models.SystemConfig, error) {
	if r == nil {
		return nil, gorm.ErrRecordNotFound
	}
	if err, ok := r.errByKey[configKey]; ok && err != nil {
		return nil, err
	}
	record, ok := r.recordByKey[configKey]
	if !ok || record == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return record, nil
}

func (r *stubSystemConfigRepository) Create(ctx context.Context, config *models.SystemConfig) error {
	if r == nil {
		return errors.New("stub system config repository is nil")
	}
	if config == nil {
		return errors.New("system config is nil")
	}
	if r.recordByKey == nil {
		r.recordByKey = map[string]*models.SystemConfig{}
	}
	configKey := strings.TrimSpace(config.ConfigKey)
	if configKey == "" {
		return errors.New("config key is empty")
	}
	if _, exists := r.recordByKey[configKey]; exists {
		return gorm.ErrDuplicatedKey
	}
	cloned := *config
	r.recordByKey[configKey] = &cloned
	return nil
}

func (r *stubSystemConfigRepository) UpdateByVersion(
	ctx context.Context,
	params repository.UpdateSystemConfigByVersionParams,
) (bool, error) {
	if r == nil {
		return false, errors.New("stub system config repository is nil")
	}
	configKey := strings.TrimSpace(params.ConfigKey)
	if configKey == "" {
		return false, nil
	}
	record, exists := r.recordByKey[configKey]
	if !exists || record == nil {
		return false, nil
	}
	if record.Version != params.ExpectedVersion {
		return false, nil
	}
	updatedAt := params.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	record.ConfigValueJSON = params.ConfigValueJSON
	record.Version = params.NextVersion
	record.UpdatedByUserID = params.UpdatedByUserID
	record.UpdatedAt = updatedAt
	return true, nil
}

func TestSearchConfigService_Resolve_DefaultWhenConfigMissing(t *testing.T) {
	systemConfigRepo := &stubSystemConfigRepository{
		recordByKey: map[string]*models.SystemConfig{},
		errByKey: map[string]error{
			searchcfg.SystemConfigKey: gorm.ErrRecordNotFound,
		},
	}
	service := NewSearchConfigService(systemConfigRepo, SearchConfigServiceOptions{})

	snapshot, err := service.Resolve(context.Background())
	if err != nil {
		t.Fatalf("resolve search config failed: %v", err)
	}
	if snapshot.Config.ActiveProvider != searchcfg.ProviderBleve {
		t.Fatalf("expected default active provider %q, got %q", searchcfg.ProviderBleve, snapshot.Config.ActiveProvider)
	}
	if snapshot.ActiveAnalyzer == nil {
		t.Fatalf("expected active analyzer")
	}
	if snapshot.ActiveAnalyzer.Name() != "simple" {
		t.Fatalf("expected default active analyzer simple, got %q", snapshot.ActiveAnalyzer.Name())
	}
	if snapshot.SourceVersion != 0 {
		t.Fatalf("expected source version 0, got %d", snapshot.SourceVersion)
	}
}

func TestSearchConfigService_Refresh_WithJiebaDictLoader(t *testing.T) {
	systemConfigRepo := &stubSystemConfigRepository{
		recordByKey: map[string]*models.SystemConfig{
			searchcfg.SystemConfigKey: {
				ConfigKey: searchcfg.SystemConfigKey,
				ConfigValueJSON: `{
					"enabled":true,"activeProvider":"bleve",
					"fallbackPolicy":"degrade_to_bleve",
					"analysis":{
						"activeAnalyzer":"jieba",
						"analyzers":{
							"simple":{"enabled":false},
							"jieba":{
								"enabled":true,
								"mode":"search",
								"hmm":true,
								"stopwordsEnabled":false,
								"dictSource":"db",
								"dictVersion":"v2026-03-02-001"
							}
						}
					}
				}`,
				Version: 3,
			},
		},
		errByKey: map[string]error{},
	}

	service := NewSearchConfigService(systemConfigRepo, SearchConfigServiceOptions{
		JiebaDictLoader: func(ctx context.Context, config searchcfg.Config) ([]string, error) {
			if config.Analysis.ActiveAnalyzer != searchcfg.AnalyzerJieba {
				t.Fatalf("expected active analyzer jieba in loader, got %q", config.Analysis.ActiveAnalyzer)
			}
			return []string{"微服务架构 200 n"}, nil
		},
	})

	snapshot, err := service.Refresh(context.Background())
	if err != nil {
		t.Fatalf("refresh search config failed: %v", err)
	}
	if snapshot.ActiveAnalyzer == nil {
		t.Fatalf("expected active analyzer")
	}
	if snapshot.ActiveAnalyzer.Name() != "jieba" {
		t.Fatalf("expected active analyzer jieba, got %q", snapshot.ActiveAnalyzer.Name())
	}
	if snapshot.SourceVersion != 3 {
		t.Fatalf("expected source version 3, got %d", snapshot.SourceVersion)
	}
	if snapshot.LoadedAt.IsZero() {
		t.Fatal("expected loadedAt not zero")
	}

	output, err := snapshot.ActiveAnalyzer.AnalyzeForQuery(context.Background(), analyzer.AnalyzeInput{
		Text: "我们基于微服务架构建设检索系统",
		Mode: analyzer.ModeQuery,
	})
	if err != nil {
		t.Fatalf("analyze query failed: %v", err)
	}
	if !slices.Contains(output.Tokens, "微服务架构") {
		t.Fatalf("expected token 微服务架构 in %v", output.Tokens)
	}
}

func TestSearchConfigService_RefreshFailure_DoesNotReplaceCurrentSnapshot(t *testing.T) {
	repo := &stubSystemConfigRepository{
		recordByKey: map[string]*models.SystemConfig{
			searchcfg.SystemConfigKey: {
				ConfigKey: searchcfg.SystemConfigKey,
				ConfigValueJSON: `{
					"enabled":true,"activeProvider":"bleve",
					"fallbackPolicy":"degrade_to_bleve",
					"analysis":{
						"activeAnalyzer":"simple",
						"analyzers":{
							"simple":{"enabled":true},
							"jieba":{
								"enabled":false,
								"mode":"search",
								"hmm":true,
								"stopwordsEnabled":false,
								"dictSource":"db",
								"dictVersion":"v1"
							}
						}
					}
				}`,
				Version:   1,
				UpdatedAt: time.Now().UTC(),
			},
		},
		errByKey: map[string]error{},
	}

	service := NewSearchConfigService(repo, SearchConfigServiceOptions{})
	firstSnapshot, err := service.Refresh(context.Background())
	if err != nil {
		t.Fatalf("first refresh failed: %v", err)
	}
	if firstSnapshot.ActiveAnalyzer == nil || firstSnapshot.ActiveAnalyzer.Name() != "simple" {
		t.Fatalf("expected first active analyzer simple")
	}

	repo.recordByKey[searchcfg.SystemConfigKey] = &models.SystemConfig{
		ConfigKey: searchcfg.SystemConfigKey,
		ConfigValueJSON: `{
			"enabled":true,"activeProvider":"bleve",
			"fallbackPolicy":"degrade_to_bleve",
			"analysis":{
				"activeAnalyzer":"jieba",
				"analyzers":{
					"simple":{"enabled":true},
					"jieba":{
						"enabled":false,
						"mode":"search",
						"hmm":true,
						"stopwordsEnabled":false,
						"dictSource":"db",
						"dictVersion":"v2"
					}
				}
			}
		}`,
		Version: 2,
	}

	if _, err := service.Refresh(context.Background()); err == nil {
		t.Fatal("expected refresh error for invalid config")
	}

	currentSnapshot, ok := service.Current()
	if !ok {
		t.Fatal("expected current snapshot exists")
	}
	if currentSnapshot.SourceVersion != 1 {
		t.Fatalf("expected current snapshot version 1, got %d", currentSnapshot.SourceVersion)
	}
	if currentSnapshot.ActiveAnalyzer == nil || currentSnapshot.ActiveAnalyzer.Name() != "simple" {
		t.Fatalf("expected current active analyzer still simple")
	}
}

func TestSearchConfigService_Refresh_JiebaDictLoaderError(t *testing.T) {
	repo := &stubSystemConfigRepository{
		recordByKey: map[string]*models.SystemConfig{
			searchcfg.SystemConfigKey: {
				ConfigKey: searchcfg.SystemConfigKey,
				ConfigValueJSON: `{
					"enabled":true,"activeProvider":"bleve",
					"fallbackPolicy":"degrade_to_bleve",
					"analysis":{
						"activeAnalyzer":"jieba",
						"analyzers":{
							"simple":{"enabled":false},
							"jieba":{
								"enabled":true,
								"mode":"search",
								"hmm":true,
								"stopwordsEnabled":false,
								"dictSource":"db",
								"dictVersion":"v1"
							}
						}
					}
				}`,
				Version: 2,
			},
		},
		errByKey: map[string]error{},
	}

	service := NewSearchConfigService(repo, SearchConfigServiceOptions{
		JiebaDictLoader: func(ctx context.Context, config searchcfg.Config) ([]string, error) {
			return nil, errors.New("dict load failed")
		},
	})

	if _, err := service.Refresh(context.Background()); err == nil {
		t.Fatal("expected refresh error when dict loader fails")
	}
	if _, ok := service.Current(); ok {
		t.Fatal("expected no snapshot when first refresh failed")
	}
}
