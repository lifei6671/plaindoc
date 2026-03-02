package search

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

const (
	// SystemConfigKey 为全文检索系统配置键。
	SystemConfigKey = "search"
	// DefaultDictVersion 为分词器默认词典版本。
	DefaultDictVersion = "default"
)

// ProviderName 定义检索引擎名称。
type ProviderName string

const (
	ProviderBleve     ProviderName = "bleve"
	ProviderMeili     ProviderName = "meili"
	ProviderTypesense ProviderName = "typesense"
	ProviderDatabase  ProviderName = "database"
)

// FallbackPolicy 定义 active provider 不可用时策略。
type FallbackPolicy string

const (
	FallbackPolicyDegradeToBleve FallbackPolicy = "degrade_to_bleve"
	FallbackPolicyReturnError    FallbackPolicy = "return_error"
)

// AnalyzerName 定义分词器名称。
type AnalyzerName string

const (
	AnalyzerSimple AnalyzerName = "simple"
	AnalyzerJieba  AnalyzerName = "jieba"
)

// JiebaMode 定义 jieba 分词模式。
type JiebaMode string

const (
	JiebaModeSearch JiebaMode = "search"
)

// JiebaDictSource 定义 jieba 用户词典来源。
type JiebaDictSource string

const (
	JiebaDictSourceDB   JiebaDictSource = "db"
	JiebaDictSourceFile JiebaDictSource = "file"
)

