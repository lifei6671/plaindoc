package analyzer

import (
	"context"
	"slices"
	"testing"
)

func TestSimpleAnalyzer_AnalyzeForIndex_UsesNormalizedMarkdown(t *testing.T) {
	provider := NewSimpleAnalyzer("dict-v1")
	output, err := provider.AnalyzeForIndex(context.Background(), AnalyzeInput{
		Text: "# 你好，World\n\n`drop_me`\n\n$E=mc^2$",
		Mode: ModeIndex,
	})
	if err != nil {
		t.Fatalf("analyze for index failed: %v", err)
	}

	if output.TokenCount == 0 {
		t.Fatalf("expected non-empty tokens")
	}
	if output.DictVersion != "dict-v1" {
		t.Fatalf("expected dict version dict-v1, got %q", output.DictVersion)
	}
	if slices.Contains(output.Tokens, "drop_me") {
		t.Fatalf("unexpected token drop_me in %v", output.Tokens)
	}
	if slices.Contains(output.Tokens, "e") || slices.Contains(output.Tokens, "mc") {
		t.Fatalf("unexpected math tokens in %v", output.Tokens)
	}
	if !slices.Contains(output.Tokens, "你") || !slices.Contains(output.Tokens, "好") {
		t.Fatalf("expected Chinese tokens in %v", output.Tokens)
	}
	if !slices.Contains(output.Tokens, "world") {
		t.Fatalf("expected world token in %v", output.Tokens)
	}
}

func TestSimpleAnalyzer_ReloadUpdatesDictVersion(t *testing.T) {
	provider := NewSimpleAnalyzer("old-version")
	if err := provider.Reload(context.Background(), "new-version"); err != nil {
		t.Fatalf("reload failed: %v", err)
	}

	output, err := provider.AnalyzeForQuery(context.Background(), AnalyzeInput{
		Text: "hello",
		Mode: ModeQuery,
	})
	if err != nil {
		t.Fatalf("analyze for query failed: %v", err)
	}
	if output.DictVersion != "new-version" {
		t.Fatalf("expected dict version new-version, got %q", output.DictVersion)
	}
}
