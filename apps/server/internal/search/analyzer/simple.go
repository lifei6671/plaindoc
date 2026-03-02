package analyzer

import (
	"context"
	"strings"
	"sync"
	"unicode"
)

const simpleAnalyzerName = "simple"

// SimpleAnalyzer 提供零依赖分词实现，主要用于兜底与开发阶段。
type SimpleAnalyzer struct {
	mu          sync.RWMutex
	dictVersion string
}

// NewSimpleAnalyzer 创建 simple analyzer。
func NewSimpleAnalyzer(dictVersion string) *SimpleAnalyzer {
	return &SimpleAnalyzer{
		dictVersion: strings.TrimSpace(dictVersion),
	}
}

func (a *SimpleAnalyzer) Name() string {
	return simpleAnalyzerName
}

func (a *SimpleAnalyzer) Health(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func (a *SimpleAnalyzer) AnalyzeForIndex(ctx context.Context, input AnalyzeInput) (AnalyzeOutput, error) {
	if err := ctx.Err(); err != nil {
		return AnalyzeOutput{}, err
	}
	return a.analyze(input), nil
}

func (a *SimpleAnalyzer) AnalyzeForQuery(ctx context.Context, input AnalyzeInput) (AnalyzeOutput, error) {
	if err := ctx.Err(); err != nil {
		return AnalyzeOutput{}, err
	}
	return a.analyze(input), nil
}

func (a *SimpleAnalyzer) Reload(ctx context.Context, dictVersion string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	a.mu.Lock()
	a.dictVersion = strings.TrimSpace(dictVersion)
	a.mu.Unlock()
	return nil
}

func (a *SimpleAnalyzer) Capabilities() Capabilities {
	return Capabilities{
		SupportsUserDict:   false,
		SupportsHotReload:  true,
		SupportsPhraseHint: false,
		SupportsStopwords:  false,
		SupportsSynonyms:   false,
	}
}

func (a *SimpleAnalyzer) analyze(input AnalyzeInput) AnalyzeOutput {
	normalizedText := NormalizeMarkdownToPlainText(input.Text)
	tokens := tokenizeSimple(normalizedText)
	return AnalyzeOutput{
		Tokens:         tokens,
		NormalizedText: normalizedText,
		TokenCount:     len(tokens),
		DictVersion:    a.getDictVersion(),
	}
}

func (a *SimpleAnalyzer) getDictVersion() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.dictVersion
}

func tokenizeSimple(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{}
	}

	lowered := strings.ToLower(value)
	tokens := make([]string, 0, 32)
	var current strings.Builder

	flushCurrent := func() {
		if current.Len() == 0 {
			return
		}
		token := strings.TrimSpace(current.String())
		current.Reset()
		if token == "" {
			return
		}
		tokens = append(tokens, token)
	}

	for _, r := range lowered {
		switch {
		case isHanRune(r):
			flushCurrent()
			tokens = append(tokens, string(r))
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			current.WriteRune(r)
		default:
			flushCurrent()
		}
	}
	flushCurrent()

	return dedupeAndTrimTokens(tokens)
}

func dedupeAndTrimTokens(input []string) []string {
	if len(input) == 0 {
		return []string{}
	}
	result := make([]string, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, token := range input {
		normalized := strings.TrimSpace(token)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}
