package provider

import (
	"reflect"
	"testing"
)

func TestBuildSearchQueryTerms_DropsSingleRuneWhenMultiRuneExists(t *testing.T) {
	terms := buildSearchQueryTerms("全文 全 文检 文 检索 检 索权 索 权限 权 限")
	expected := []string{"全文", "文检", "检索", "索权", "权限"}
	if !reflect.DeepEqual(expected, terms) {
		t.Fatalf("expected terms=%v, got=%v", expected, terms)
	}
}

func TestResolveTokenMinShouldMatch(t *testing.T) {
	cases := []struct {
		count    int
		expected int
	}{
		{count: 1, expected: 1},
		{count: 2, expected: 1},
		{count: 3, expected: 1},
		{count: 4, expected: 2},
		{count: 5, expected: 2},
		{count: 8, expected: 3},
	}
	for _, item := range cases {
		got := resolveTokenMinShouldMatch(item.count)
		if got != item.expected {
			t.Fatalf("count=%d expected=%d got=%d", item.count, item.expected, got)
		}
	}
}

func TestBuildSearchQueryTermsWithRaw_IncludesCompoundLiteral(t *testing.T) {
	terms := buildSearchQueryTermsWithRaw("doc visibility level", "doc_visibility_level")
	expected := []string{"doc", "visibility", "level", "doc_visibility_level"}
	if !reflect.DeepEqual(expected, terms) {
		t.Fatalf("expected terms=%v, got=%v", expected, terms)
	}
}

func TestExtractCompoundLiteralTokens(t *testing.T) {
	terms := ExtractCompoundLiteralTokens(" [doc_visibility_level], abc x-y-z test ")
	expected := []string{"doc_visibility_level", "x-y-z"}
	if !reflect.DeepEqual(expected, terms) {
		t.Fatalf("expected terms=%v, got=%v", expected, terms)
	}
}

func TestBuildSearchSnippetKeywords_PrioritizesCompoundLiteral(t *testing.T) {
	terms := buildSearchSnippetKeywords("doc visibility level", "doc_visibility_level")
	expected := []string{
		"doc_visibility_level",
		"doc visibility level",
		"doc-visibility-level",
		"doc.visibility.level",
		"doc/visibility/level",
		"doc:visibility:level",
		"docvisibilitylevel",
		"doc",
		"visibility",
		"level",
	}
	if !reflect.DeepEqual(expected, terms) {
		t.Fatalf("expected terms=%v, got=%v", expected, terms)
	}
}
