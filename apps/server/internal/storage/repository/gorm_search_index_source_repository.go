package repository

import (
	"context"
	"database/sql/driver"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

const defaultSearchIndexSourcePageLimit = 200

type gormSearchIndexSourceRepository struct {
	db *gorm.DB
}

type searchIndexSourceRow struct {
	SpaceID         string                    `gorm:"column:space_id"`
	DocumentID      string                    `gorm:"column:document_id"`
	NodeID          string                    `gorm:"column:node_id"`
	Format          models.DocumentFormat     `gorm:"column:format"`
	Title           string                    `gorm:"column:title"`
	ContentMD       string                    `gorm:"column:content_md"`
	SpaceVisibility string                    `gorm:"column:space_visibility"`
	DocVisibility   string                    `gorm:"column:doc_visibility"`
	UpdatedAt       searchIndexSourceScanTime `gorm:"column:updated_at"`
}

type searchIndexSourceScanTime struct {
	time.Time
}

func (s searchIndexSourceScanTime) Value() (driver.Value, error) {
	if s.Time.IsZero() {
		return nil, nil
	}
	return s.Time, nil
}

// NewGormSearchIndexSourceRepository 创建索引源数据仓储实现。
func NewGormSearchIndexSourceRepository(db *gorm.DB) SearchIndexSourceRepository {
	return &gormSearchIndexSourceRepository{db: db}
}

func (r *gormSearchIndexSourceRepository) ListActiveDocuments(
	ctx context.Context,
	params ListSearchIndexSourceDocumentsParams,
) ([]SearchIndexSourceDocumentRecord, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("search index source repository db is nil")
	}

	limit := params.Limit
	if limit <= 0 {
		limit = defaultSearchIndexSourcePageLimit
	}
	offset := params.Offset
	if offset < 0 {
		offset = 0
	}

	rows := make([]searchIndexSourceRow, 0, limit)
	err := r.selectSearchIndexSourceColumns(r.baseActiveDocumentsQuery(ctx)).
		Order("d.id ASC").
		Limit(limit).
		Offset(offset).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return mapSearchIndexSourceRows(rows), nil
}

func (r *gormSearchIndexSourceRepository) GetActiveDocumentByDocumentID(
	ctx context.Context,
	documentID string,
) (*SearchIndexSourceDocumentRecord, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("search index source repository db is nil")
	}

	normalizedDocumentID := strings.TrimSpace(documentID)
	if normalizedDocumentID == "" {
		return nil, gorm.ErrRecordNotFound
	}

	var row searchIndexSourceRow
	err := r.selectSearchIndexSourceColumns(r.baseActiveDocumentsQuery(ctx)).
		Where("d.document_id = ?", normalizedDocumentID).
		Take(&row).Error
	if err != nil {
		return nil, err
	}

	result := mapSearchIndexSourceRow(row)
	return &result, nil
}

func (r *gormSearchIndexSourceRepository) ListActiveDocumentsBySpaceID(
	ctx context.Context,
	params ListSearchIndexSourceDocumentsBySpaceParams,
) ([]SearchIndexSourceDocumentRecord, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("search index source repository db is nil")
	}

	normalizedSpaceID := strings.TrimSpace(params.SpaceID)
	if normalizedSpaceID == "" {
		return []SearchIndexSourceDocumentRecord{}, nil
	}

	limit := params.Limit
	if limit <= 0 {
		limit = defaultSearchIndexSourcePageLimit
	}
	offset := params.Offset
	if offset < 0 {
		offset = 0
	}

	rows := make([]searchIndexSourceRow, 0, limit)
	err := r.selectSearchIndexSourceColumns(r.baseActiveDocumentsQuery(ctx)).
		Where("s.space_id = ?", normalizedSpaceID).
		Order("d.id ASC").
		Limit(limit).
		Offset(offset).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return mapSearchIndexSourceRows(rows), nil
}

func (r *gormSearchIndexSourceRepository) baseActiveDocumentsQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).
		Table("documents AS d").
		Joins("JOIN nodes AS n ON n.node_id = d.node_id").
		Joins("JOIN spaces AS s ON s.space_id = n.space_id").
		Where("s.status = ? AND s.deleted_at IS NULL", models.EntityStatusActive).
		Where("d.status = ? AND d.deleted_at IS NULL", models.EntityStatusActive).
		Where(
			"(d.format = ?) OR (d.format IN (?, ?) AND d.render_status = ? AND TRIM(d.content_md) <> '')",
			models.DocumentFormatMarkdown,
			models.DocumentFormatDOCX,
			models.DocumentFormatXLSX,
			models.DocumentRenderStatusSuccess,
		)
}

func (r *gormSearchIndexSourceRepository) selectSearchIndexSourceColumns(query *gorm.DB) *gorm.DB {
	return query.Select(
		"s.space_id AS space_id",
		"d.document_id AS document_id",
		"d.node_id AS node_id",
		"d.format AS format",
		"d.title AS title",
		"d.content_md AS content_md",
		"s.visibility AS space_visibility",
		"d.visibility AS doc_visibility",
		"d.updated_at AS updated_at",
	)
}

func mapSearchIndexSourceRows(rows []searchIndexSourceRow) []SearchIndexSourceDocumentRecord {
	if len(rows) == 0 {
		return []SearchIndexSourceDocumentRecord{}
	}
	result := make([]SearchIndexSourceDocumentRecord, 0, len(rows))
	for _, row := range rows {
		result = append(result, mapSearchIndexSourceRow(row))
	}
	return result
}

func mapSearchIndexSourceRow(row searchIndexSourceRow) SearchIndexSourceDocumentRecord {
	return SearchIndexSourceDocumentRecord{
		SpaceID:         strings.TrimSpace(row.SpaceID),
		DocumentID:      strings.TrimSpace(row.DocumentID),
		NodeID:          strings.TrimSpace(row.NodeID),
		Format:          row.Format,
		Title:           strings.TrimSpace(row.Title),
		ContentMD:       strings.TrimSpace(row.ContentMD),
		SpaceVisibility: strings.TrimSpace(row.SpaceVisibility),
		DocVisibility:   strings.TrimSpace(row.DocVisibility),
		UpdatedAt:       row.UpdatedAt.Time,
	}
}

func (s *searchIndexSourceScanTime) Scan(value any) error {
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
		s.Time = parseSearchIndexSourceTimeString(string(current))
		return nil
	case string:
		s.Time = parseSearchIndexSourceTimeString(current)
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
		s.Time = parseSearchIndexSourceTimeString(fmt.Sprint(current))
		return nil
	}
}

func parseSearchIndexSourceTimeString(value string) time.Time {
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
