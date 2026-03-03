package provider

import (
	"strings"
	"testing"
)

func TestBuildKeywordWindowSnippet_ContainsMatchedKeyword(t *testing.T) {
	plain := strings.Repeat("a", 230) + "keyword-match" + strings.Repeat("b", 90)
	snippet := buildKeywordWindowSnippet(plain, []string{"keyword-match"})
	if snippet == "" {
		t.Fatal("expected non-empty snippet")
	}
	if !strings.Contains(strings.ToLower(snippet), "keyword-match") {
		t.Fatalf("expected snippet contains keyword-match, got=%q", snippet)
	}
	if !strings.HasPrefix(snippet, "...") {
		t.Fatalf("expected snippet starts with ellipsis for late match, got=%q", snippet)
	}
}

func TestBuildKeywordWindowSnippet_FallbackToPrefixWhenNoKeywordMatched(t *testing.T) {
	plain := strings.Repeat("x", 260)
	snippet := buildKeywordWindowSnippet(plain, []string{"not-found"})
	if !strings.HasSuffix(snippet, "...") {
		t.Fatalf("expected fallback snippet has suffix ellipsis, got=%q", snippet)
	}
	if strings.Contains(snippet, "not-found") {
		t.Fatalf("expected fallback snippet does not contain missing keyword, got=%q", snippet)
	}
}

func TestBuildKeywordWindowSnippetFromTitleAndBody_CoversTitleHit(t *testing.T) {
	title := "仅标题包含关键字Alpha"
	body := strings.Repeat("正文内容", 120)
	snippet := buildKeywordWindowSnippetFromTitleAndBody(title, body, []string{"alpha"})
	if snippet == "" {
		t.Fatal("expected non-empty snippet")
	}
	if !strings.Contains(strings.ToLower(snippet), "alpha") {
		t.Fatalf("expected snippet contains title keyword alpha, got=%q", snippet)
	}
}

func TestBuildKeywordWindowSnippet_PrioritizesCompoundKeyword(t *testing.T) {
	plain := strings.Repeat("doc ", 90) + "doc_visibility_level marker"
	keywords := []string{"doc_visibility_level", "doc"}
	snippet := buildKeywordWindowSnippet(plain, keywords)
	if !strings.Contains(strings.ToLower(snippet), "doc_visibility_level") {
		t.Fatalf("expected snippet prefers compound token window, got=%q", snippet)
	}
}

func TestBuildKeywordWindowSnippet_PrioritizesCompoundVariantKeyword(t *testing.T) {
	plain := strings.Repeat("doc ", 90) + "doc-visibility-level marker"
	keywords := buildSearchSnippetKeywords("doc visibility level", "doc_visibility_level")
	snippet := buildKeywordWindowSnippet(plain, keywords)
	if !strings.Contains(strings.ToLower(snippet), "doc-visibility-level") {
		t.Fatalf("expected snippet prefers compound variant window, got=%q", snippet)
	}
}
