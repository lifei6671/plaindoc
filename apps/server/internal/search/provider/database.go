package provider

import (
	"context"
	"errors"
	"strings"

	searchanalyzer "github.com/lifei6671/plaindoc/apps/server/internal/search/analyzer"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

const (
	defaultDatabaseSearchPage     = 1
	defaultDatabaseSearchPageSize = 20
	maxDatabaseSearchPageSize     = 200
)

// DatabaseProvider 基于数据库 LIKE 的简易检索 Provider。
//
// 说明：
// 1) 仅适用于轻量场景，不提供倒排索引能力；
// 2) 作为可配置检索引擎中的一种，便于在外部引擎未接入时快速落地。
type DatabaseProvider struct {
	visibilityRepo repository.SearchVisibilityRepository
}

// NewDatabaseProvider 创建 DatabaseProvider。
func NewDatabaseProvider(db *gorm.DB) *DatabaseProvider {
	return &DatabaseProvider{
		visibilityRepo: repository.NewGormSearchVisibilityRepository(db),
	}
}

func (p *DatabaseProvider) Name() string {
	return "database"
}

func (p *DatabaseProvider) Health(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if p == nil || p.visibilityRepo == nil {
		return errors.New("database search provider visibility repository is nil")
	}
	return nil
}

func (p *DatabaseProvider) Verify(ctx context.Context, config map[string]any) error {
	return p.Health(ctx)
}

func (p *DatabaseProvider) EnsureSchema(ctx context.Context) error {
	return p.Health(ctx)
}

func (p *DatabaseProvider) Upsert(ctx context.Context, records []IndexRecord) error {
	return p.Health(ctx)
}

func (p *DatabaseProvider) Delete(ctx context.Context, docIDs []string) error {
	return p.Health(ctx)
}

func (p *DatabaseProvider) PurgeBySpace(ctx context.Context, spaceID string) error {
	return p.Health(ctx)
}

func (p *DatabaseProvider) Search(ctx context.Context, request SearchRequest) (SearchResponse, error) {
	if err := p.Health(ctx); err != nil {
		return SearchResponse{}, err
	}

	terms := extractSearchTerms(request.Query)
	if len(terms) == 0 {
		return SearchResponse{Total: 0, Hits: []SearchHit{}}, nil
	}

	_, pageSize, offset := normalizeDatabaseSearchPagination(request.Page, request.PageSize)
	rows, total, err := p.visibilityRepo.SearchVisibleDocuments(ctx, repository.SearchVisibleDocumentsParams{
		ActorUserID: strings.TrimSpace(request.ActorUserID),
		SpaceID:     strings.TrimSpace(request.SpaceID),
		Terms:       terms,
		Limit:       pageSize,
		Offset:      offset,
	})
	if err != nil {
		return SearchResponse{}, err
	}
	if total <= 0 {
		return SearchResponse{Total: 0, Hits: []SearchHit{}}, nil
	}

	hits := make([]SearchHit, 0, len(rows))
	baseScore := float64(len(terms))
	for index, row := range rows {
		hits = append(hits, SearchHit{
			DocID:   strings.TrimSpace(row.DocumentID),
			Score:   baseScore - (float64(index) * 0.001),
			Snippet: buildDatabaseSearchSnippet(row.ContentMD),
		})
	}

	return SearchResponse{
		Total: total,
		Hits:  hits,
	}, nil
}

func (p *DatabaseProvider) Capabilities() Capabilities {
	return Capabilities{
		SupportsHighlight:      false,
		SupportsSortUpdatedAt:  true,
		SupportsTypo:           false,
		SupportsFacets:         false,
		SupportsCustomAnalyzer: false,
	}
}

func normalizeDatabaseSearchPagination(page int, pageSize int) (int, int, int) {
	normalizedPage := page
	if normalizedPage <= 0 {
		normalizedPage = defaultDatabaseSearchPage
	}

	normalizedPageSize := pageSize
	if normalizedPageSize <= 0 {
		normalizedPageSize = defaultDatabaseSearchPageSize
	}
	if normalizedPageSize > maxDatabaseSearchPageSize {
		normalizedPageSize = maxDatabaseSearchPageSize
	}

	offset := (normalizedPage - 1) * normalizedPageSize
	if offset < 0 {
		offset = 0
	}

	return normalizedPage, normalizedPageSize, offset
}

func extractSearchTerms(query string) []string {
	trimmedQuery := strings.TrimSpace(query)
	if trimmedQuery == "" {
		return []string{}
	}

	fields := strings.Fields(strings.ToLower(trimmedQuery))
	if len(fields) == 0 {
		return []string{}
	}

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

func buildDatabaseSearchSnippet(contentMD string) string {
	plain := strings.TrimSpace(searchanalyzer.NormalizeMarkdownToPlainText(contentMD))
	if plain == "" {
		return ""
	}
	limit := 200
	runes := []rune(plain)
	if len(runes) <= limit {
		return plain
	}
	return strings.TrimSpace(string(runes[:limit])) + "..."
}
