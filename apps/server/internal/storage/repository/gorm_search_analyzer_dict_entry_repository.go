package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/recordtime"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type gormSearchAnalyzerDictEntryRepository struct {
	db *gorm.DB
}

type searchAnalyzerDictEntryRow = searchAnalyzerDictEntryRowDB

// NewGormSearchAnalyzerDictEntryRepository 创建分词词典词条仓储实现。
func NewGormSearchAnalyzerDictEntryRepository(db *gorm.DB) SearchAnalyzerDictEntryRepository {
	return &gormSearchAnalyzerDictEntryRepository{db: db}
}

func (r *gormSearchAnalyzerDictEntryRepository) List(
	ctx context.Context,
	params ListSearchAnalyzerDictEntriesParams,
) ([]models.SearchAnalyzerDictEntry, int64, error) {
	if r == nil || r.db == nil {
		return nil, 0, fmt.Errorf("search analyzer dict entry repository db is nil")
	}

	normalizedAnalyzer := strings.TrimSpace(params.Analyzer)
	if normalizedAnalyzer == "" {
		return []models.SearchAnalyzerDictEntry{}, 0, nil
	}

	baseQuery := r.db.WithContext(ctx).
		Model(&models.SearchAnalyzerDictEntry{}).
		Where(models.SearchAnalyzerDictEntryColumns.Analyzer+" = ?", normalizedAnalyzer)

	statuses := normalizeSearchAnalyzerDictEntryStatuses(params.Statuses)
	if len(statuses) > 0 {
		baseQuery = baseQuery.Where("LOWER("+models.SearchAnalyzerDictEntryColumns.Status+") IN ?", statuses)
	}

	var total int64
	if err := baseQuery.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := params.Offset
	if offset < 0 {
		offset = 0
	}

	rows := make([]searchAnalyzerDictEntryRow, 0, limit)
	if err := baseQuery.Session(&gorm.Session{}).
		Select(
			models.SearchAnalyzerDictEntryColumns.ID,
			models.SearchAnalyzerDictEntryColumns.Analyzer,
			models.SearchAnalyzerDictEntryColumns.Term,
			models.SearchAnalyzerDictEntryColumns.Weight,
			models.SearchAnalyzerDictEntryColumns.Tag,
			models.SearchAnalyzerDictEntryColumns.Status,
			models.SearchAnalyzerDictEntryColumns.CreatedByUserID,
			models.SearchAnalyzerDictEntryColumns.UpdatedByUserID,
			models.SearchAnalyzerDictEntryColumns.CreatedAt+" AS created_at_raw",
			models.SearchAnalyzerDictEntryColumns.UpdatedAt+" AS updated_at_raw",
		).
		Order(clause.OrderByColumn{
			Column: clause.Column{Name: models.SearchAnalyzerDictEntryColumns.UpdatedAt},
			Desc:   true,
		}).
		Order(clause.OrderByColumn{
			Column: clause.Column{Name: models.SearchAnalyzerDictEntryColumns.ID},
			Desc:   true,
		}).
		Offset(offset).
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	items := make([]models.SearchAnalyzerDictEntry, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapSearchAnalyzerDictEntryRow(row))
	}
	return items, total, nil
}

func (r *gormSearchAnalyzerDictEntryRepository) ListActiveByAnalyzer(
	ctx context.Context,
	analyzer string,
) ([]models.SearchAnalyzerDictEntry, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("search analyzer dict entry repository db is nil")
	}

	normalizedAnalyzer := strings.TrimSpace(analyzer)
	if normalizedAnalyzer == "" {
		return []models.SearchAnalyzerDictEntry{}, nil
	}

	rows := make([]searchAnalyzerDictEntryRow, 0, 32)
	if err := r.db.WithContext(ctx).
		Model(&models.SearchAnalyzerDictEntry{}).
		Select(
			models.SearchAnalyzerDictEntryColumns.ID,
			models.SearchAnalyzerDictEntryColumns.Analyzer,
			models.SearchAnalyzerDictEntryColumns.Term,
			models.SearchAnalyzerDictEntryColumns.Weight,
			models.SearchAnalyzerDictEntryColumns.Tag,
			models.SearchAnalyzerDictEntryColumns.Status,
			models.SearchAnalyzerDictEntryColumns.CreatedByUserID,
			models.SearchAnalyzerDictEntryColumns.UpdatedByUserID,
			models.SearchAnalyzerDictEntryColumns.CreatedAt+" AS created_at_raw",
			models.SearchAnalyzerDictEntryColumns.UpdatedAt+" AS updated_at_raw",
		).
		Where(models.SearchAnalyzerDictEntryColumns.Analyzer+" = ?", normalizedAnalyzer).
		Where(models.SearchAnalyzerDictEntryColumns.Status+" = ?", models.SearchAnalyzerDictEntryStatusActive).
		Order("LENGTH(" + models.SearchAnalyzerDictEntryColumns.Term + ") DESC").
		Order(clause.OrderByColumn{
			Column: clause.Column{Name: models.SearchAnalyzerDictEntryColumns.UpdatedAt},
			Desc:   true,
		}).
		Order(clause.OrderByColumn{
			Column: clause.Column{Name: models.SearchAnalyzerDictEntryColumns.ID},
			Desc:   true,
		}).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]models.SearchAnalyzerDictEntry, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapSearchAnalyzerDictEntryRow(row))
	}
	return items, nil
}

