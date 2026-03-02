package service

import (
	"context"
	"fmt"

	searchcfg "github.com/lifei6671/plaindoc/apps/server/internal/search"
	searchanalyzer "github.com/lifei6671/plaindoc/apps/server/internal/search/analyzer"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
)

// SearchAnalyzerDictService 封装分词词典加载能力。
type SearchAnalyzerDictService struct {
	dictEntryRepo repository.SearchAnalyzerDictEntryRepository
}

// NewSearchAnalyzerDictService 创建分词词典服务。
func NewSearchAnalyzerDictService(
	dictEntryRepo repository.SearchAnalyzerDictEntryRepository,
) *SearchAnalyzerDictService {
	return &SearchAnalyzerDictService{
		dictEntryRepo: dictEntryRepo,
	}
}

// BuildJiebaDictLoader 构建供 SearchConfigService 使用的 Jieba 词典加载器。
func (s *SearchAnalyzerDictService) BuildJiebaDictLoader() SearchJiebaDictLoader {
	return func(ctx context.Context, config searchcfg.Config) ([]string, error) {
		return s.LoadJiebaEntries(ctx, config)
	}
}

// LoadJiebaEntries 从数据库加载 Jieba 生效词条，并格式化为 jieba 词典行。
func (s *SearchAnalyzerDictService) LoadJiebaEntries(
	ctx context.Context,
	config searchcfg.Config,
) ([]string, error) {
	if s == nil || s.dictEntryRepo == nil {
		return []string{}, nil
	}
	if !config.Analysis.Analyzers.Jieba.Enabled {
		return []string{}, nil
	}
	if config.Analysis.Analyzers.Jieba.DictSource != searchcfg.JiebaDictSourceDB {
		return []string{}, nil
	}

	entries, err := s.dictEntryRepo.ListActiveByAnalyzer(ctx, string(searchcfg.AnalyzerJieba))
	if err != nil {
		return nil, err
	}

	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		weight := 0
		if entry.Weight != nil && *entry.Weight > 0 {
			weight = *entry.Weight
		}
		line, err := searchanalyzer.FormatJiebaDictEntry(entry.Term, weight, entry.Tag)
		if err != nil {
			return nil, fmt.Errorf("invalid jieba dict entry id=%d: %w", entry.ID, err)
		}
		lines = append(lines, line)
	}
	return lines, nil
}
