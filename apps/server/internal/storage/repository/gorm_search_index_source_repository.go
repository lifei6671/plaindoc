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

type searchIndexSourceRow = searchIndexSourceRowDB

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
		Order(qualifiedColumn("d", models.DocumentColumns.ID) + " ASC").
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
		Where(qualifiedColumn("d", models.DocumentColumns.DocumentID)+" = ?", normalizedDocumentID).
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
		Where(qualifiedColumn("s", models.SpaceColumns.SpaceID)+" = ?", normalizedSpaceID).
		Order(qualifiedColumn("d", models.DocumentColumns.ID) + " ASC").
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
		Table(tableWithAlias(models.Document{}, "d")).
		Joins("JOIN "+tableName(models.Node{})+" AS n ON "+qualifiedColumn("n", models.NodeColumns.NodeID)+" = "+qualifiedColumn("d", models.DocumentColumns.NodeID)).
		Joins("JOIN "+tableName(models.Space{})+" AS s ON "+qualifiedColumn("s", models.SpaceColumns.SpaceID)+" = "+qualifiedColumn("n", models.NodeColumns.SpaceID)).
		Where(qualifiedColumn("s", models.SpaceColumns.Status)+" = ?", models.EntityStatusActive).
		Where(qualifiedColumn("s", models.SpaceColumns.DeletedAt)+" IS NULL").
		Where(qualifiedColumn("d", models.DocumentColumns.Status)+" = ?", models.EntityStatusActive).
		Where(qualifiedColumn("d", models.DocumentColumns.DeletedAt)+" IS NULL").
		Where(
			"("+qualifiedColumn("d", models.DocumentColumns.Format)+" = ?) OR ("+qualifiedColumn("d", models.DocumentColumns.Format)+" IN (?, ?) AND "+qualifiedColumn("d", models.DocumentColumns.RenderStatus)+" = ? AND TRIM("+qualifiedColumn("d", models.DocumentColumns.ContentMD)+") <> '')",
			models.DocumentFormatMarkdown,
			models.DocumentFormatDOCX,
			models.DocumentFormatXLSX,
			models.DocumentRenderStatusSuccess,
		)
}

func (r *gormSearchIndexSourceRepository) selectSearchIndexSourceColumns(query *gorm.DB) *gorm.DB {
	return query.Select(
		qualifiedColumn("s", models.SpaceColumns.SpaceID)+" AS space_id",
		qualifiedColumn("d", models.DocumentColumns.DocumentID)+" AS document_id",
		qualifiedColumn("d", models.DocumentColumns.NodeID)+" AS node_id",
		qualifiedColumn("d", models.DocumentColumns.Format)+" AS format",
		qualifiedColumn("d", models.DocumentColumns.Title)+" AS title",
		qualifiedColumn("d", models.DocumentColumns.ContentMD)+" AS content_md",
		qualifiedColumn("s", models.SpaceColumns.Visibility)+" AS space_visibility",
		qualifiedColumn("d", models.DocumentColumns.Visibility)+" AS doc_visibility",
		qualifiedColumn("d", models.DocumentColumns.UpdatedAt)+" AS updated_at",
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
