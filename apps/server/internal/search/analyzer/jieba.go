package analyzer

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"unicode"

	"github.com/go-ego/gse"
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
	enableHMM   bool
	segmenter   *gse.Segmenter
}

// NewJiebaAnalyzer 创建 jieba analyzer。
func NewJiebaAnalyzer(options JiebaOptions) (*JiebaAnalyzer, error) {
	normalizedEntries := normalizeDictEntries(options.UserDictEntries)
	segmenter, err := buildJiebaSegmenter(normalizedEntries)
	if err != nil {
		return nil, err
	}

	analyzer := &JiebaAnalyzer{
		dictVersion: strings.TrimSpace(options.DictVersion),
		userEntries: normalizedEntries,
		enableHMM:   options.EnableHMM,
		segmenter:   segmenter,
	}
	if analyzer.dictVersion == "" {
		analyzer.dictVersion = "default"
	}
	if !options.EnableHMM {
		analyzer.enableHMM = false
	} else {
		analyzer.enableHMM = true
	}
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

	a.mu.RLock()
	entries := append([]string(nil), a.userEntries...)
	a.mu.RUnlock()
	segmenter, err := buildJiebaSegmenter(entries)
	if err != nil {
		return err
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	normalizedVersion := strings.TrimSpace(dictVersion)
	if normalizedVersion == "" {
		normalizedVersion = "default"
	}
	a.dictVersion = normalizedVersion
	a.segmenter = segmenter
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
	segmenter, err := buildJiebaSegmenter(normalizedEntries)
	if err != nil {
		return err
	}

	a.mu.Lock()
	a.userEntries = normalizedEntries
	a.segmenter = segmenter
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
	enableHMM := a.enableHMM
	segmenter := a.segmenter
	a.mu.RUnlock()
	if segmenter == nil {
		return AnalyzeOutput{}, errors.New("jieba segmenter is nil")
	}

	normalizedText := NormalizeMarkdownToPlainText(input.Text)
	if strings.TrimSpace(normalizedText) == "" {
		return AnalyzeOutput{
			Tokens:         []string{},
			NormalizedText: normalizedText,
			TokenCount:     0,
			DictVersion:    dictVersion,
		}, nil
	}
	rawTokens := segmenter.CutSearch(normalizedText, enableHMM)
	tokens := normalizeJiebaTokens(rawTokens)

	return AnalyzeOutput{
		Tokens:         tokens,
		NormalizedText: normalizedText,
		TokenCount:     len(tokens),
		DictVersion:    dictVersion,
	}, nil
}

func buildJiebaSegmenter(entries []string) (*gse.Segmenter, error) {
	segmenter := &gse.Segmenter{
		SkipLog:  true,
		AlphaNum: true,
	}
	if err := segmenter.LoadDictEmbed("zh"); err != nil {
		return nil, fmt.Errorf("load gse default dictionary: %w", err)
	}
	if len(entries) == 0 {
		return segmenter, nil
	}

	dictPayload := strings.Join(entries, "\n")
	if err := segmenter.LoadDictStr(dictPayload); err != nil {
		return nil, fmt.Errorf("load gse user dictionary: %w", err)
	}
	return segmenter, nil
}

func normalizeJiebaTokens(input []string) []string {
	if len(input) == 0 {
		return []string{}
	}

	result := make([]string, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, token := range input {
		normalized := strings.ToLower(strings.TrimSpace(token))
		if normalized == "" {
			continue
		}
		if !containsWordRune(normalized) {
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

func containsWordRune(value string) bool {
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
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
