package search

import (
	"errors"
	"fmt"
	"strings"

	"github.com/lifei6671/plaindoc/apps/server/internal/search/analyzer"
)

// AnalyzerResolverOptions 定义分词器构建参数。
type AnalyzerResolverOptions struct {
	JiebaUserDictEntries []string
}

// BuildAnalyzerRegistry 根据 search 配置构建 analyzer 注册表。
func BuildAnalyzerRegistry(
	config Config,
	options AnalyzerResolverOptions,
) (*analyzer.Registry, error) {
	providers := make([]analyzer.Provider, 0, 2)
	if config.Analysis.Analyzers.Simple.Enabled {
		providers = append(providers, analyzer.NewSimpleAnalyzer(DefaultDictVersion))
	}

	if config.Analysis.Analyzers.Jieba.Enabled {
		jiebaProvider, err := analyzer.NewJiebaAnalyzer(analyzer.JiebaOptions{
			DictVersion:     normalizeDictVersion(config.Analysis.Analyzers.Jieba.DictVersion),
			UserDictEntries: append([]string(nil), options.JiebaUserDictEntries...),
			EnableHMM:       config.Analysis.Analyzers.Jieba.HMM,
		})
		if err != nil {
			return nil, fmt.Errorf("build jieba analyzer: %w", err)
		}
		providers = append(providers, jiebaProvider)
	}
	if len(providers) == 0 {
		return nil, errors.New("no analyzer provider enabled")
	}

	registry, err := analyzer.NewRegistry(providers...)
	if err != nil {
		return nil, fmt.Errorf("build analyzer registry: %w", err)
	}
	return registry, nil
}

// ResolveActiveAnalyzer 从 registry 中解析 active analyzer。
func ResolveActiveAnalyzer(config Config, registry *analyzer.Registry) (analyzer.Provider, error) {
	if registry == nil {
		return nil, errors.New("analyzer registry is nil")
	}
	activeAnalyzerName := normalizeAnalyzerName(string(config.Analysis.ActiveAnalyzer))
	if activeAnalyzerName == "" {
		activeAnalyzerName = AnalyzerSimple
	}
	return registry.Get(string(activeAnalyzerName))
}

func normalizeDictVersion(value string) string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return DefaultDictVersion
	}
	return normalized
}
