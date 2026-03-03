package provider

import (
	"context"
	"time"
)

// SortMode 定义检索排序方式。
type SortMode string

const (
	SortModeRelevance     SortMode = "relevance"
	SortModeUpdatedAtDesc SortMode = "updated_at_desc"
)

// IndexRecord 定义统一索引文档结构。
type IndexRecord struct {
	SpaceID         string
	DocID           string
	NodeID          string
	Title           string
	BodyPlain       string
	Terms           string
	TitleTerms      string
	VisibilityScope string
	MinRole         int
	UpdatedAtUnix   int64
	IsDeleted       bool
	SpaceStatus     string
	DocStatus       string
	AnalyzerName    string
	AnalyzerVersion string
}

// SearchRequest 定义统一检索请求结构。
type SearchRequest struct {
	SpaceID           string
	ScopeSpaceIDs     []string
	ActorUserID       string
	IsAuthenticated   bool
	UserRoleLevel     int
	Query             string
	Page              int
	PageSize          int
	Sort              SortMode
	NeedHighlight     bool
	DictVersion       string
	NormalizerVersion string
}

// SearchHit 定义统一检索命中结构。
type SearchHit struct {
	DocID     string
	Score     float64
	Snippet   string
	UpdatedAt time.Time
}

// SearchResponse 定义统一检索返回结构。
type SearchResponse struct {
	Total int64
	Hits  []SearchHit
}

// Capabilities 描述 provider 能力位。
type Capabilities struct {
	SupportsHighlight      bool
	SupportsSortUpdatedAt  bool
	SupportsTypo           bool
	SupportsFacets         bool
	SupportsCustomAnalyzer bool
}

// Provider 定义可插拔检索引擎契约。
type Provider interface {
	Name() string
	Health(ctx context.Context) error
	Verify(ctx context.Context, config map[string]any) error
	EnsureSchema(ctx context.Context) error
	Upsert(ctx context.Context, records []IndexRecord) error
	Delete(ctx context.Context, docIDs []string) error
	PurgeBySpace(ctx context.Context, spaceID string) error
	Search(ctx context.Context, request SearchRequest) (SearchResponse, error)
	Capabilities() Capabilities
}

// ResettableProvider 定义支持“全量重建前清空索引”的可选能力。
type ResettableProvider interface {
	Reset(ctx context.Context) error
}

// IndexStatsProvider 定义索引统计能力（用于判断是否需要重建）。
type IndexStatsProvider interface {
	DocCount(ctx context.Context) (uint64, error)
}
