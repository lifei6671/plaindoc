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
