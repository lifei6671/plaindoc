package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

type gormReaderPageRepository struct {
	db *gorm.DB
}

type readerPageDocumentRow struct {
	DocumentID     string  `gorm:"column:document_id"`
	NodeID         string  `gorm:"column:node_id"`
	ReaderSlug     *string `gorm:"column:reader_slug"`
	ThemeID        string  `gorm:"column:theme_id"`
	Visibility     string  `gorm:"column:visibility"`
	Title          string  `gorm:"column:title"`
	ContentMD      string  `gorm:"column:content_md"`
	Version        int     `gorm:"column:version"`
	AuthorNickname string  `gorm:"column:author_nickname"`
	UpdatedAtRaw   string  `gorm:"column:updated_at"`
	SpaceID        string  `gorm:"column:space_id"`
}

type readerPageTreeNodeRow struct {
	NodeID             string          `gorm:"column:node_id"`
	DocumentID         *string         `gorm:"column:document_id"`
	ReaderSlug         *string         `gorm:"column:reader_slug"`
	ParentNodeID       *string         `gorm:"column:parent_node_id"`
	Type               models.NodeType `gorm:"column:type"`
	Title              string          `gorm:"column:title"`
	Sort               int             `gorm:"column:sort"`
	DocumentVisibility *string         `gorm:"column:document_visibility"`
}

type readerPageDocumentIDRow struct {
	DocumentID string `gorm:"column:document_id"`
}

type readerPageResolvedDocumentRow struct {
	DocumentID string `gorm:"column:document_id"`
}

// NewGormReaderPageRepository 创建阅读页数据仓储实现。
func NewGormReaderPageRepository(db *gorm.DB) ReaderPageRepository {
	return &gormReaderPageRepository{db: db}
}

func (r *gormReaderPageRepository) ResolveDocumentID(
	ctx context.Context,
	spaceID string,
	rawDocumentID string,
) (string, error) {
	if r == nil || r.db == nil {
		return "", fmt.Errorf("reader page repository db is nil")
	}

	normalizedSpaceID := strings.TrimSpace(spaceID)
	normalizedDocumentID := strings.TrimSpace(rawDocumentID)
	if normalizedSpaceID == "" || normalizedDocumentID == "" {
		return "", gorm.ErrRecordNotFound
	}
	normalizedDocumentSlug := strings.ToLower(normalizedDocumentID)

	tryResolve := func(
		useSpaceFilter bool,
		condition string,
		args ...any,
	) (string, error) {
		query := r.db.WithContext(ctx).Table("documents AS d").Select("d.document_id")
		if useSpaceFilter {
			query = query.
				Joins("JOIN nodes AS n ON n.node_id = d.node_id").
				Where("n.space_id = ?", normalizedSpaceID)
		}

		var row readerPageResolvedDocumentRow
		err := query.Where(condition, args...).Take(&row).Error
		if err == nil {
			return strings.TrimSpace(row.DocumentID), nil
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}

	spaceScopedMatchers := []struct {
		condition string
		args      []any
	}{
		{condition: "d.document_id = ?", args: []any{normalizedDocumentID}},
		{condition: "d.node_id = ?", args: []any{normalizedDocumentID}},
		{condition: "n.reader_slug = ?", args: []any{normalizedDocumentSlug}},
	}
	for _, matcher := range spaceScopedMatchers {
		resolvedDocumentID, err := tryResolve(true, matcher.condition, matcher.args...)
		if err != nil {
			return "", err
		}
		if resolvedDocumentID != "" {
			return resolvedDocumentID, nil
		}
	}

	globalMatchers := []struct {
		condition string
		args      []any
	}{
		{condition: "d.document_id = ?", args: []any{normalizedDocumentID}},
		{condition: "d.node_id = ?", args: []any{normalizedDocumentID}},
	}
	for _, matcher := range globalMatchers {
		resolvedDocumentID, err := tryResolve(false, matcher.condition, matcher.args...)
		if err != nil {
			return "", err
		}
		if resolvedDocumentID != "" {
			return resolvedDocumentID, nil
		}
	}
	return "", gorm.ErrRecordNotFound
}

func (r *gormReaderPageRepository) GetDocumentByDocumentID(
	ctx context.Context,
	documentID string,
) (*ReaderPageDocumentRecord, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("reader page repository db is nil")
	}

	normalizedDocumentID := strings.TrimSpace(documentID)
	if normalizedDocumentID == "" {
		return nil, gorm.ErrRecordNotFound
	}

	var row readerPageDocumentRow
	err := r.db.WithContext(ctx).
		Table("documents AS d").
		Select(
			"d.document_id",
			"d.node_id",
			"n.reader_slug",
			"d.theme_id",
			"d.visibility",
			"d.title",
			"d.content_md",
			"d.version",
			"COALESCE(NULLIF(TRIM(u_creator.name), ''), '未知作者') AS author_nickname",
			"d.updated_at",
			"n.space_id AS space_id",
		).
		Joins("JOIN nodes AS n ON n.node_id = d.node_id").
		Joins("LEFT JOIN users AS u_creator ON u_creator.user_id = d.created_by_user_id").
		Where("d.document_id = ?", normalizedDocumentID).
		Take(&row).Error
	if err != nil {
		return nil, err
	}

	return &ReaderPageDocumentRecord{
		DocumentID:     strings.TrimSpace(row.DocumentID),
		NodeID:         strings.TrimSpace(row.NodeID),
		ReaderSlug:     trimReaderOptionalString(row.ReaderSlug),
		ThemeID:        strings.TrimSpace(row.ThemeID),
		Visibility:     strings.TrimSpace(row.Visibility),
		Title:          strings.TrimSpace(row.Title),
		ContentMD:      row.ContentMD,
		Version:        row.Version,
		AuthorNickname: strings.TrimSpace(row.AuthorNickname),
		UpdatedAt:      parseRecordTime(row.UpdatedAtRaw),
		SpaceID:        strings.TrimSpace(row.SpaceID),
	}, nil
}

