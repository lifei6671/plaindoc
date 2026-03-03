package view

import (
	"strings"
	"testing"
)

func TestHighlightSearchText_HighlightsAndEscapes(t *testing.T) {
	output := string(highlightSearchText(`Go <script>alert(1)</script> test`, "script"))
	if !strings.Contains(output, `<mark class="yt-search-highlight">script</mark>`) {
		t.Fatalf("expected highlighted script token, got=%q", output)
	}
	if strings.Contains(output, "<script>") {
		t.Fatalf("expected escaped html output, got=%q", output)
	}
	if !strings.Contains(output, "&lt;") {
		t.Fatalf("expected escaped angle bracket, got=%q", output)
	}
}

func TestHighlightSearchText_SupportsPhraseAndTokens(t *testing.T) {
	output := string(highlightSearchText("matrix hit example", "matrix hit"))
	if !strings.Contains(output, `<mark class="yt-search-highlight">matrix hit</mark>`) {
		t.Fatalf("expected phrase highlight, got=%q", output)
	}
}

func TestHighlightSearchText_EmptyKeywordReturnsEscapedText(t *testing.T) {
	output := string(highlightSearchText("<b>raw</b>", ""))
	if output != "&lt;b&gt;raw&lt;/b&gt;" {
		t.Fatalf("expected escaped original text, got=%q", output)
	}
}

func TestHighlightSearchText_SupportsChineseBigrams(t *testing.T) {
	output := string(highlightSearchText("正文包含全文索引权限内容", "全文检索权限"))
	if !strings.Contains(output, `<mark class="yt-search-highlight">全文</mark>`) {
		t.Fatalf("expected output contains highlighted 全文, got=%q", output)
	}
	if !strings.Contains(output, `<mark class="yt-search-highlight">权限</mark>`) {
		t.Fatalf("expected output contains highlighted 权限, got=%q", output)
	}
}
