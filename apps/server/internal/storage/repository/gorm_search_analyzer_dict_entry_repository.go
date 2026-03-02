package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

type gormSearchAnalyzerDictEntryRepository struct {
	db *gorm.DB
}

type searchAnalyzerDictEntryRow struct {
	ID              int64   `gorm:"column:id"`
	Analyzer        string  `gorm:"column:analyzer"`
	Term            string  `gorm:"column:term"`
	Weight          *int    `gorm:"column:weight"`
	Tag             string  `gorm:"column:tag"`
	Status          string  `gorm:"column:status"`
	CreatedByUserID *string `gorm:"column:created_by_user_id"`
	UpdatedByUserID *string `gorm:"column:updated_by_user_id"`
	CreatedAtRaw    string  `gorm:"column:created_at"`
	UpdatedAtRaw    string  `gorm:"column:updated_at"`
}

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

	normalizedAnalyzer := strings.ToLower(strings.TrimSpace(params.Analyzer))
	if normalizedAnalyzer == "" {
		return []models.SearchAnalyzerDictEntry{}, 0, nil
	}

	baseQuery := r.db.WithContext(ctx).
		Table("search_analyzer_dict_entries").
		Where("LOWER(analyzer) = ?", normalizedAnalyzer)

	statuses := normalizeSearchAnalyzerDictEntryStatuses(params.Statuses)
	if len(statuses) > 0 {
		baseQuery = baseQuery.Where("LOWER(status) IN ?", statuses)
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
			"id",
			"analyzer",
			"term",
			"weight",
			"tag",
			"status",
			"created_by_user_id",
			"updated_by_user_id",
			"created_at",
			"updated_at",
		).
		Order("updated_at DESC, id DESC").
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

	normalizedAnalyzer := strings.ToLower(strings.TrimSpace(analyzer))
	if normalizedAnalyzer == "" {
		return []models.SearchAnalyzerDictEntry{}, nil
	}

	rows := make([]searchAnalyzerDictEntryRow, 0, 32)
	if err := r.db.WithContext(ctx).
		Table("search_analyzer_dict_entries").
		Select(
			"id",
			"analyzer",
			"term",
			"weight",
			"tag",
			"status",
			"created_by_user_id",
			"updated_by_user_id",
			"created_at",
			"updated_at",
		).
		Where("LOWER(analyzer) = ? AND LOWER(status) = ?", normalizedAnalyzer, models.SearchAnalyzerDictEntryStatusActive).
		Order("LENGTH(term) DESC, updated_at DESC, id DESC").
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
		Table("search_analyzer_dict_entries").
		Select(
			"id",
			"analyzer",
			"term",
			"weight",
			"tag",
			"status",
			"created_by_user_id",
			"updated_by_user_id",
			"created_at",
			"updated_at",
		).
		Where("id = ?", id).
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
	normalizedAnalyzer := strings.ToLower(strings.TrimSpace(analyzer))
	normalizedTerm := strings.TrimSpace(term)
	if normalizedAnalyzer == "" || normalizedTerm == "" {
		return nil, gorm.ErrRecordNotFound
	}

	var row searchAnalyzerDictEntryRow
	if err := r.db.WithContext(ctx).
		Table("search_analyzer_dict_entries").
		Select(
			"id",
			"analyzer",
			"term",
			"weight",
			"tag",
			"status",
			"created_by_user_id",
			"updated_by_user_id",
			"created_at",
			"updated_at",
		).
		Where("LOWER(analyzer) = ? AND term = ?", normalizedAnalyzer, normalizedTerm).
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
	if _, exists := updates["updated_at"]; !exists {
		updates["updated_at"] = time.Now().UTC()
	}

	tx := r.db.WithContext(ctx).
		Model(&models.SearchAnalyzerDictEntry{}).
		Where("id = ?", id).
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
		CreatedAt:       parseRecordTime(row.CreatedAtRaw),
		UpdatedAt:       parseRecordTime(row.UpdatedAtRaw),
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
