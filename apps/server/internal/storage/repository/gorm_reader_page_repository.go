package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/recordtime"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

type gormReaderPageRepository struct {
	db *gorm.DB
}

type readerPageDocumentRow = readerPageDocumentRowDB

type readerPageTreeNodeRow = readerPageTreeNodeRowDB

type readerPageDocumentIDRow = readerPageDocumentIDRowDB

type readerPageResolvedDocumentRow = readerPageResolvedDocumentRowDB

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
		query := r.db.WithContext(ctx).
			Table(tableWithAlias(models.Document{}, "d")).
			Select("d." + models.DocumentColumns.DocumentID + " AS document_id")
		if useSpaceFilter {
			query = query.
				Joins("JOIN "+tableName(models.Node{})+" AS n ON n."+models.NodeColumns.NodeID+" = d."+models.DocumentColumns.NodeID).
				Where("n."+models.NodeColumns.SpaceID+" = ?", normalizedSpaceID)
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
		{condition: "d." + models.DocumentColumns.DocumentID + " = ?", args: []any{normalizedDocumentID}},
		{condition: "d." + models.DocumentColumns.NodeID + " = ?", args: []any{normalizedDocumentID}},
		{condition: "n." + models.NodeColumns.ReaderSlug + " = ?", args: []any{normalizedDocumentSlug}},
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
		{condition: "d." + models.DocumentColumns.DocumentID + " = ?", args: []any{normalizedDocumentID}},
		{condition: "d." + models.DocumentColumns.NodeID + " = ?", args: []any{normalizedDocumentID}},
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
		Table(tableWithAlias(models.Document{}, "d")).
		Select(
			"d."+models.DocumentColumns.DocumentID+" AS document_id",
			"d."+models.DocumentColumns.NodeID+" AS node_id",
			"n."+models.NodeColumns.ReaderSlug+" AS reader_slug",
			"d."+models.DocumentColumns.ThemeID+" AS theme_id",
			"d."+models.DocumentColumns.Format+" AS format",
			"d."+models.DocumentColumns.Visibility+" AS visibility",
			"d."+models.DocumentColumns.Title+" AS title",
			"d."+models.DocumentColumns.ContentMD+" AS content_md",
			"d."+models.DocumentColumns.RenderStatus+" AS render_status",
			"d."+models.DocumentColumns.RenderError+" AS render_error",
			"d."+models.DocumentColumns.RenderedAt+" AS rendered_at_raw",
			"d."+models.DocumentColumns.Version+" AS version",
			"d."+models.DocumentColumns.SourceBlobID+" AS source_blob_id",
			"d."+models.DocumentColumns.SourceFileName+" AS source_file_name",
			"d."+models.DocumentColumns.SourceMimeType+" AS source_mime_type",
			"d."+models.DocumentColumns.ContentVersion+" AS content_version",
			"COALESCE(NULLIF(TRIM(u_creator."+models.UserColumns.Name+"), ''), '未知作者') AS author_nickname",
			"d."+models.DocumentColumns.UpdatedAt+" AS updated_at_raw",
			"n."+models.NodeColumns.SpaceID+" AS space_id",
		).
		Joins("JOIN "+tableName(models.Node{})+" AS n ON n."+models.NodeColumns.NodeID+" = d."+models.DocumentColumns.NodeID).
		Joins("LEFT JOIN "+tableName(models.User{})+" AS u_creator ON u_creator."+models.UserColumns.UserID+" = d."+models.DocumentColumns.CreatedByUserID).
		Where("d."+models.DocumentColumns.DocumentID+" = ?", normalizedDocumentID).
		Take(&row).Error
	if err != nil {
		return nil, err
	}

	return &ReaderPageDocumentRecord{
		DocumentID:     strings.TrimSpace(row.DocumentID),
		NodeID:         strings.TrimSpace(row.NodeID),
		ReaderSlug:     trimReaderOptionalString(row.ReaderSlug),
		ThemeID:        strings.TrimSpace(row.ThemeID),
		Format:         models.NormalizeDocumentFormat(row.Format),
		Visibility:     strings.TrimSpace(row.Visibility),
		Title:          strings.TrimSpace(row.Title),
		ContentMD:      row.ContentMD,
		RenderStatus:   models.NormalizeDocumentRenderStatus(row.RenderStatus),
		RenderError:    strings.TrimSpace(row.RenderError),
		RenderedAt:     recordtime.ParseNullable(row.RenderedAtRaw),
		Version:        row.Version,
		SourceBlobID:   trimOptionalString(row.SourceBlobID),
		SourceFileName: trimOptionalString(row.SourceFileName),
		SourceMimeType: trimOptionalString(row.SourceMimeType),
		ContentVersion: normalizeContentVersion(row.ContentVersion, row.Version),
		AuthorNickname: strings.TrimSpace(row.AuthorNickname),
		UpdatedAt:      recordtime.Parse(row.UpdatedAtRaw),
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
		Table(tableWithAlias(models.Document{}, "d")).
		Select("d."+models.DocumentColumns.DocumentID+" AS document_id").
		Joins("JOIN "+tableName(models.Node{})+" AS n ON n."+models.NodeColumns.NodeID+" = d."+models.DocumentColumns.NodeID).
		Where("n."+models.NodeColumns.SpaceID+" = ?", normalizedSpaceID).
		Where("d." + models.DocumentColumns.DeletedAt + " IS NULL").
		Order("CASE WHEN n." + models.NodeColumns.ParentNodeID + " IS NULL THEN 0 ELSE 1 END ASC").
		Order("n." + models.NodeColumns.ParentNodeID + " ASC").
		Order("n." + models.NodeColumns.Sort + " ASC, n." + models.NodeColumns.ID + " ASC, d." + models.DocumentColumns.ID + " ASC").
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
		Table(tableWithAlias(models.Node{}, "n")).
		Select(
			"n."+models.NodeColumns.NodeID+" AS node_id",
			"d."+models.DocumentColumns.DocumentID+" AS document_id",
			"n."+models.NodeColumns.ReaderSlug+" AS reader_slug",
			"n."+models.NodeColumns.ParentNodeID+" AS parent_node_id",
			"n."+models.NodeColumns.Type+" AS type",
			"n."+models.NodeColumns.Title+" AS title",
			"n."+models.NodeColumns.Sort+" AS sort",
			"d."+models.DocumentColumns.Visibility+" AS document_visibility",
			"d."+models.DocumentColumns.Format+" AS document_format",
		).
		Joins("LEFT JOIN "+tableName(models.Document{})+" AS d ON d."+models.DocumentColumns.NodeID+" = n."+models.NodeColumns.NodeID).
		Where("n."+models.NodeColumns.SpaceID+" = ?", normalizedSpaceID).
		Order("n." + models.NodeColumns.ParentNodeID + " ASC, n." + models.NodeColumns.Sort + " ASC, n." + models.NodeColumns.ID + " ASC").
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
			DocumentFormat:     normalizeOptionalDocumentFormat(row.DocumentFormat),
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
