package search

import (
	"context"
	"slices"
	"testing"

	"github.com/lifei6671/plaindoc/apps/server/internal/search/analyzer"
)

func TestBuildAnalyzerRegistryAndResolveActive_DefaultSimple(t *testing.T) {
	config := DefaultConfig()

	registry, err := BuildAnalyzerRegistry(config, AnalyzerResolverOptions{})
	if err != nil {
		t.Fatalf("build analyzer registry failed: %v", err)
	}

	provider, err := ResolveActiveAnalyzer(config, registry)
	if err != nil {
		t.Fatalf("resolve active analyzer failed: %v", err)
	}
	if provider.Name() != "simple" {
		t.Fatalf("expected active analyzer simple, got %q", provider.Name())
	}
}

func TestBuildAnalyzerRegistryAndResolveActive_Jieba(t *testing.T) {
	config := DefaultConfig()
	config.Analysis.Analyzers.Simple.Enabled = false
	config.Analysis.ActiveAnalyzer = AnalyzerJieba
	config.Analysis.Analyzers.Jieba.Enabled = true
	config.Analysis.Analyzers.Jieba.DictVersion = "v2026-03-02-001"
	config.Analysis.Analyzers.Jieba.HMM = true

	registry, err := BuildAnalyzerRegistry(config, AnalyzerResolverOptions{
		JiebaUserDictEntries: []string{"微服务架构 200 n"},
	})
	if err != nil {
		t.Fatalf("build analyzer registry failed: %v", err)
	}

	provider, err := ResolveActiveAnalyzer(config, registry)
	if err != nil {
		t.Fatalf("resolve active analyzer failed: %v", err)
	}
	if provider.Name() != "jieba" {
		t.Fatalf("expected active analyzer jieba, got %q", provider.Name())
	}

	output, err := provider.AnalyzeForQuery(context.Background(), analyzerInput("我们用微服务架构实现搜索"))
	if err != nil {
		t.Fatalf("analyze query failed: %v", err)
	}
	if !slices.Contains(output.Tokens, "微服务架构") {
		t.Fatalf("expected token 微服务架构 in %+v", output.Tokens)
	}
	if output.DictVersion != "v2026-03-02-001" {
		t.Fatalf("expected dict version %q, got %q", "v2026-03-02-001", output.DictVersion)
	}
}

func TestBuildAnalyzerRegistry_RejectsWhenAllAnalyzersDisabled(t *testing.T) {
	config := DefaultConfig()
	config.Analysis.Analyzers.Simple.Enabled = false
	config.Analysis.Analyzers.Jieba.Enabled = false

	_, err := BuildAnalyzerRegistry(config, AnalyzerResolverOptions{})
	if err == nil {
		t.Fatal("expected error when all analyzers are disabled")
	}
}

func analyzerInput(text string) analyzer.AnalyzeInput {
	return analyzer.AnalyzeInput{
		Text: text,
		Mode: analyzer.ModeQuery,
	}
}
