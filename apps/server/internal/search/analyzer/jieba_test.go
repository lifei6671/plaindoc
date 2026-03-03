package analyzer

import (
	"context"
	"slices"
	"strings"
	"testing"
)

func TestJiebaAnalyzer_AnalyzeForIndex_UsesNormalizedMarkdown(t *testing.T) {
	provider, err := NewJiebaAnalyzer(JiebaOptions{
		DictVersion: "dict-v1",
		EnableHMM:   true,
	})
	if err != nil {
		t.Fatalf("create jieba analyzer failed: %v", err)
	}

	output, err := provider.AnalyzeForIndex(context.Background(), AnalyzeInput{
		Text: "# Hello **World**\n\n`drop_me`\n\n$E=mc^2$",
		Mode: ModeIndex,
	})
	if err != nil {
		t.Fatalf("analyze for index failed: %v", err)
	}

	if output.NormalizedText != "Hello World drop_me" {
		t.Fatalf("expected normalized text %q, got %q", "Hello World drop_me", output.NormalizedText)
	}
	if !strings.Contains(output.NormalizedText, "drop_me") {
		t.Fatalf("expected normalized text to preserve inline code token, got %q", output.NormalizedText)
	}
	if output.DictVersion != "dict-v1" {
		t.Fatalf("expected dict version dict-v1, got %q", output.DictVersion)
	}
	if len(output.Tokens) == 0 {
		t.Fatalf("expected non-empty tokens")
	}
}

func TestJiebaAnalyzer_UpdateUserDictEntries_RebuildsTerms(t *testing.T) {
	provider, err := NewJiebaAnalyzer(JiebaOptions{
		DictVersion: "dict-v1",
		EnableHMM:   true,
	})
	if err != nil {
		t.Fatalf("create jieba analyzer failed: %v", err)
	}

	if err := provider.UpdateUserDictEntries(
		context.Background(),
		[]string{"微服务架构 200 n", "领域驱动设计 180 n"},
		"dict-v2",
	); err != nil {
		t.Fatalf("update user dict entries failed: %v", err)
	}

	output, err := provider.AnalyzeForQuery(context.Background(), AnalyzeInput{
		Text: "我们今天讨论微服务架构和领域驱动设计",
		Mode: ModeQuery,
	})
	if err != nil {
		t.Fatalf("analyze for query failed: %v", err)
	}

	if output.DictVersion != "dict-v2" {
		t.Fatalf("expected dict version dict-v2, got %q", output.DictVersion)
	}
	if !slices.Contains(output.Tokens, "微服务架构") {
		t.Fatalf("expected token 微服务架构 in %v", output.Tokens)
	}
	if !slices.Contains(output.Tokens, "领域驱动设计") {
		t.Fatalf("expected token 领域驱动设计 in %v", output.Tokens)
	}
}

func TestJiebaAnalyzer_AnalyzeForQuery_MatchesASCIICompoundDictTerm(t *testing.T) {
	provider, err := NewJiebaAnalyzer(JiebaOptions{
		DictVersion:     "dict-v3",
		UserDictEntries: []string{"doc_visibility_level 500 nz"},
		EnableHMM:       true,
	})
	if err != nil {
		t.Fatalf("create jieba analyzer failed: %v", err)
	}

	output, err := provider.AnalyzeForQuery(context.Background(), AnalyzeInput{
		Text: "字段 doc_visibility_level 需要排序",
		Mode: ModeQuery,
	})
	if err != nil {
		t.Fatalf("analyze for query failed: %v", err)
	}

	if !slices.Contains(output.Tokens, "doc_visibility_level") {
		t.Fatalf("expected token doc_visibility_level in %v", output.Tokens)
	}
	if slices.Contains(output.Tokens, "doc") || slices.Contains(output.Tokens, "visibility") || slices.Contains(output.Tokens, "level") {
		t.Fatalf("expected compound token prioritized, got split tokens %v", output.Tokens)
	}
}

func TestFormatJiebaDictEntry(t *testing.T) {
	line, err := FormatJiebaDictEntry("微服务", 200, "n")
	if err != nil {
		t.Fatalf("format dict entry failed: %v", err)
	}
	if line != "微服务 200 n" {
		t.Fatalf("expected %q, got %q", "微服务 200 n", line)
	}

	line, err = FormatJiebaDictEntry("搜索", 0, "")
	if err != nil {
		t.Fatalf("format dict entry failed: %v", err)
	}
	if line != "搜索" {
		t.Fatalf("expected %q, got %q", "搜索", line)
	}

	_, err = FormatJiebaDictEntry(" \t ", 0, "")
	if err == nil {
		t.Fatalf("expected error for empty term")
	}
}
