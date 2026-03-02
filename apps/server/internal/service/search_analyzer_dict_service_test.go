package service

import (
	"context"
	"errors"
	"slices"
	"testing"

	searchcfg "github.com/lifei6671/plaindoc/apps/server/internal/search"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
)

type stubSearchAnalyzerDictEntryRepository struct {
	items  []models.SearchAnalyzerDictEntry
	err    error
	called bool
}

func (r *stubSearchAnalyzerDictEntryRepository) List(
	ctx context.Context,
	params repository.ListSearchAnalyzerDictEntriesParams,
) ([]models.SearchAnalyzerDictEntry, int64, error) {
	if r.err != nil {
		return nil, 0, r.err
	}
	return append([]models.SearchAnalyzerDictEntry(nil), r.items...), int64(len(r.items)), nil
}

func (r *stubSearchAnalyzerDictEntryRepository) ListActiveByAnalyzer(
	ctx context.Context,
	analyzer string,
) ([]models.SearchAnalyzerDictEntry, error) {
	r.called = true
	if r.err != nil {
		return nil, r.err
	}
	return append([]models.SearchAnalyzerDictEntry(nil), r.items...), nil
}

func (r *stubSearchAnalyzerDictEntryRepository) GetByID(
	ctx context.Context,
	id int64,
) (*models.SearchAnalyzerDictEntry, error) {
	return nil, errors.New("not implemented")
}

func (r *stubSearchAnalyzerDictEntryRepository) GetByAnalyzerAndTerm(
	ctx context.Context,
	analyzer string,
	term string,
) (*models.SearchAnalyzerDictEntry, error) {
	return nil, errors.New("not implemented")
}

func (r *stubSearchAnalyzerDictEntryRepository) Create(
	ctx context.Context,
	entry *models.SearchAnalyzerDictEntry,
) error {
	return errors.New("not implemented")
}

func (r *stubSearchAnalyzerDictEntryRepository) UpdateByID(
	ctx context.Context,
	id int64,
	updates map[string]any,
) (bool, error) {
	return false, errors.New("not implemented")
}

func TestSearchAnalyzerDictService_LoadJiebaEntries_DBSource(t *testing.T) {
	weight200 := 200
	repo := &stubSearchAnalyzerDictEntryRepository{
		items: []models.SearchAnalyzerDictEntry{
			{
				ID:       1,
				Analyzer: "jieba",
				Term:     "微服务架构",
				Weight:   &weight200,
				Tag:      "n",
				Status:   models.SearchAnalyzerDictEntryStatusActive,
			},
			{
				ID:       2,
				Analyzer: "jieba",
				Term:     "搜索",
				Status:   models.SearchAnalyzerDictEntryStatusActive,
			},
		},
	}
	service := NewSearchAnalyzerDictService(repo)
	config := searchcfg.DefaultConfig()
	config.Analysis.Analyzers.Jieba.Enabled = true
	config.Analysis.Analyzers.Jieba.DictSource = searchcfg.JiebaDictSourceDB

	lines, err := service.LoadJiebaEntries(context.Background(), config)
	if err != nil {
		t.Fatalf("load jieba entries failed: %v", err)
	}
	if !repo.called {
		t.Fatal("expected repository called")
	}
	if !slices.Contains(lines, "微服务架构 200 n") {
		t.Fatalf("expected formatted line 微服务架构 200 n in %v", lines)
	}
	if !slices.Contains(lines, "搜索") {
		t.Fatalf("expected formatted line 搜索 in %v", lines)
	}
}

func TestSearchAnalyzerDictService_LoadJiebaEntries_NonDBSourceSkipsLoad(t *testing.T) {
	repo := &stubSearchAnalyzerDictEntryRepository{}
	service := NewSearchAnalyzerDictService(repo)
	config := searchcfg.DefaultConfig()
	config.Analysis.Analyzers.Jieba.Enabled = true
	config.Analysis.Analyzers.Jieba.DictSource = searchcfg.JiebaDictSourceFile

	lines, err := service.LoadJiebaEntries(context.Background(), config)
	if err != nil {
		t.Fatalf("load jieba entries failed: %v", err)
	}
	if len(lines) != 0 {
		t.Fatalf("expected empty lines, got %v", lines)
	}
	if repo.called {
		t.Fatal("expected repository not called when dict source is file")
	}
}

func TestSearchAnalyzerDictService_LoadJiebaEntries_InvalidEntryReturnsError(t *testing.T) {
	repo := &stubSearchAnalyzerDictEntryRepository{
		items: []models.SearchAnalyzerDictEntry{
			{
				ID:       10,
				Analyzer: "jieba",
				Term:     " ",
				Status:   models.SearchAnalyzerDictEntryStatusActive,
			},
		},
	}
	service := NewSearchAnalyzerDictService(repo)
	config := searchcfg.DefaultConfig()
	config.Analysis.Analyzers.Jieba.Enabled = true
	config.Analysis.Analyzers.Jieba.DictSource = searchcfg.JiebaDictSourceDB

	if _, err := service.LoadJiebaEntries(context.Background(), config); err == nil {
		t.Fatal("expected invalid entry error")
	}
}

func TestSearchAnalyzerDictService_BuildJiebaDictLoader(t *testing.T) {
	repo := &stubSearchAnalyzerDictEntryRepository{
		err: errors.New("db down"),
	}
	service := NewSearchAnalyzerDictService(repo)
	config := searchcfg.DefaultConfig()
	config.Analysis.Analyzers.Jieba.Enabled = true
	config.Analysis.Analyzers.Jieba.DictSource = searchcfg.JiebaDictSourceDB

	loader := service.BuildJiebaDictLoader()
	if _, err := loader(context.Background(), config); err == nil {
		t.Fatal("expected loader error")
	}
}
