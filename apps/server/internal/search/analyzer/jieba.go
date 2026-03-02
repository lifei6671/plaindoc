package analyzer

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

const jiebaAnalyzerName = "jieba"

// JiebaOptions 定义 jieba analyzer 初始化参数。
type JiebaOptions struct {
	DictVersion     string
	UserDictEntries []string
	EnableHMM       bool
}

// JiebaAnalyzer 提供 jieba 风格分词能力，支持用户词典热重载。
//
// 说明：
// 1) 当前实现不依赖 cgo，可在现有构建环境直接运行；
// 2) 用户词典遵循 jieba 用户词典格式（term [weight] [tag]）；
// 3) 后续可在此基础上增加 gojieba/cppjieba 适配实现。
type JiebaAnalyzer struct {
	mu          sync.RWMutex
	dictVersion string
	userEntries []string
	dictTerms   []string
	enableHMM   bool
}

// NewJiebaAnalyzer 创建 jieba analyzer。
func NewJiebaAnalyzer(options JiebaOptions) (*JiebaAnalyzer, error) {
	analyzer := &JiebaAnalyzer{
		dictVersion: strings.TrimSpace(options.DictVersion),
		userEntries: normalizeDictEntries(options.UserDictEntries),
		enableHMM:   options.EnableHMM,
	}
	if analyzer.dictVersion == "" {
		analyzer.dictVersion = "default"
	}
	if !options.EnableHMM {
		analyzer.enableHMM = false
	} else {
		analyzer.enableHMM = true
	}
	analyzer.dictTerms = parseAndSortDictTerms(analyzer.userEntries)
	return analyzer, nil
}

func (a *JiebaAnalyzer) Name() string {
	return jiebaAnalyzerName
}

func (a *JiebaAnalyzer) Health(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func (a *JiebaAnalyzer) AnalyzeForIndex(ctx context.Context, input AnalyzeInput) (AnalyzeOutput, error) {
	if err := ctx.Err(); err != nil {
		return AnalyzeOutput{}, err
	}
	return a.analyze(input)
}

func (a *JiebaAnalyzer) AnalyzeForQuery(ctx context.Context, input AnalyzeInput) (AnalyzeOutput, error) {
	if err := ctx.Err(); err != nil {
		return AnalyzeOutput{}, err
	}
	return a.analyze(input)
}

func (a *JiebaAnalyzer) Reload(ctx context.Context, dictVersion string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	normalizedVersion := strings.TrimSpace(dictVersion)
	if normalizedVersion == "" {
		normalizedVersion = "default"
	}
	a.dictVersion = normalizedVersion
	a.dictTerms = parseAndSortDictTerms(a.userEntries)
	return nil
}

// UpdateUserDictEntries 更新用户词典并触发重载。
func (a *JiebaAnalyzer) UpdateUserDictEntries(ctx context.Context, entries []string, dictVersion string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	normalizedEntries := normalizeDictEntries(entries)
	normalizedVersion := strings.TrimSpace(dictVersion)
	if normalizedVersion == "" {
		normalizedVersion = "default"
	}

	a.mu.Lock()
	a.userEntries = normalizedEntries
	a.dictTerms = parseAndSortDictTerms(normalizedEntries)
	a.dictVersion = normalizedVersion
	a.mu.Unlock()
	return nil
}

func (a *JiebaAnalyzer) Capabilities() Capabilities {
	return Capabilities{
		SupportsUserDict:   true,
		SupportsHotReload:  true,
		SupportsPhraseHint: false,
		SupportsStopwords:  false,
		SupportsSynonyms:   false,
	}
}

func (a *JiebaAnalyzer) analyze(input AnalyzeInput) (AnalyzeOutput, error) {
	a.mu.RLock()
	dictVersion := a.dictVersion
	dictTerms := append([]string(nil), a.dictTerms...)
	enableHMM := a.enableHMM
	a.mu.RUnlock()

	normalizedText := NormalizeMarkdownToPlainText(input.Text)
	rawTokens := tokenizeJiebaStyle(normalizedText, dictTerms, enableHMM)
	tokens := dedupeAndTrimTokens(rawTokens)

	return AnalyzeOutput{
		Tokens:         tokens,
		NormalizedText: normalizedText,
		TokenCount:     len(tokens),
		DictVersion:    dictVersion,
	}, nil
}

func tokenizeJiebaStyle(text string, dictTerms []string, enableHMM bool) []string {
	if strings.TrimSpace(text) == "" {
		return []string{}
	}

	tokens := make([]string, 0, 32)
	var current strings.Builder

	flushCurrent := func() {
		if current.Len() == 0 {
			return
		}
		word := strings.TrimSpace(current.String())
		current.Reset()
		if word != "" {
			tokens = append(tokens, strings.ToLower(word))
		}
	}

	for index := 0; index < len(text); {
		r, size := utf8.DecodeRuneInString(text[index:])
		if r == utf8.RuneError && size == 1 {
			flushCurrent()
			index += size
			continue
		}

		if isHanRune(r) {
			flushCurrent()

			matched := ""
			for _, term := range dictTerms {
				if strings.HasPrefix(text[index:], term) {
					matched = term
					break
				}
			}
			if matched != "" {
				tokens = append(tokens, matched)
				index += len(matched)
				continue
			}

			if enableHMM {
				// HMM 开启时，连续汉字段做二元切分，提升短语命中概率。
				nextIndex := index + size
				nextRune, nextSize := utf8.DecodeRuneInString(text[nextIndex:])
				if nextRune != utf8.RuneError && isHanRune(nextRune) {
					bigram := text[index : nextIndex+nextSize]
					tokens = append(tokens, bigram)
				}
			}
			tokens = append(tokens, string(r))
			index += size
			continue
		}

		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(r)
			index += size
			continue
		}

		flushCurrent()
		index += size
	}
	flushCurrent()

	return tokens
}

