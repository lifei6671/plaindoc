package provider

import (
	"context"
	"errors"
	"strings"

	searchanalyzer "github.com/lifei6671/plaindoc/apps/server/internal/search/analyzer"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
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
	db *gorm.DB
}

// NewDatabaseProvider 创建 DatabaseProvider。
func NewDatabaseProvider(db *gorm.DB) *DatabaseProvider {
	return &DatabaseProvider{db: db}
}

func (p *DatabaseProvider) Name() string {
	return "database"
}

func (p *DatabaseProvider) Health(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if p == nil || p.db == nil {
		return errors.New("database search provider db is nil")
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
	actorUserID := strings.TrimSpace(request.ActorUserID)
	spaceID := strings.TrimSpace(request.SpaceID)

	baseQuery := p.db.WithContext(ctx).
		Table("documents AS d").
		Joins("JOIN nodes AS n ON n.node_id = d.node_id").
		Joins("JOIN spaces AS s ON s.space_id = n.space_id").
		Where("s.status = ? AND s.deleted_at IS NULL", models.EntityStatusActive).
		Where("d.status = ? AND d.deleted_at IS NULL", models.EntityStatusActive)

	if spaceID != "" {
		baseQuery = baseQuery.Where("s.space_id = ?", spaceID)
	}

	if actorUserID == "" {
		baseQuery = baseQuery.Where(
			"s.visibility = ? AND d.visibility = ?",
			models.VisibilityPublic,
			models.VisibilityPublic,
		)
	} else {
		baseQuery = baseQuery.
			Joins("LEFT JOIN space_members AS sm ON sm.space_id = s.space_id AND sm.user_id = ?", actorUserID).
			Where(
				"("+
					"s.owner_user_id = ? OR "+
					"((s.visibility IN (?,?)) AND (d.visibility IN (?,?))) OR "+
					"((s.visibility = ? OR d.visibility = ?) AND sm.id IS NOT NULL)"+
					")",
				actorUserID,
				models.VisibilityPublic,
				models.VisibilityAuthenticated,
				models.VisibilityPublic,
				models.VisibilityAuthenticated,
				models.VisibilityMember,
				models.VisibilityMember,
			)
	}

	for _, term := range terms {
		likePattern := "%" + strings.ToLower(strings.TrimSpace(term)) + "%"
		baseQuery = baseQuery.Where("(LOWER(d.title) LIKE ? OR LOWER(d.content_md) LIKE ?)", likePattern, likePattern)
	}

	var total int64
	if err := baseQuery.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return SearchResponse{}, err
	}
	if total <= 0 {
		return SearchResponse{Total: 0, Hits: []SearchHit{}}, nil
	}

	type searchRow struct {
		DocumentID string `gorm:"column:document_id"`
		ContentMD  string `gorm:"column:content_md"`
	}

	rows := make([]searchRow, 0, pageSize)
	query := baseQuery.Session(&gorm.Session{}).
		Select("d.document_id", "d.content_md")
	if request.Sort == SortModeUpdatedAtDesc || request.Sort == SortModeRelevance {
		query = query.Order("d.updated_at DESC, d.id DESC")
	}
	if err := query.Limit(pageSize).Offset(offset).Find(&rows).Error; err != nil {
		return SearchResponse{}, err
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
