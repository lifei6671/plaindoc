package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	searchcfg "github.com/lifei6671/plaindoc/apps/server/internal/search"
	searchprovider "github.com/lifei6671/plaindoc/apps/server/internal/search/provider"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

func TestSearchQueryService_SearchInjectsUserRoleLevel(t *testing.T) {
	configJSON, err := buildEnabledDatabaseSearchConfigJSON()
	if err != nil {
		t.Fatalf("build search config json failed: %v", err)
	}

	systemConfigRepo := &staticSystemConfigRepository{
		record: &models.SystemConfig{
			ConfigKey:       searchcfg.SystemConfigKey,
			ConfigValueJSON: configJSON,
			Version:         1,
			CreatedAt:       time.Now().UTC(),
			UpdatedAt:       time.Now().UTC(),
		},
	}
	visibilityRepo := &roleAwareSearchVisibilityRepository{
		roleLevelBySpace: map[string]map[string]int{
			"space-1": {
				"reader-1":   1,
				"collab-1":   2,
				"owner-1":    3,
				"outsider-1": 0,
			},
		},
		scopeSpaceIDsByUser: map[string][]string{
			"":           {"space-public"},
			"reader-1":   {"space-1"},
			"collab-1":   {"space-1"},
			"owner-1":    {"space-1"},
			"outsider-1": {"space-public"},
		},
	}
	provider := &recordingSearchProvider{name: string(searchcfg.ProviderDatabase)}

	searchConfigService := NewSearchConfigService(systemConfigRepo, SearchConfigServiceOptions{})
	searchQueryService := NewSearchQueryService(searchConfigService, provider)
	searchQueryService.SetSearchVisibilityRepository(visibilityRepo)

	testCases := []struct {
		name              string
		spaceID           string
		viewerUserID      string
		expectedRoleLevel int
		expectedScopes    []string
	}{
		{
			name:              "anonymous",
			spaceID:           "space-1",
			viewerUserID:      "",
			expectedRoleLevel: 0,
			expectedScopes:    []string{"space-1"},
		},
		{
			name:              "reader",
			spaceID:           "space-1",
			viewerUserID:      "reader-1",
			expectedRoleLevel: 1,
			expectedScopes:    []string{"space-1"},
		},
		{
			name:              "collaborator",
			spaceID:           "space-1",
			viewerUserID:      "collab-1",
			expectedRoleLevel: 2,
			expectedScopes:    []string{"space-1"},
		},
		{
			name:              "owner",
			spaceID:           "space-1",
			viewerUserID:      "owner-1",
			expectedRoleLevel: 3,
			expectedScopes:    []string{"space-1"},
		},
		{
			name:              "non-member",
			spaceID:           "space-1",
			viewerUserID:      "outsider-1",
			expectedRoleLevel: 0,
			expectedScopes:    []string{"space-1"},
		},
		{
			name:              "empty-space-id",
			spaceID:           "",
			viewerUserID:      "owner-1",
			expectedRoleLevel: 0,
			expectedScopes:    []string{"space-1"},
		},
	}

	for _, item := range testCases {
		item := item
		t.Run(item.name, func(t *testing.T) {
			_, searchErr := searchQueryService.Search(context.Background(), SearchQueryInput{
				SpaceID:       item.spaceID,
				ViewerUserID:  item.viewerUserID,
				Query:         "hello world",
				Page:          1,
				PageSize:      20,
				Sort:          searchprovider.SortModeRelevance,
				NeedHighlight: false,
			})
			if searchErr != nil {
				t.Fatalf("search failed: %v", searchErr)
			}

			request := provider.LastRequest()
			if request.UserRoleLevel != item.expectedRoleLevel {
				t.Fatalf("expected user role level=%d, got=%d", item.expectedRoleLevel, request.UserRoleLevel)
			}
			if request.SpaceID != item.spaceID {
				t.Fatalf("expected space id=%q, got=%q", item.spaceID, request.SpaceID)
			}
			if len(request.ScopeSpaceIDs) != len(item.expectedScopes) {
				t.Fatalf("expected scope size=%d, got=%d", len(item.expectedScopes), len(request.ScopeSpaceIDs))
			}
			scopeSet := make(map[string]struct{}, len(request.ScopeSpaceIDs))
			for _, scopeSpaceID := range request.ScopeSpaceIDs {
				scopeSet[scopeSpaceID] = struct{}{}
			}
			for _, expectedScope := range item.expectedScopes {
				if _, exists := scopeSet[expectedScope]; !exists {
					t.Fatalf("expected scope %q in request scopes=%v", expectedScope, request.ScopeSpaceIDs)
				}
			}
		})
	}
}

func TestSearchQueryService_SearchReturnsEmptyWhenScopeUnavailable(t *testing.T) {
	configJSON, err := buildEnabledDatabaseSearchConfigJSON()
	if err != nil {
		t.Fatalf("build search config json failed: %v", err)
	}
	systemConfigRepo := &staticSystemConfigRepository{
		record: &models.SystemConfig{
			ConfigKey:       searchcfg.SystemConfigKey,
			ConfigValueJSON: configJSON,
			Version:         1,
			CreatedAt:       time.Now().UTC(),
			UpdatedAt:       time.Now().UTC(),
		},
	}
	visibilityRepo := &roleAwareSearchVisibilityRepository{
		roleLevelBySpace:    map[string]map[string]int{},
		scopeSpaceIDsByUser: map[string][]string{},
	}
	provider := &recordingSearchProvider{name: string(searchcfg.ProviderDatabase)}

	searchConfigService := NewSearchConfigService(systemConfigRepo, SearchConfigServiceOptions{})
	searchQueryService := NewSearchQueryService(searchConfigService, provider)
	searchQueryService.SetSearchVisibilityRepository(visibilityRepo)

	result, searchErr := searchQueryService.Search(context.Background(), SearchQueryInput{
		ViewerUserID:  "no-scope-user",
		Query:         "hello world",
		Page:          1,
		PageSize:      20,
		Sort:          searchprovider.SortModeRelevance,
		NeedHighlight: false,
	})
	if searchErr != nil {
		t.Fatalf("search failed: %v", searchErr)
	}
	if result.Response.Total != 0 {
		t.Fatalf("expected total=0, got=%d", result.Response.Total)
	}
	if provider.CallCount() != 0 {
		t.Fatalf("expected provider call count=0, got=%d", provider.CallCount())
	}
}

