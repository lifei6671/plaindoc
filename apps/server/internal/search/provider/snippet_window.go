package provider

import (
	"strings"
	"unicode/utf8"

	searchanalyzer "github.com/lifei6671/plaindoc/apps/server/internal/search/analyzer"
)

const (
	defaultSearchSnippetRuneLimit   = 200
	defaultSearchSnippetLeadContext = 64
)

func buildKeywordWindowSnippetFromTitleAndBody(
	title string,
	body string,
	keywords []string,
) string {
	merged := mergeSnippetSourceText(title, body)
	if merged == "" {
		return ""
	}
	plain := strings.TrimSpace(searchanalyzer.NormalizeMarkdownToPlainText(merged))
	return buildKeywordWindowSnippet(plain, keywords)
}

func mergeSnippetSourceText(title string, body string) string {
	normalizedTitle := strings.TrimSpace(title)
	normalizedBody := strings.TrimSpace(body)
	if normalizedTitle == "" && normalizedBody == "" {
		return ""
	}
	if normalizedTitle == "" {
		return normalizedBody
	}
	if normalizedBody == "" {
		return normalizedTitle
	}
	return normalizedTitle + "\n" + normalizedBody
}

func buildKeywordWindowSnippet(plain string, keywords []string) string {
	normalizedPlain := strings.TrimSpace(plain)
	if normalizedPlain == "" {
		return ""
	}
	runes := []rune(normalizedPlain)
	if len(runes) <= defaultSearchSnippetRuneLimit {
		return normalizedPlain
	}

	startRuneIndex, endRuneIndex, matched := locateFirstKeywordRuneRange(normalizedPlain, keywords)
	if !matched {
		return strings.TrimSpace(string(runes[:defaultSearchSnippetRuneLimit])) + "..."
	}

	windowStart := startRuneIndex - defaultSearchSnippetLeadContext
	if windowStart < 0 {
		windowStart = 0
	}
	windowEnd := windowStart + defaultSearchSnippetRuneLimit
	if windowEnd < endRuneIndex {
		windowEnd = endRuneIndex
	}
	if windowEnd > len(runes) {
		windowEnd = len(runes)
		windowStart = windowEnd - defaultSearchSnippetRuneLimit
		if windowStart < 0 {
			windowStart = 0
		}
	}

	snippet := strings.TrimSpace(string(runes[windowStart:windowEnd]))
	if snippet == "" {
		return ""
	}
	if windowStart > 0 {
		snippet = "..." + snippet
	}
	if windowEnd < len(runes) {
		snippet = snippet + "..."
	}
	return snippet
}

func locateFirstKeywordRuneRange(plain string, keywords []string) (int, int, bool) {
	normalizedKeywords := normalizeSnippetKeywords(keywords)
	if len(normalizedKeywords) == 0 {
		return 0, 0, false
	}

	lowerPlain := strings.ToLower(plain)
	matchedStartByte := -1
	matchedEndByte := -1
	for _, keyword := range normalizedKeywords {
		startByte := strings.Index(lowerPlain, keyword)
		if startByte < 0 {
			continue
		}
		endByte := startByte + len(keyword)
		if matchedStartByte < 0 || startByte < matchedStartByte {
			matchedStartByte = startByte
			matchedEndByte = endByte
		}
	}
	if matchedStartByte < 0 {
		return 0, 0, false
	}

	startRuneIndex := utf8.RuneCountInString(lowerPlain[:matchedStartByte])
	endRuneIndex := startRuneIndex + utf8.RuneCountInString(lowerPlain[matchedStartByte:matchedEndByte])
	if endRuneIndex <= startRuneIndex {
		return 0, 0, false
	}
	return startRuneIndex, endRuneIndex, true
}

func normalizeSnippetKeywords(keywords []string) []string {
	if len(keywords) == 0 {
		return []string{}
	}
	result := make([]string, 0, len(keywords))
	seen := make(map[string]struct{}, len(keywords))
	for _, item := range keywords {
		keyword := strings.ToLower(strings.TrimSpace(item))
		if keyword == "" {
			continue
		}
		if _, exists := seen[keyword]; exists {
			continue
		}
		seen[keyword] = struct{}{}
		result = append(result, keyword)
	}
	return result
}
