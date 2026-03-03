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