func parseAndSortDictTerms(entries []string) []string {
	if len(entries) == 0 {
		return []string{}
	}

	terms := make([]string, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		fields := strings.Fields(entry)
		if len(fields) == 0 {
			continue
		}
		term := strings.TrimSpace(fields[0])
		if term == "" {
			continue
		}
		if _, exists := seen[term]; exists {
			continue
		}
		seen[term] = struct{}{}
		terms = append(terms, term)
	}

	sort.SliceStable(terms, func(i, j int) bool {
		leftLength := utf8.RuneCountInString(terms[i])
		rightLength := utf8.RuneCountInString(terms[j])
		if leftLength == rightLength {
			return terms[i] < terms[j]
		}
		return leftLength > rightLength
	})
	return terms
}

func normalizeDictEntries(entries []string) []string {
	if len(entries) == 0 {
		return []string{}
	}

	result := make([]string, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		normalized := strings.TrimSpace(entry)
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

// FormatJiebaDictEntry 将词条规范为 jieba 用户词典行。
//
// 支持：
// - term
// - term weight
// - term weight tag
func FormatJiebaDictEntry(term string, weight int, tag string) (string, error) {
	normalizedTerm := strings.TrimSpace(term)
	if normalizedTerm == "" {
		return "", errors.New("jieba dict term is empty")
	}
	if strings.ContainsAny(normalizedTerm, "\n\r\t") {
		return "", errors.New("jieba dict term contains invalid whitespace")
	}

	parts := []string{normalizedTerm}
	if weight > 0 {
		parts = append(parts, fmt.Sprintf("%d", weight))
	}
	normalizedTag := strings.TrimSpace(tag)
	if normalizedTag != "" {
		if weight <= 0 {
			parts = append(parts, "1")
		}
		parts = append(parts, normalizedTag)
	}
	return strings.Join(parts, " "), nil
}