func buildEnabledDatabaseSearchConfigJSON() (string, error) {
	config := searchcfg.DefaultConfig()
	config.Enabled = true
	config.ActiveProvider = searchcfg.ProviderDatabase
	payload, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

type staticSystemConfigRepository struct {
	record *models.SystemConfig
}

func (r *staticSystemConfigRepository) List(ctx context.Context) ([]models.SystemConfig, error) {
	if r == nil || r.record == nil {
		return []models.SystemConfig{}, nil
	}
	return []models.SystemConfig{*r.record}, nil
}

func (r *staticSystemConfigRepository) GetByConfigKey(ctx context.Context, configKey string) (*models.SystemConfig, error) {
	if r == nil || r.record == nil {
		return nil, gorm.ErrRecordNotFound
	}
	if r.record.ConfigKey != configKey {
		return nil, gorm.ErrRecordNotFound
	}
	return r.record, nil
}

func (r *staticSystemConfigRepository) Create(ctx context.Context, config *models.SystemConfig) error {
	return nil
}

func (r *staticSystemConfigRepository) UpdateByVersion(
	ctx context.Context,
	params repository.UpdateSystemConfigByVersionParams,
) (bool, error) {
	return true, nil
}

type roleAwareSearchVisibilityRepository struct {
	roleLevelBySpace    map[string]map[string]int
	scopeSpaceIDsByUser map[string][]string
}

func (r *roleAwareSearchVisibilityRepository) SearchVisibleDocuments(
	ctx context.Context,
	params repository.SearchVisibleDocumentsParams,
) ([]repository.SearchVisibleDocumentRow, int64, error) {
	return []repository.SearchVisibleDocumentRow{}, 0, nil
}

func (r *roleAwareSearchVisibilityRepository) FilterVisibleDocumentIDsByCandidates(
	ctx context.Context,
	params repository.SearchVisibleDocumentIDsByCandidatesParams,
) ([]string, error) {
	return []string{}, nil
}

func (r *roleAwareSearchVisibilityRepository) ResolveUserRoleLevelsBySpaces(
	ctx context.Context,
	actorUserID string,
	spaceIDs []string,
) (map[string]int, error) {
	result := make(map[string]int, len(spaceIDs))
	if r == nil {
		return result, nil
	}
	for _, spaceID := range spaceIDs {
		if roleMap, exists := r.roleLevelBySpace[spaceID]; exists {
			result[spaceID] = roleMap[actorUserID]
		}
	}
	return result, nil
}

func (r *roleAwareSearchVisibilityRepository) ResolveSearchScopeSpaceIDs(
	ctx context.Context,
	actorUserID string,
) ([]string, error) {
	if r == nil {
		return []string{}, nil
	}
	scopes := r.scopeSpaceIDsByUser[actorUserID]
	result := make([]string, 0, len(scopes))
	for _, item := range scopes {
		result = append(result, item)
	}
	return result, nil
}

func (r *roleAwareSearchVisibilityRepository) ResolveUserRoleLevel(
	ctx context.Context,
	spaceID string,
	actorUserID string,
) (int, error) {
	if r == nil {
		return 0, nil
	}
	if roleMap, exists := r.roleLevelBySpace[spaceID]; exists {
		return roleMap[actorUserID], nil
	}
	return 0, nil
}

type recordingSearchProvider struct {
	name        string
	lastRequest searchprovider.SearchRequest
	callCount   int
}

func (p *recordingSearchProvider) Name() string {
	if p == nil || p.name == "" {
		return string(searchcfg.ProviderDatabase)
	}
	return p.name
}

func (p *recordingSearchProvider) Health(ctx context.Context) error {
	return nil
}

func (p *recordingSearchProvider) Verify(ctx context.Context, config map[string]any) error {
	return nil
}

func (p *recordingSearchProvider) EnsureSchema(ctx context.Context) error {
	return nil
}

func (p *recordingSearchProvider) Upsert(ctx context.Context, records []searchprovider.IndexRecord) error {
	return nil
}

func (p *recordingSearchProvider) Delete(ctx context.Context, docIDs []string) error {
	return nil
}

func (p *recordingSearchProvider) PurgeBySpace(ctx context.Context, spaceID string) error {
	return nil
}

func (p *recordingSearchProvider) Search(
	ctx context.Context,
	request searchprovider.SearchRequest,
) (searchprovider.SearchResponse, error) {
	if p != nil {
		p.lastRequest = request
		p.callCount++
	}
	return searchprovider.SearchResponse{Total: 0, Hits: []searchprovider.SearchHit{}}, nil
}

func (p *recordingSearchProvider) Capabilities() searchprovider.Capabilities {
	return searchprovider.Capabilities{}
}

func (p *recordingSearchProvider) LastRequest() searchprovider.SearchRequest {
	if p == nil {
		return searchprovider.SearchRequest{}
	}
	return p.lastRequest
}

func (p *recordingSearchProvider) CallCount() int {
	if p == nil {
		return 0
	}
	return p.callCount
}
