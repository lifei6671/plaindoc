package analyzer

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

var (
	frontMatterRegexp   = regexp.MustCompile(`(?s)\A---\s*\n.*?\n---\s*`)
	blockMathRegexp     = regexp.MustCompile(`(?s)\$\$.*?\$\$`)
	inlineMathRegexp    = regexp.MustCompile(`\$(?:\\.|[^$\n])+\$`)
	compoundIdentRegexp = regexp.MustCompile(`[A-Za-z0-9]+(?:_[A-Za-z0-9]+)+`)
	whitespaceRegexp    = regexp.MustCompile(`\s+`)
)

const protectedUnderscoreRune = '\uFF3F'

// NormalizeMarkdownToPlainText 将 Markdown 清洗为可检索纯文本。
//
// 清洗规则：
// 1) 删除 front matter、块级/行内公式；
// 2) 解析 Markdown AST，忽略代码块、行内代码、HTML 块；
// 3) 仅提取自然语言文本节点，并做空白归一化。
func NormalizeMarkdownToPlainText(markdown string) string {
	source := preprocessMarkdown(markdown)
	if strings.TrimSpace(source) == "" {
		return ""
	}

	markdownParser := goldmark.New().Parser()
	sourceBytes := []byte(source)
	reader := text.NewReader(sourceBytes)
	document := markdownParser.Parse(reader)

	segments := make([]string, 0, 64)
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if shouldIgnoreNode(node) {
			return ast.WalkSkipChildren, nil
		}

		switch current := node.(type) {
		case *ast.Text:
			value := string(current.Segment.Value(sourceBytes))
			value = strings.TrimSpace(value)
			if value != "" {
				segments = append(segments, value)
			}
		case *ast.String:
			value := strings.TrimSpace(string(current.Value))
			if value != "" {
				segments = append(segments, value)
			}
		}
		return ast.WalkContinue, nil
	})

	joined := strings.Join(segments, " ")
	return restoreCompoundIdentifiers(normalizeWhitespace(joined))
}

func preprocessMarkdown(markdown string) string {
	normalized := strings.ReplaceAll(markdown, "\r\n", "\n")
	normalized = frontMatterRegexp.ReplaceAllString(normalized, "")
	normalized = blockMathRegexp.ReplaceAllString(normalized, " ")
	normalized = inlineMathRegexp.ReplaceAllString(normalized, " ")
	normalized = protectCompoundIdentifiers(normalized)
	return normalized
}

func shouldIgnoreNode(node ast.Node) bool {
	switch node.(type) {
	case *ast.CodeBlock, *ast.FencedCodeBlock, *ast.CodeSpan, *ast.HTMLBlock, *ast.RawHTML:
		return true
	default:
		return false
	}
}

func normalizeWhitespace(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	trimmed = whitespaceRegexp.ReplaceAllString(trimmed, " ")
	return strings.TrimSpace(trimmed)
}

func protectCompoundIdentifiers(value string) string {
	if strings.TrimSpace(value) == "" {
		return value
	}
	replacement := string(protectedUnderscoreRune)
	return compoundIdentRegexp.ReplaceAllStringFunc(value, func(token string) string {
		return strings.ReplaceAll(token, "_", replacement)
	})
}

func restoreCompoundIdentifiers(value string) string {
	if strings.TrimSpace(value) == "" {
		return value
	}
	return strings.ReplaceAll(value, string(protectedUnderscoreRune), "_")
}

func isHanRune(r rune) bool {
	return unicode.Is(unicode.Han, r)
}