func (r *gormSearchAnalyzerDictEntryRepository) GetByID(
	ctx context.Context,
	id int64,
) (*models.SearchAnalyzerDictEntry, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("search analyzer dict entry repository db is nil")
	}
	if id <= 0 {
		return nil, gorm.ErrRecordNotFound
	}

	var row searchAnalyzerDictEntryRow
	if err := r.db.WithContext(ctx).
		Model(&models.SearchAnalyzerDictEntry{}).
		Select(
			models.SearchAnalyzerDictEntryColumns.ID,
			models.SearchAnalyzerDictEntryColumns.Analyzer,
			models.SearchAnalyzerDictEntryColumns.Term,
			models.SearchAnalyzerDictEntryColumns.Weight,
			models.SearchAnalyzerDictEntryColumns.Tag,
			models.SearchAnalyzerDictEntryColumns.Status,
			models.SearchAnalyzerDictEntryColumns.CreatedByUserID,
			models.SearchAnalyzerDictEntryColumns.UpdatedByUserID,
			models.SearchAnalyzerDictEntryColumns.CreatedAt+" AS created_at_raw",
			models.SearchAnalyzerDictEntryColumns.UpdatedAt+" AS updated_at_raw",
		).
		Where(models.SearchAnalyzerDictEntryColumns.ID+" = ?", id).
		Take(&row).Error; err != nil {
		return nil, err
	}

	item := mapSearchAnalyzerDictEntryRow(row)
	return &item, nil
}

func (r *gormSearchAnalyzerDictEntryRepository) GetByAnalyzerAndTerm(
	ctx context.Context,
	analyzer string,
	term string,
) (*models.SearchAnalyzerDictEntry, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("search analyzer dict entry repository db is nil")
	}
	normalizedAnalyzer := strings.TrimSpace(analyzer)
	normalizedTerm := strings.TrimSpace(term)
	if normalizedAnalyzer == "" || normalizedTerm == "" {
		return nil, gorm.ErrRecordNotFound
	}

	var row searchAnalyzerDictEntryRow
	if err := r.db.WithContext(ctx).
		Model(&models.SearchAnalyzerDictEntry{}).
		Select(
			models.SearchAnalyzerDictEntryColumns.ID,
			models.SearchAnalyzerDictEntryColumns.Analyzer,
			models.SearchAnalyzerDictEntryColumns.Term,
			models.SearchAnalyzerDictEntryColumns.Weight,
			models.SearchAnalyzerDictEntryColumns.Tag,
			models.SearchAnalyzerDictEntryColumns.Status,
			models.SearchAnalyzerDictEntryColumns.CreatedByUserID,
			models.SearchAnalyzerDictEntryColumns.UpdatedByUserID,
			models.SearchAnalyzerDictEntryColumns.CreatedAt+" AS created_at_raw",
			models.SearchAnalyzerDictEntryColumns.UpdatedAt+" AS updated_at_raw",
		).
		Where(models.SearchAnalyzerDictEntryColumns.Analyzer+" = ?", normalizedAnalyzer).
		Where(models.SearchAnalyzerDictEntryColumns.Term+" = ?", normalizedTerm).
		Take(&row).Error; err != nil {
		return nil, err
	}

	item := mapSearchAnalyzerDictEntryRow(row)
	return &item, nil
}

func (r *gormSearchAnalyzerDictEntryRepository) Create(
	ctx context.Context,
	entry *models.SearchAnalyzerDictEntry,
) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("search analyzer dict entry repository db is nil")
	}
	if entry == nil {
		return fmt.Errorf("search analyzer dict entry is nil")
	}
	return r.db.WithContext(ctx).Create(entry).Error
}

func (r *gormSearchAnalyzerDictEntryRepository) UpdateByID(
	ctx context.Context,
	id int64,
	updates map[string]any,
) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("search analyzer dict entry repository db is nil")
	}
	if id <= 0 || len(updates) == 0 {
		return false, nil
	}
	if _, exists := updates[models.SearchAnalyzerDictEntryColumns.UpdatedAt]; !exists {
		updates[models.SearchAnalyzerDictEntryColumns.UpdatedAt] = time.Now().UTC()
	}

	tx := r.db.WithContext(ctx).
		Model(&models.SearchAnalyzerDictEntry{}).
		Where(models.SearchAnalyzerDictEntryColumns.ID+" = ?", id).
		Updates(updates)
	if tx.Error != nil {
		return false, tx.Error
	}
	return tx.RowsAffected > 0, nil
}

func mapSearchAnalyzerDictEntryRow(row searchAnalyzerDictEntryRow) models.SearchAnalyzerDictEntry {
	return models.SearchAnalyzerDictEntry{
		ID:              row.ID,
		Analyzer:        strings.ToLower(strings.TrimSpace(row.Analyzer)),
		Term:            strings.TrimSpace(row.Term),
		Weight:          row.Weight,
		Tag:             strings.TrimSpace(row.Tag),
		Status:          strings.ToLower(strings.TrimSpace(row.Status)),
		CreatedByUserID: row.CreatedByUserID,
		UpdatedByUserID: row.UpdatedByUserID,
		CreatedAt:       recordtime.Parse(row.CreatedAtRaw),
		UpdatedAt:       recordtime.Parse(row.UpdatedAtRaw),
	}
}

func normalizeSearchAnalyzerDictEntryStatuses(input []string) []string {
	if len(input) == 0 {
		return nil
	}
	items := make([]string, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, item := range input {
		normalized := strings.ToLower(strings.TrimSpace(item))
		if normalized != models.SearchAnalyzerDictEntryStatusActive &&
			normalized != models.SearchAnalyzerDictEntryStatusDeleted {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		items = append(items, normalized)
	}
	return items
}
