package service

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	searchprovider "github.com/lifei6671/plaindoc/apps/server/internal/search/provider"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
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

type searchScanTime struct {
	time.Time
}

func (s searchScanTime) Value() (driver.Value, error) {
	if s.Time.IsZero() {
		return nil, nil
	}
	return s.Time, nil
}

// HomeSearchService 封装首页全文检索读取能力。
type HomeSearchService struct {
	searchQueryService *SearchQueryService
	db                 *gorm.DB
}

// NewHomeSearchService 创建首页全文检索服务。
func NewHomeSearchService(searchQueryService *SearchQueryService, db *gorm.DB) *HomeSearchService {
	return &HomeSearchService{
		searchQueryService: searchQueryService,
		db:                 db,
	}
}

// Search 执行首页全文检索（通过统一检索模块）。
func (s *HomeSearchService) Search(
	ctx context.Context,
	input HomeSearchInput,
) (HomeSearchPageRecord, error) {
	if s == nil || s.db == nil || s.searchQueryService == nil {
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
	if s == nil || s.db == nil {
		return nil, errors.New("home search service db is nil")
	}
	if len(documentIDs) == 0 {
		return map[string]homeSearchDocumentMetadata{}, nil
	}

	type metadataRow struct {
		SpaceID    string         `gorm:"column:space_id"`
		SpaceName  string         `gorm:"column:space_name"`
		DocumentID string         `gorm:"column:document_id"`
		Title      string         `gorm:"column:title"`
		UpdatedAt  searchScanTime `gorm:"column:updated_at"`
	}

	rows := make([]metadataRow, 0, len(documentIDs))
	if err := s.db.WithContext(ctx).
		Table("documents AS d").
		Select(
			"s.space_id AS space_id",
			"s.name AS space_name",
			"d.document_id AS document_id",
			"d.title AS title",
			"d.updated_at AS updated_at",
		).
		Joins("JOIN nodes AS n ON n.node_id = d.node_id").
		Joins("JOIN spaces AS s ON s.space_id = n.space_id").
		Where("d.document_id IN ?", documentIDs).
		Where("d.status = ? AND d.deleted_at IS NULL", models.EntityStatusActive).
		Where("s.status = ? AND s.deleted_at IS NULL", models.EntityStatusActive).
		Find(&rows).Error; err != nil {
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
			UpdatedAt:  row.UpdatedAt.Time,
		}
	}
	return result, nil
}

func (s *searchScanTime) Scan(value any) error {
	if s == nil {
		return nil
	}
	switch current := value.(type) {
	case nil:
		s.Time = time.Time{}
		return nil
	case time.Time:
		s.Time = current
		return nil
	case *time.Time:
		if current == nil {
			s.Time = time.Time{}
			return nil
		}
		s.Time = *current
		return nil
	case []byte:
		s.Time = parseSearchUpdatedAtString(string(current))
		return nil
	case string:
		s.Time = parseSearchUpdatedAtString(current)
		return nil
	case int:
		s.Time = time.Unix(int64(current), 0).UTC()
		return nil
	case int64:
		s.Time = time.Unix(current, 0).UTC()
		return nil
	case float64:
		s.Time = time.Unix(int64(current), 0).UTC()
		return nil
	default:
		s.Time = parseSearchUpdatedAtString(fmt.Sprint(current))
		return nil
	}
}

func parseSearchUpdatedAtString(value string) time.Time {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return time.Time{}
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
		time.DateTime,
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, raw)
		if err == nil {
			return parsed
		}
	}
	if epochSeconds, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return time.Unix(epochSeconds, 0).UTC()
	}
	if epochFloat, err := strconv.ParseFloat(raw, 64); err == nil {
		return time.Unix(int64(epochFloat), 0).UTC()
	}
	return time.Time{}
}