var dictVersionRegexp = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:-]{0,63}$`)

// Config 定义全文检索系统配置。
type Config struct {
	Enabled        bool           `json:"enabled"`
	ActiveProvider ProviderName   `json:"activeProvider"`
	FallbackPolicy FallbackPolicy `json:"fallbackPolicy"`
	Analysis       AnalysisConfig `json:"analysis"`
}

// AnalysisConfig 定义分词策略配置。
type AnalysisConfig struct {
	ActiveAnalyzer AnalyzerName    `json:"activeAnalyzer"`
	Analyzers      AnalyzerConfigs `json:"analyzers"`
}

// AnalyzerConfigs 聚合所有分词器配置。
type AnalyzerConfigs struct {
	Simple SimpleAnalyzerConfig `json:"simple"`
	Jieba  JiebaAnalyzerConfig  `json:"jieba"`
}

// SimpleAnalyzerConfig 定义 simple 分词器配置。
type SimpleAnalyzerConfig struct {
	Enabled bool `json:"enabled"`
}

// JiebaAnalyzerConfig 定义 jieba 分词器配置。
type JiebaAnalyzerConfig struct {
	Enabled          bool            `json:"enabled"`
	Mode             JiebaMode       `json:"mode"`
	HMM              bool            `json:"hmm"`
	StopwordsEnabled bool            `json:"stopwordsEnabled"`
	DictSource       JiebaDictSource `json:"dictSource"`
	DictVersion      string          `json:"dictVersion"`
}

// DefaultConfig 返回全文检索默认配置。
func DefaultConfig() Config {
	return Config{
		Enabled:        false,
		ActiveProvider: ProviderBleve,
		FallbackPolicy: FallbackPolicyDegradeToBleve,
		Analysis: AnalysisConfig{
			ActiveAnalyzer: AnalyzerSimple,
			Analyzers: AnalyzerConfigs{
				Simple: SimpleAnalyzerConfig{
					Enabled: true,
				},
				Jieba: JiebaAnalyzerConfig{
					Enabled:          false,
					Mode:             JiebaModeSearch,
					HMM:              true,
					StopwordsEnabled: false,
					DictSource:       JiebaDictSourceDB,
					DictVersion:      DefaultDictVersion,
				},
			},
		},
	}
}

// NormalizeConfig 将任意 payload 归一为可用检索配置。
func NormalizeConfig(payload map[string]any) Config {
	config := DefaultConfig()
	if payload == nil {
		return config
	}

	config.Enabled = readBool(payload, "enabled", config.Enabled)
	if provider := normalizeProviderName(readString(payload, "activeProvider")); provider != "" {
		config.ActiveProvider = provider
	}
	if policy := normalizeFallbackPolicy(readString(payload, "fallbackPolicy")); policy != "" {
		config.FallbackPolicy = policy
	}

	analysis, ok := readObject(payload, "analysis")
	if !ok {
		return config
	}
	if analyzerName := normalizeAnalyzerName(readString(analysis, "activeAnalyzer")); analyzerName != "" {
		config.Analysis.ActiveAnalyzer = analyzerName
	}

	analyzers, ok := readObject(analysis, "analyzers")
	if !ok {
		return config
	}

	if simple, ok := readObject(analyzers, "simple"); ok {
		config.Analysis.Analyzers.Simple.Enabled = readBool(
			simple,
			"enabled",
			config.Analysis.Analyzers.Simple.Enabled,
		)
	}

	if jieba, ok := readObject(analyzers, "jieba"); ok {
		config.Analysis.Analyzers.Jieba.Enabled = readBool(
			jieba,
			"enabled",
			config.Analysis.Analyzers.Jieba.Enabled,
		)
		if mode := normalizeJiebaMode(readString(jieba, "mode")); mode != "" {
			config.Analysis.Analyzers.Jieba.Mode = mode
		}
		config.Analysis.Analyzers.Jieba.HMM = readBool(
			jieba,
			"hmm",
			config.Analysis.Analyzers.Jieba.HMM,
		)
		config.Analysis.Analyzers.Jieba.StopwordsEnabled = readBool(
			jieba,
			"stopwordsEnabled",
			config.Analysis.Analyzers.Jieba.StopwordsEnabled,
		)
		if dictSource := normalizeJiebaDictSource(readString(jieba, "dictSource")); dictSource != "" {
			config.Analysis.Analyzers.Jieba.DictSource = dictSource
		}
		dictVersion := strings.TrimSpace(readString(jieba, "dictVersion"))
		if dictVersion != "" {
			config.Analysis.Analyzers.Jieba.DictVersion = dictVersion
		}
	}

	return config
}

// ValidateConfigPayload 校验 search 系统配置合法性。
func ValidateConfigPayload(payload map[string]any) error {
	if payload == nil {
		return fmt.Errorf("search config is required")
	}
	if err := validateNoUnknownKeys(payload, map[string]struct{}{
		"enabled":        {},
		"activeProvider": {},
		"fallbackPolicy": {},
		"analysis":       {},
	}); err != nil {
		return err
	}

	if rawEnabled, exists := payload["enabled"]; exists {
		if _, ok := rawEnabled.(bool); !ok {
			return fmt.Errorf("enabled must be boolean")
		}
	}

	activeProvider, err := getRequiredString(payload, "activeProvider")
	if err != nil {
		return err
	}
	if normalizeProviderName(activeProvider) == "" {
		return fmt.Errorf("activeProvider must be bleve/meili/typesense/database")
	}

	fallbackPolicy, err := getRequiredString(payload, "fallbackPolicy")
	if err != nil {
		return err
	}
	if normalizeFallbackPolicy(fallbackPolicy) == "" {
		return fmt.Errorf("fallbackPolicy must be degrade_to_bleve/return_error")
	}

	analysis, err := getRequiredObject(payload, "analysis")
	if err != nil {
		return err
	}
	if err := validateNoUnknownKeys(analysis, map[string]struct{}{
		"activeAnalyzer": {},
		"analyzers":      {},
	}); err != nil {
		return fmt.Errorf("analysis %w", err)
	}

	activeAnalyzer, err := getRequiredString(analysis, "activeAnalyzer")
	if err != nil {
		return fmt.Errorf("analysis %w", err)
	}
	normalizedActiveAnalyzer := normalizeAnalyzerName(activeAnalyzer)
	if normalizedActiveAnalyzer == "" {
		return fmt.Errorf("analysis.activeAnalyzer must be simple/jieba")
	}

	analyzers, err := getRequiredObject(analysis, "analyzers")
	if err != nil {
		return fmt.Errorf("analysis %w", err)
	}
	if err := validateNoUnknownKeys(analyzers, map[string]struct{}{
		"simple": {},
		"jieba":  {},
	}); err != nil {
		return fmt.Errorf("analysis.analyzers %w", err)
	}

	simpleConfig, err := getRequiredObject(analyzers, "simple")
	if err != nil {
		return fmt.Errorf("analysis.analyzers %w", err)
	}
	if err := validateNoUnknownKeys(simpleConfig, map[string]struct{}{
		"enabled": {},
	}); err != nil {
		return fmt.Errorf("analysis.analyzers.simple %w", err)
	}
	simpleEnabled, err := getRequiredBool(simpleConfig, "enabled")
	if err != nil {
		return fmt.Errorf("analysis.analyzers.simple %w", err)
	}

	jiebaConfig, err := getRequiredObject(analyzers, "jieba")
	if err != nil {
		return fmt.Errorf("analysis.analyzers %w", err)
	}
	if err := validateNoUnknownKeys(jiebaConfig, map[string]struct{}{
		"enabled":          {},
		"mode":             {},
		"hmm":              {},
		"stopwordsEnabled": {},
		"dictSource":       {},
		"dictVersion":      {},
	}); err != nil {
		return fmt.Errorf("analysis.analyzers.jieba %w", err)
	}
	jiebaEnabled, err := getRequiredBool(jiebaConfig, "enabled")
	if err != nil {
		return fmt.Errorf("analysis.analyzers.jieba %w", err)
	}
	mode, err := getRequiredString(jiebaConfig, "mode")
	if err != nil {
		return fmt.Errorf("analysis.analyzers.jieba %w", err)
	}
	if normalizeJiebaMode(mode) == "" {
		return fmt.Errorf("analysis.analyzers.jieba.mode must be search")
	}
	if _, err := getRequiredBool(jiebaConfig, "hmm"); err != nil {
		return fmt.Errorf("analysis.analyzers.jieba %w", err)
	}
	if _, err := getRequiredBool(jiebaConfig, "stopwordsEnabled"); err != nil {
		return fmt.Errorf("analysis.analyzers.jieba %w", err)
	}
	dictSource, err := getRequiredString(jiebaConfig, "dictSource")
	if err != nil {
		return fmt.Errorf("analysis.analyzers.jieba %w", err)
	}
	if normalizeJiebaDictSource(dictSource) == "" {
		return fmt.Errorf("analysis.analyzers.jieba.dictSource must be db/file")
	}
	dictVersion, err := getRequiredString(jiebaConfig, "dictVersion")
	if err != nil {
		return fmt.Errorf("analysis.analyzers.jieba %w", err)
	}
	if !isValidDictVersion(dictVersion) {
		return fmt.Errorf("analysis.analyzers.jieba.dictVersion is invalid")
	}

	switch normalizedActiveAnalyzer {
	case AnalyzerSimple:
		if !simpleEnabled {
			return fmt.Errorf("analysis.activeAnalyzer simple must be enabled")
		}
	case AnalyzerJieba:
		if !jiebaEnabled {
			return fmt.Errorf("analysis.activeAnalyzer jieba must be enabled")
		}
	}

	return nil
}

// IsAnalyzerEnabled 判断指定 analyzer 是否启用。
func (config Config) IsAnalyzerEnabled(name AnalyzerName) bool {
	switch normalizeAnalyzerName(string(name)) {
	case AnalyzerSimple:
		return config.Analysis.Analyzers.Simple.Enabled
	case AnalyzerJieba:
		return config.Analysis.Analyzers.Jieba.Enabled
	default:
		return false
	}
}

func isValidDictVersion(value string) bool {
	return dictVersionRegexp.MatchString(strings.TrimSpace(value))
}

func normalizeProviderName(value string) ProviderName {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(ProviderBleve):
		return ProviderBleve
	case string(ProviderMeili):
		return ProviderMeili
	case string(ProviderTypesense):
		return ProviderTypesense
	case string(ProviderDatabase):
		return ProviderDatabase
	default:
		return ""
	}
}

func normalizeFallbackPolicy(value string) FallbackPolicy {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(FallbackPolicyDegradeToBleve):
		return FallbackPolicyDegradeToBleve
	case string(FallbackPolicyReturnError):
		return FallbackPolicyReturnError
	default:
		return ""
	}
}

func normalizeAnalyzerName(value string) AnalyzerName {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(AnalyzerSimple):
		return AnalyzerSimple
	case string(AnalyzerJieba):
		return AnalyzerJieba
	default:
		return ""
	}
}

func normalizeJiebaMode(value string) JiebaMode {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(JiebaModeSearch):
		return JiebaModeSearch
	default:
		return ""
	}
}

func normalizeJiebaDictSource(value string) JiebaDictSource {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(JiebaDictSourceDB):
		return JiebaDictSourceDB
	case string(JiebaDictSourceFile):
		return JiebaDictSourceFile
	default:
		return ""
	}
}

func validateNoUnknownKeys(payload map[string]any, allowed map[string]struct{}) error {
	unexpectedKeys := make([]string, 0, 2)
	for key := range payload {
		if _, ok := allowed[key]; ok {
			continue
		}
		unexpectedKeys = append(unexpectedKeys, key)
	}
	if len(unexpectedKeys) == 0 {
		return nil
	}
	slices.Sort(unexpectedKeys)
	return fmt.Errorf("unexpected config keys: %s", strings.Join(unexpectedKeys, ", "))
}

func getRequiredString(payload map[string]any, key string) (string, error) {
	raw, ok := payload[key]
	if !ok {
		return "", fmt.Errorf("%s is required", key)
	}
	value, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("%s must be string", key)
	}
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "", fmt.Errorf("%s must not be empty", key)
	}
	return normalized, nil
}

func getRequiredBool(payload map[string]any, key string) (bool, error) {
	raw, ok := payload[key]
	if !ok {
		return false, fmt.Errorf("%s is required", key)
	}
	value, ok := raw.(bool)
	if !ok {
		return false, fmt.Errorf("%s must be boolean", key)
	}
	return value, nil
}

func getRequiredObject(payload map[string]any, key string) (map[string]any, error) {
	raw, ok := payload[key]
	if !ok {
		return nil, fmt.Errorf("%s is required", key)
	}
	value, ok := raw.(map[string]any)
	if !ok || value == nil {
		return nil, fmt.Errorf("%s must be object", key)
	}
	return value, nil
}

func readString(payload map[string]any, key string) string {
	raw, ok := payload[key]
	if !ok {
		return ""
	}
	value, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func readObject(payload map[string]any, key string) (map[string]any, bool) {
	raw, ok := payload[key]
	if !ok {
		return nil, false
	}
	value, ok := raw.(map[string]any)
	if !ok || value == nil {
		return nil, false
	}
	return value, true
}

func readBool(payload map[string]any, key string, fallback bool) bool {
	raw, ok := payload[key]
	if !ok {
		return fallback
	}
	value, ok := raw.(bool)
	if !ok {
		return fallback
	}
	return value
}
