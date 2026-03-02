package analyzer

import "context"

// Mode 定义分词执行模式。
type Mode string

const (
	ModeIndex Mode = "index"
	ModeQuery Mode = "query"
)

// AnalyzeInput 定义分词输入参数。
type AnalyzeInput struct {
	Text     string
	Mode     Mode
	Language string
	SpaceID  string
}

// AnalyzeOutput 定义分词输出结果。
type AnalyzeOutput struct {
	Tokens         []string
	NormalizedText string
	TokenCount     int
	DictVersion    string
}

// Capabilities 描述分词器能力位。
type Capabilities struct {
	SupportsUserDict   bool
	SupportsHotReload  bool
	SupportsPhraseHint bool
	SupportsStopwords  bool
	SupportsSynonyms   bool
}

// Provider 定义分词器统一契约。
type Provider interface {
	Name() string
	Health(ctx context.Context) error
	AnalyzeForIndex(ctx context.Context, input AnalyzeInput) (AnalyzeOutput, error)
	AnalyzeForQuery(ctx context.Context, input AnalyzeInput) (AnalyzeOutput, error)
	Reload(ctx context.Context, dictVersion string) error
	Capabilities() Capabilities
}
