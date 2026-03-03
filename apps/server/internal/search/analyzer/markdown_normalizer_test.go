package analyzer

import (
	"strings"
	"testing"
)

func TestNormalizeMarkdownToPlainText_RemovesCodeMathAndSyntax(t *testing.T) {
	input := strings.Join([]string{
		"# 文档标题",
		"",
		"这是 **正文**，包含 [链接文本](https://example.com) 和 ![图片说明](https://example.com/image.png)。",
		"",
		"`inline_identifier_kept`",
		"",
		"```go",
		"fmt.Println(\"code_should_not_appear\")",
		"```",
		"",
		"```mermaid",
		"graph TD",
		"A-->B",
		"```",
		"",
		"行内公式 $E=mc^2$ 不应进入索引。",
		"",
		"$$",
		"a^2+b^2=c^2",
		"$$",
	}, "\n")

	normalized := NormalizeMarkdownToPlainText(input)

	mustContain := []string{
		"文档标题",
		"这是",
		"正文",
		"链接文本",
		"图片说明",
		"inline_identifier_kept",
		"不应进入索引",
	}
	for _, expected := range mustContain {
		if !strings.Contains(normalized, expected) {
			t.Fatalf("expected normalized content to contain %q, got %q", expected, normalized)
		}
	}

	mustNotContain := []string{
		"code_should_not_appear",
		"graph TD",
		"A-->B",
		"E=mc^2",
		"a^2+b^2=c^2",
		"```",
		"#",
		"[",
		"](",
	}
	for _, unexpected := range mustNotContain {
		if strings.Contains(normalized, unexpected) {
			t.Fatalf("expected normalized content not to contain %q, got %q", unexpected, normalized)
		}
	}
}

func TestNormalizeMarkdownToPlainText_EmptyInput(t *testing.T) {
	normalized := NormalizeMarkdownToPlainText("   \n\n")
	if normalized != "" {
		t.Fatalf("expected empty normalized output, got %q", normalized)
	}
}

func TestNormalizeMarkdownToPlainText_PreservesCompoundIdentifier(t *testing.T) {
	input := strings.Join([]string{
		"字段名是 doc_visibility_level，需要参与检索。",
		"",
		"`doc_visibility_level_in_code`",
	}, "\n")

	normalized := NormalizeMarkdownToPlainText(input)
	if !strings.Contains(normalized, "doc_visibility_level") {
		t.Fatalf("expected normalized content to contain %q, got %q", "doc_visibility_level", normalized)
	}
	if !strings.Contains(normalized, "doc_visibility_level_in_code") {
		t.Fatalf("expected normalized content to contain inline code token, got %q", normalized)
	}
}