func (r *gormReaderPageRepository) ListSpaceDocumentIDs(
	ctx context.Context,
	spaceID string,
) ([]string, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("reader page repository db is nil")
	}
	normalizedSpaceID := strings.TrimSpace(spaceID)
	if normalizedSpaceID == "" {
		return []string{}, nil
	}

	rows := make([]readerPageDocumentIDRow, 0, 64)
	if err := r.db.WithContext(ctx).
		Table("documents AS d").
		Select("d.document_id").
		Joins("JOIN nodes AS n ON n.node_id = d.node_id").
		Where("n.space_id = ?", normalizedSpaceID).
		Where("d.deleted_at IS NULL").
		Order("CASE WHEN n.parent_node_id IS NULL THEN 0 ELSE 1 END ASC").
		Order("n.parent_node_id ASC").
		Order("n.sort ASC, n.id ASC, d.id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]string, 0, len(rows))
	for _, row := range rows {
		documentID := strings.TrimSpace(row.DocumentID)
		if documentID == "" {
			continue
		}
		result = append(result, documentID)
	}
	return result, nil
}

func (r *gormReaderPageRepository) ListTreeNodesBySpaceID(
	ctx context.Context,
	spaceID string,
) ([]ReaderPageTreeNodeRecord, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("reader page repository db is nil")
	}
	normalizedSpaceID := strings.TrimSpace(spaceID)
	if normalizedSpaceID == "" {
		return []ReaderPageTreeNodeRecord{}, nil
	}

	rows := make([]readerPageTreeNodeRow, 0, 64)
	if err := r.db.WithContext(ctx).
		Table("nodes AS n").
		Select(
			"n.node_id",
			"d.document_id AS document_id",
			"n.reader_slug",
			"n.parent_node_id",
			"n.type",
			"n.title",
			"n.sort",
			"d.visibility AS document_visibility",
		).
		Joins("LEFT JOIN documents AS d ON d.node_id = n.node_id").
		Where("n.space_id = ?", normalizedSpaceID).
		Order("n.parent_node_id ASC, n.sort ASC, n.id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]ReaderPageTreeNodeRecord, 0, len(rows))
	for _, row := range rows {
		result = append(result, ReaderPageTreeNodeRecord{
			NodeID:             strings.TrimSpace(row.NodeID),
			DocumentID:         trimReaderOptionalString(row.DocumentID),
			ReaderSlug:         trimReaderOptionalString(row.ReaderSlug),
			ParentNodeID:       trimReaderOptionalString(row.ParentNodeID),
			Type:               row.Type,
			Title:              strings.TrimSpace(row.Title),
			Sort:               row.Sort,
			DocumentVisibility: trimReaderOptionalString(row.DocumentVisibility),
		})
	}
	return result, nil
}

func trimReaderOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func readerPageNowUTC() time.Time {
	return time.Now().UTC()
}
