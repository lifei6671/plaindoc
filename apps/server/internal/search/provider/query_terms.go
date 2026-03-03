package provider

import (
	"strings"
	"unicode/utf8"
)

func buildSearchQueryTerms(query string) []string {
	terms := splitQueryTerms(query)
	if len(terms) == 0 {
		return []string{}
	}

	hasMultiRuneTerm := false
	for _, item := range terms {
		if utf8.RuneCountInString(item) > 1 {
			hasMultiRuneTerm = true
			break
		}
	}
	if !hasMultiRuneTerm {
		return terms
	}

	filtered := make([]string, 0, len(terms))
	for _, item := range terms {
		if utf8.RuneCountInString(item) <= 1 {
			continue
		}
		filtered = append(filtered, item)
	}
	if len(filtered) == 0 {
		return terms
	}
	return filtered
}

func buildSearchQueryTermsWithRaw(normalizedQuery string, rawQuery string) []string {
	terms := buildSearchQueryTerms(normalizedQuery)
	rawTerms := extractCompoundLiteralTokens(rawQuery)
	if len(rawTerms) == 0 {
		return terms
	}
	if len(terms) == 0 {
		return rawTerms
	}

	merged := make([]string, 0, len(terms)+len(rawTerms))
	seen := make(map[string]struct{}, len(terms)+len(rawTerms))
	for _, item := range terms {
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		merged = append(merged, item)
	}
	for _, item := range rawTerms {
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		merged = append(merged, item)
	}
	return merged
}

func buildSearchSnippetKeywords(normalizedQuery string, rawQuery string) []string {
	baseTerms := buildSearchQueryTerms(normalizedQuery)
	compoundTerms := extractCompoundLiteralTokens(rawQuery)
	compoundVariants := expandCompoundLiteralVariants(compoundTerms)

	result := make([]string, 0, len(baseTerms)+len(compoundVariants))
	seen := make(map[string]struct{}, len(baseTerms)+len(compoundVariants))
	appendUnique := func(item string) {
		term := strings.TrimSpace(strings.ToLower(item))
		if term == "" {
			return
		}
		if _, exists := seen[term]; exists {
			return
		}
		seen[term] = struct{}{}
		result = append(result, term)
	}

	// 片段定位优先：复合词（含变体） > 常规分词
	for _, item := range compoundVariants {
		appendUnique(item)
	}
	for _, item := range baseTerms {
		appendUnique(item)
	}
	return result
}

func splitQueryTerms(query string) []string {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return []string{}
	}

	fields := strings.Fields(strings.ToLower(trimmed))
	result := make([]string, 0, len(fields))
	seen := make(map[string]struct{}, len(fields))
	for _, item := range fields {
		term := strings.TrimSpace(item)
		if term == "" {
			continue
		}
		if _, exists := seen[term]; exists {
			continue
		}
		seen[term] = struct{}{}
		result = append(result, term)
	}
	return result
}

// ExtractCompoundLiteralTokens 从文本中提取带分隔符的“复合词”。
//
// 示例：
// 1) doc_visibility_level
// 2) search-index-v2
func ExtractCompoundLiteralTokens(text string) []string {
	return extractCompoundLiteralTokens(text)
}

func extractCompoundLiteralTokens(text string) []string {
	fields := strings.Fields(strings.TrimSpace(strings.ToLower(text)))
	if len(fields) == 0 {
		return []string{}
	}

	result := make([]string, 0, len(fields))
	seen := make(map[string]struct{}, len(fields))
	for _, item := range fields {
		token := normalizeCompoundLiteralToken(item)
		if token == "" {
			continue
		}
		if _, exists := seen[token]; exists {
			continue
		}
		seen[token] = struct{}{}
		result = append(result, token)
	}
	return result
}

func expandCompoundLiteralVariants(tokens []string) []string {
	if len(tokens) == 0 {
		return []string{}
	}
	result := make([]string, 0, len(tokens)*8)
	seen := make(map[string]struct{}, len(tokens)*8)
	appendUnique := func(item string) {
		term := strings.TrimSpace(strings.ToLower(item))
		if term == "" {
			return
		}
		if _, exists := seen[term]; exists {
			return
		}
		seen[term] = struct{}{}
		result = append(result, term)
	}

	for _, item := range tokens {
		base := strings.TrimSpace(strings.ToLower(item))
		if base == "" {
			continue
		}

		appendUnique(base)
		segments := splitCompoundTokenSegments(base)
		if len(segments) < 2 {
			continue
		}
		appendUnique(strings.Join(segments, " "))
		appendUnique(strings.Join(segments, "-"))
		appendUnique(strings.Join(segments, "."))
		appendUnique(strings.Join(segments, "/"))
		appendUnique(strings.Join(segments, ":"))
		appendUnique(strings.Join(segments, ""))
	}
	return result
}

func splitCompoundTokenSegments(token string) []string {
	trimmed := strings.TrimSpace(strings.ToLower(token))
	if trimmed == "" {
		return []string{}
	}

	segments := make([]string, 0, 8)
	var current strings.Builder
	flushCurrent := func() {
		if current.Len() == 0 {
			return
		}
		value := strings.TrimSpace(current.String())
		current.Reset()
		if value == "" {
			return
		}
		segments = append(segments, value)
	}

	for _, r := range trimmed {
		if isCompoundSeparator(r) {
			flushCurrent()
			continue
		}
		current.WriteRune(r)
	}
	flushCurrent()
	if len(segments) == 0 {
		return []string{}
	}
	return segments
}

func normalizeCompoundLiteralToken(token string) string {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return ""
	}

	trimmed = strings.Trim(trimmed, "()[]{}<>'\"`.,;!?")
	if trimmed == "" {
		return ""
	}

	hasSeparator := false
	segmentCount := 0
	segmentStart := 0
	for index, r := range trimmed {
		if isCompoundSeparator(r) {
			hasSeparator = true
			if index > segmentStart {
				segmentCount++
			}
			segmentStart = index + utf8.RuneLen(r)
		}
	}
	if len(trimmed) > segmentStart {
		segmentCount++
	}

	if !hasSeparator || segmentCount < 2 {
		return ""
	}
	return trimmed
}

func isCompoundSeparator(r rune) bool {
	switch r {
	case '_', '-', '.', '/', '\\', ':':
		return true
	default:
		return false
	}
}

func resolveTokenMinShouldMatch(termCount int) int {
	if termCount <= 0 {
		return 0
	}
	min := (termCount + 2) / 3 // ceil(termCount/3)
	if min < 1 {
		return 1
	}
	if min > termCount {
		return termCount
	}
	return min
}
