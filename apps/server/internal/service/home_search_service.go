package service

import (
	"context"
	"errors"
	"strings"
	"time"

	searchprovider "github.com/lifei6671/plaindoc/apps/server/internal/search/provider"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

// HomeSearchInput 首页全文检索查询参数。
type HomeSearchInput struct {
	ViewerUserID string
	Keyword      string
	Page         int
	PageSize     int
}

// HomeSearchHitRecord 首页全文检索命中项。
type HomeSearchHitRecord struct {
	SpaceID    string
	SpaceName  string
	DocumentID string
	Title      string
	Snippet    string
	UpdatedAt  time.Time
}

// HomeSearchPageRecord 首页全文检索分页结果。
type HomeSearchPageRecord struct {
	Keyword  string
	Items    []HomeSearchHitRecord
	Page     int
	PageSize int
	Total    int64
}

type homeSearchDocumentMetadata struct {
	SpaceID    string
	SpaceName  string
	DocumentID string
	Title      string
	UpdatedAt  time.Time
}

// HomeSearchService 封装首页全文检索读取能力。
type HomeSearchService struct {
	searchQueryService *SearchQueryService
	homeSearchRepo     repository.HomeSearchRepository
}

// NewHomeSearchService 创建首页全文检索服务。
func NewHomeSearchService(searchQueryService *SearchQueryService, db *gorm.DB) *HomeSearchService {
	return &HomeSearchService{
		searchQueryService: searchQueryService,
		homeSearchRepo:     repository.NewGormHomeSearchRepository(db),
	}
}

// Search 执行首页全文检索（通过统一检索模块）。
func (s *HomeSearchService) Search(
	ctx context.Context,
	input HomeSearchInput,
) (HomeSearchPageRecord, error) {
	if s == nil || s.homeSearchRepo == nil || s.searchQueryService == nil {
		return HomeSearchPageRecord{}, errors.New("home search service dependencies are nil")
	}

	page, pageSize, _ := normalizeHomepagePagination(input.Page, input.PageSize)
	keyword := strings.TrimSpace(input.Keyword)
	result := HomeSearchPageRecord{
		Keyword:  keyword,
		Items:    []HomeSearchHitRecord{},
		Page:     page,
		PageSize: pageSize,
		Total:    0,
	}
	if keyword == "" {
		return result, nil
	}

	queryResult, err := s.searchQueryService.Search(ctx, SearchQueryInput{
		ViewerUserID:  strings.TrimSpace(input.ViewerUserID),
		Query:         keyword,
		Page:          page,
		PageSize:      pageSize,
		Sort:          searchprovider.SortModeRelevance,
		NeedHighlight: false,
	})
	if err != nil {
		if errors.Is(err, ErrSearchDisabled) || errors.Is(err, ErrSearchProviderUnavailable) {
			return result, nil
		}
		return HomeSearchPageRecord{}, err
	}

	result.Total = queryResult.Response.Total
	if len(queryResult.Response.Hits) == 0 {
		return result, nil
	}

	documentIDs := make([]string, 0, len(queryResult.Response.Hits))
	for _, item := range queryResult.Response.Hits {
		documentID := strings.TrimSpace(item.DocID)
		if documentID == "" {
			continue
		}
		documentIDs = append(documentIDs, documentID)
	}
	if len(documentIDs) == 0 {
		return result, nil
	}

	metadataByDocumentID, err := s.loadDocumentMetadata(ctx, documentIDs)
	if err != nil {
		return HomeSearchPageRecord{}, err
	}

	items := make([]HomeSearchHitRecord, 0, len(queryResult.Response.Hits))
	for _, item := range queryResult.Response.Hits {
		documentID := strings.TrimSpace(item.DocID)
		if documentID == "" {
			continue
		}
		metadata, exists := metadataByDocumentID[documentID]
		if !exists {
			continue
		}
		items = append(items, HomeSearchHitRecord{
			SpaceID:    metadata.SpaceID,
			SpaceName:  metadata.SpaceName,
			DocumentID: metadata.DocumentID,
			Title:      metadata.Title,
			Snippet:    strings.TrimSpace(item.Snippet),
			UpdatedAt:  metadata.UpdatedAt,
		})
	}

	result.Items = items
	return result, nil
}

func (s *HomeSearchService) loadDocumentMetadata(
	ctx context.Context,
	documentIDs []string,
) (map[string]homeSearchDocumentMetadata, error) {
	if s == nil || s.homeSearchRepo == nil {
		return nil, errors.New("home search service repository is nil")
	}
	if len(documentIDs) == 0 {
		return map[string]homeSearchDocumentMetadata{}, nil
	}

	rows, err := s.homeSearchRepo.ListActiveDocumentMetadataByDocumentIDs(ctx, documentIDs)
	if err != nil {
		return nil, err
	}

	result := make(map[string]homeSearchDocumentMetadata, len(rows))
	for _, row := range rows {
		documentID := strings.TrimSpace(row.DocumentID)
		if documentID == "" {
			continue
		}
		result[documentID] = homeSearchDocumentMetadata{
			SpaceID:    strings.TrimSpace(row.SpaceID),
			SpaceName:  strings.TrimSpace(row.SpaceName),
			DocumentID: documentID,
			Title:      strings.TrimSpace(row.Title),
			UpdatedAt:  row.UpdatedAt,
		}
	}
	return result, nil
}
