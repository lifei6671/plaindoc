package provider

import (
	"context"
	"errors"
	"strings"

	searchanalyzer "github.com/lifei6671/plaindoc/apps/server/internal/search/analyzer"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

const (
	defaultDatabaseSearchPage     = 1
	defaultDatabaseSearchPageSize = 20
	maxDatabaseSearchPageSize     = 200
	maxDatabaseSearchCandidate    = 5000
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

	page, pageSize, _ := normalizeDatabaseSearchPagination(request.Page, request.PageSize)
	rows, total, err := p.visibilityRepo.SearchVisibleDocuments(ctx, repository.SearchVisibleDocumentsParams{
		ActorUserID:   strings.TrimSpace(request.ActorUserID),
		SpaceID:       strings.TrimSpace(request.SpaceID),
		ScopeSpaceIDs: request.ScopeSpaceIDs,
		Terms:         terms,
		Limit:         maxDatabaseSearchCandidate,
		Offset:        0,
	})
	if err != nil {
		return SearchResponse{}, err
	}
	if total <= 0 {
		return SearchResponse{Total: 0, Hits: []SearchHit{}}, nil
	}

	filteredRows, err := p.filterRowsByRole(ctx, request, rows)
	if err != nil {
		return SearchResponse{}, err
	}
	if len(filteredRows) == 0 {
		return SearchResponse{Total: 0, Hits: []SearchHit{}}, nil
	}

	start := (page - 1) * pageSize
	if start >= len(filteredRows) {
		return SearchResponse{Total: int64(len(filteredRows)), Hits: []SearchHit{}}, nil
	}
	end := start + pageSize
	if end > len(filteredRows) {
		end = len(filteredRows)
	}
	pageRows := filteredRows[start:end]

	hits := make([]SearchHit, 0, len(pageRows))
	baseScore := float64(len(terms))
	for index, row := range pageRows {
		hits = append(hits, SearchHit{
			DocID:   strings.TrimSpace(row.DocumentID),
			Score:   baseScore - (float64(index) * 0.001),
			Snippet: buildDatabaseSearchSnippet(row.ContentMD),
		})
	}

	return SearchResponse{
		Total: int64(len(filteredRows)),
		Hits:  hits,
	}, nil
}

func (p *DatabaseProvider) filterRowsByRole(
	ctx context.Context,
	request SearchRequest,
	rows []repository.SearchVisibleDocumentRow,
) ([]repository.SearchVisibleDocumentRow, error) {
	if len(rows) == 0 {
		return []repository.SearchVisibleDocumentRow{}, nil
	}

	hasActorIdentity := strings.TrimSpace(request.ActorUserID) != ""
	isSingleSpaceSearch := strings.TrimSpace(request.SpaceID) != ""
	scopeSpaceIDSet := make(map[string]struct{}, 0)
	if !isSingleSpaceSearch {
		scopeSpaceIDSet = make(map[string]struct{}, len(request.ScopeSpaceIDs))
		for _, item := range request.ScopeSpaceIDs {
			spaceID := strings.TrimSpace(item)
			if spaceID == "" {
				continue
			}
			scopeSpaceIDSet[spaceID] = struct{}{}
		}
	}
	filtered := make([]repository.SearchVisibleDocumentRow, 0, len(rows))

	if isSingleSpaceSearch {
		userRoleLevel := request.UserRoleLevel
		if userRoleLevel < 0 {
			userRoleLevel = 0
		}
		for _, item := range rows {
			if len(scopeSpaceIDSet) > 0 {
				spaceID := strings.TrimSpace(item.SpaceID)
				if spaceID == "" {
					continue
				}
				if _, exists := scopeSpaceIDSet[spaceID]; !exists {
					continue
				}
			}
			scope := strings.ToLower(strings.TrimSpace(item.VisibilityScope))
			switch scope {
			case string(models.VisibilityPublic):
				filtered = append(filtered, item)
			case string(models.VisibilityAuthenticated):
				if hasActorIdentity {
					filtered = append(filtered, item)
				}
			case string(models.VisibilityMember):
				minRole := item.MinRole
				if minRole <= 0 {
					minRole = 1
				}
				if hasActorIdentity && userRoleLevel >= minRole {
					filtered = append(filtered, item)
				}
			}
		}
		return filtered, nil
	}

	roleLevelBySpaceID := map[string]int{}
	if hasActorIdentity {
		spaceIDs := make([]string, 0, len(rows))
		for _, item := range rows {
			if strings.EqualFold(strings.TrimSpace(item.VisibilityScope), string(models.VisibilityMember)) {
				spaceIDs = append(spaceIDs, strings.TrimSpace(item.SpaceID))
			}
		}
		if len(spaceIDs) > 0 {
			resolvedRoleLevels, err := p.visibilityRepo.ResolveUserRoleLevelsBySpaces(
				ctx,
				strings.TrimSpace(request.ActorUserID),
				spaceIDs,
			)
			if err != nil {
				return nil, err
			}
			roleLevelBySpaceID = resolvedRoleLevels
		}
	}

	for _, item := range rows {
		if len(scopeSpaceIDSet) > 0 {
			spaceID := strings.TrimSpace(item.SpaceID)
			if spaceID == "" {
				continue
			}
			if _, exists := scopeSpaceIDSet[spaceID]; !exists {
				continue
			}
		}
		scope := strings.ToLower(strings.TrimSpace(item.VisibilityScope))
		switch scope {
		case string(models.VisibilityPublic):
			filtered = append(filtered, item)
		case string(models.VisibilityAuthenticated):
			if hasActorIdentity {
				filtered = append(filtered, item)
			}
		case string(models.VisibilityMember):
			minRole := item.MinRole
			if minRole <= 0 {
				minRole = 1
			}
			spaceID := strings.TrimSpace(item.SpaceID)
			if hasActorIdentity && roleLevelBySpaceID[spaceID] >= minRole {
				filtered = append(filtered, item)
			}
		}
	}
	return filtered, nil
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
