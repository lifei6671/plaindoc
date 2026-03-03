package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	searchcfg "github.com/lifei6671/plaindoc/apps/server/internal/search"
	searchanalyzer "github.com/lifei6671/plaindoc/apps/server/internal/search/analyzer"
	searchprovider "github.com/lifei6671/plaindoc/apps/server/internal/search/provider"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
)

const searchNormalizerVersion = "markdown_normalizer_v1"

var (
	// ErrSearchDisabled 表示全文检索全局开关未启用。
	ErrSearchDisabled = errors.New("search is disabled")
	// ErrSearchProviderUnavailable 表示当前配置的检索引擎不可用。
	ErrSearchProviderUnavailable = errors.New("search provider unavailable")
)

// SearchQueryInput 定义统一检索查询参数。
type SearchQueryInput struct {
	SpaceID        string
	ViewerUserID   string
	Query          string
	Page           int
	PageSize       int
	Sort           searchprovider.SortMode
	NeedHighlight  bool
	ForceConfigRef bool
}

// SearchQueryResult 定义统一检索查询结果。
type SearchQueryResult struct {
	Provider searchcfg.ProviderName
	Response searchprovider.SearchResponse
}

// SearchQueryService 负责“配置解析 + 分词 + provider 路由 + 检索执行”。
type SearchQueryService struct {
	searchConfigService  *SearchConfigService
	searchVisibilityRepo repository.SearchVisibilityRepository
	providers            map[searchcfg.ProviderName]searchprovider.Provider
}

// NewSearchQueryService 创建统一检索查询服务。
func NewSearchQueryService(
	searchConfigService *SearchConfigService,
	providers ...searchprovider.Provider,
) *SearchQueryService {
	providerMap := make(map[searchcfg.ProviderName]searchprovider.Provider, len(providers))
	for _, item := range providers {
		if item == nil {
			continue
		}
		name := searchcfg.ProviderName(strings.ToLower(strings.TrimSpace(item.Name())))
		if name == "" {
			continue
		}
		providerMap[name] = item
	}
	return &SearchQueryService{
		searchConfigService: searchConfigService,
		providers:           providerMap,
	}
}

// SetSearchVisibilityRepository 注入检索可见性仓储，用于解析用户在空间内的角色级别。
func (s *SearchQueryService) SetSearchVisibilityRepository(
	searchVisibilityRepo repository.SearchVisibilityRepository,
) {
	if s == nil {
		return
	}
	s.searchVisibilityRepo = searchVisibilityRepo
}

// Search 执行统一检索请求。
func (s *SearchQueryService) Search(
	ctx context.Context,
	input SearchQueryInput,
) (SearchQueryResult, error) {
	if s == nil || s.searchConfigService == nil {
		return SearchQueryResult{}, errors.New("search query service dependencies are nil")
	}
	if ctx == nil {
		return SearchQueryResult{}, errors.New("search context is nil")
	}

	snapshot, err := s.searchConfigService.Refresh(ctx)
	if err != nil {
		return SearchQueryResult{}, err
	}
	if !snapshot.Config.Enabled {
		return SearchQueryResult{}, ErrSearchDisabled
	}
	if snapshot.ActiveAnalyzer == nil {
		return SearchQueryResult{}, fmt.Errorf("%w: active analyzer is nil", ErrSearchProviderUnavailable)
	}

	providerName, providerInstance, err := s.resolveProvider(snapshot.Config)
	if err != nil {
		return SearchQueryResult{}, err
	}

	queryText := strings.TrimSpace(input.Query)
	if queryText == "" {
		return SearchQueryResult{
			Provider: providerName,
			Response: searchprovider.SearchResponse{Total: 0, Hits: []searchprovider.SearchHit{}},
		}, nil
	}

	analyzeOutput, err := snapshot.ActiveAnalyzer.AnalyzeForQuery(ctx, searchanalyzer.AnalyzeInput{
		Text:    queryText,
		Mode:    searchanalyzer.ModeQuery,
		SpaceID: strings.TrimSpace(input.SpaceID),
	})
	if err != nil {
		return SearchQueryResult{}, err
	}

	normalizedQuery := strings.Join(analyzeOutput.Tokens, " ")
	normalizedQuery = strings.TrimSpace(normalizedQuery)
	if normalizedQuery == "" {
		normalizedQuery = strings.TrimSpace(analyzeOutput.NormalizedText)
	}
	if normalizedQuery == "" {
		return SearchQueryResult{
			Provider: providerName,
			Response: searchprovider.SearchResponse{Total: 0, Hits: []searchprovider.SearchHit{}},
		}, nil
	}

	normalizedSpaceID := strings.TrimSpace(input.SpaceID)
	normalizedViewerUserID := strings.TrimSpace(input.ViewerUserID)
	userRoleLevel, err := s.resolveUserRoleLevel(ctx, normalizedSpaceID, normalizedViewerUserID)
	if err != nil {
		return SearchQueryResult{}, err
	}

	request := searchprovider.SearchRequest{
		SpaceID:           normalizedSpaceID,
		ActorUserID:       normalizedViewerUserID,
		IsAuthenticated:   normalizedViewerUserID != "",
		UserRoleLevel:     userRoleLevel,
		Query:             normalizedQuery,
		Page:              input.Page,
		PageSize:          input.PageSize,
		Sort:              input.Sort,
		NeedHighlight:     input.NeedHighlight,
		DictVersion:       strings.TrimSpace(analyzeOutput.DictVersion),
		NormalizerVersion: searchNormalizerVersion,
	}

	response, err := providerInstance.Search(ctx, request)
	if err != nil {
		return SearchQueryResult{}, err
	}
	return SearchQueryResult{
		Provider: providerName,
		Response: response,
	}, nil
}

func (s *SearchQueryService) resolveUserRoleLevel(
	ctx context.Context,
	spaceID string,
	viewerUserID string,
) (int, error) {
	if s == nil || s.searchVisibilityRepo == nil {
		return 0, nil
	}
	normalizedSpaceID := strings.TrimSpace(spaceID)
	normalizedViewerUserID := strings.TrimSpace(viewerUserID)
	if normalizedSpaceID == "" || normalizedViewerUserID == "" {
		return 0, nil
	}

	roleLevel, err := s.searchVisibilityRepo.ResolveUserRoleLevel(ctx, normalizedSpaceID, normalizedViewerUserID)
	if err != nil {
		return 0, err
	}
	if roleLevel < 0 {
		return 0, nil
	}
	if roleLevel > 3 {
		return 3, nil
	}
	return roleLevel, nil
}

func (s *SearchQueryService) resolveProvider(
	config searchcfg.Config,
) (searchcfg.ProviderName, searchprovider.Provider, error) {
	if s == nil {
		return "", nil, fmt.Errorf("%w: search query service is nil", ErrSearchProviderUnavailable)
	}
	activeProvider := config.ActiveProvider
	if providerInstance := s.providers[activeProvider]; providerInstance != nil {
		return activeProvider, providerInstance, nil
	}

	if config.FallbackPolicy == searchcfg.FallbackPolicyDegradeToDatabase &&
		activeProvider != searchcfg.ProviderDatabase {
		if fallbackProvider := s.providers[searchcfg.ProviderDatabase]; fallbackProvider != nil {
			return searchcfg.ProviderDatabase, fallbackProvider, nil
		}
	}

	return "", nil, fmt.Errorf(
		"%w: provider %q is not configured",
		ErrSearchProviderUnavailable,
		activeProvider,
	)
}
