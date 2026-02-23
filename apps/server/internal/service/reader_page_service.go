package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

// ReaderPageService 负责聚合阅读页 SSR 所需的数据视图。
type ReaderPageService struct {
	db                *gorm.DB
	visibilityService *VisibilityService
}

type readerDocumentRow struct {
	DocumentID   string `gorm:"column:document_id"`
	NodeID       string `gorm:"column:node_id"`
	ThemeID      string `gorm:"column:theme_id"`
	Title        string `gorm:"column:title"`
	ContentMD    string `gorm:"column:content_md"`
	Version      int    `gorm:"column:version"`
	UpdatedAtRaw string `gorm:"column:updated_at"`
	SpaceID      string `gorm:"column:space_id"`
}

type readerTreeNodeRow struct {
	NodeID       string          `gorm:"column:node_id"`
	ParentNodeID *string         `gorm:"column:parent_node_id"`
	Type         models.NodeType `gorm:"column:type"`
	Title        string          `gorm:"column:title"`
	Sort         int             `gorm:"column:sort"`
}

type readerTreeNode struct {
	ID       string
	ParentID *string
	Type     models.NodeType
	Title    string
	Sort     int
	Children []*readerTreeNode
}

type readerDocumentIDRow struct {
	DocumentID string `gorm:"column:document_id"`
}

type readerResolvedDocument struct {
	DocumentID string `gorm:"column:document_id"`
}

// NewReaderPageService 创建阅读页聚合服务。
func NewReaderPageService(db *gorm.DB, visibilityService *VisibilityService) *ReaderPageService {
	return &ReaderPageService{
		db:                db,
		visibilityService: visibilityService,
	}
}

// ResolveLandingDocumentID 返回空间阅读入口应跳转的首篇可读文档 ID。
func (s *ReaderPageService) ResolveLandingDocumentID(
	ctx context.Context,
	spaceID string,
	viewerUserID string,
) (string, error) {
	if s == nil || s.db == nil || s.visibilityService == nil {
		return "", errors.New("reader page service dependencies are nil")
	}

	normalizedSpaceID := strings.TrimSpace(spaceID)
	normalizedViewerUserID := strings.TrimSpace(viewerUserID)
	if normalizedSpaceID == "" {
		return "", errors.New("space id is required")
	}

	if _, err := s.visibilityService.GetSpace(ctx, normalizedSpaceID, normalizedViewerUserID); err != nil {
		return "", err
	}

	candidateDocumentRows, err := s.loadSpaceDocumentIDs(ctx, normalizedSpaceID)
	if err != nil {
		return "", err
	}
	if len(candidateDocumentRows) == 0 {
		return "", ErrDocumentNotFound
	}

	hasDocumentAccessDenied := false
	hasLoginRequired := false
	for _, item := range candidateDocumentRows {
		documentID := strings.TrimSpace(item.DocumentID)
		if documentID == "" {
			continue
		}

		_, readErr := s.visibilityService.GetDocument(ctx, documentID, normalizedViewerUserID)
		if readErr == nil {
			return documentID, nil
		}

		switch {
		case errors.Is(readErr, ErrViewerLoginRequired):
			hasLoginRequired = true
		case errors.Is(readErr, ErrDocumentAccessDenied):
			hasDocumentAccessDenied = true
		case errors.Is(readErr, ErrDocumentNotFound):
			continue
		default:
			return "", readErr
		}
	}

	if hasLoginRequired {
		return "", ErrViewerLoginRequired
	}
	if hasDocumentAccessDenied {
		return "", ErrDocumentAccessDenied
	}
	return "", ErrDocumentNotFound
}

// BuildPage 按 space/doc/viewer 组装阅读页 SSR 输入视图。
func (s *ReaderPageService) BuildPage(
	ctx context.Context,
	spaceID string,
	documentID string,
	viewerUserID string,
) (ReaderPageViewModel, error) {
	if s == nil || s.db == nil || s.visibilityService == nil {
		return ReaderPageViewModel{}, errors.New("reader page service dependencies are nil")
	}

	normalizedSpaceID := strings.TrimSpace(spaceID)
	normalizedDocumentID := strings.TrimSpace(documentID)
	normalizedViewerUserID := strings.TrimSpace(viewerUserID)
	if normalizedSpaceID == "" {
		return ReaderPageViewModel{}, errors.New("space id is required")
	}
	if normalizedDocumentID == "" {
		return ReaderPageViewModel{}, errors.New("document id is required")
	}

	space, err := s.visibilityService.GetSpace(ctx, normalizedSpaceID, normalizedViewerUserID)
	if err != nil {
		return ReaderPageViewModel{}, err
	}

	resolvedDocumentID, err := s.resolveDocumentID(ctx, normalizedSpaceID, normalizedDocumentID)
	if err != nil {
		return ReaderPageViewModel{}, err
	}
	if _, err := s.visibilityService.GetDocument(ctx, resolvedDocumentID, normalizedViewerUserID); err != nil {
		return ReaderPageViewModel{}, err
	}

	documentRow, err := s.loadDocumentRow(ctx, resolvedDocumentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ReaderPageViewModel{}, ErrDocumentNotFound
		}
		return ReaderPageViewModel{}, err
	}
	if strings.TrimSpace(documentRow.SpaceID) != normalizedSpaceID {
		return ReaderPageViewModel{}, ErrDocumentNotFound
	}

	tree, err := s.loadTree(ctx, normalizedSpaceID)
	if err != nil {
		return ReaderPageViewModel{}, err
	}

	spaceName := strings.TrimSpace(space.Name)
	documentTitle := strings.TrimSpace(documentRow.Title)
	pageTitle := documentTitle
	if pageTitle == "" {
		pageTitle = "未命名文档"
	}
	if spaceName != "" {
		pageTitle = pageTitle + " - " + spaceName
	}

	updatedAt := formatReaderTime(documentRow.UpdatedAtRaw)

	return ReaderPageViewModel{
		Space: ReaderSpaceViewModel{
			ID:    normalizedSpaceID,
			Name:  spaceName,
			Title: pageTitle,
		},
		Document: ReaderDocumentViewModel{
			ID:        strings.TrimSpace(documentRow.DocumentID),
			NodeID:    strings.TrimSpace(documentRow.NodeID),
			ThemeID:   strings.TrimSpace(documentRow.ThemeID),
			Title:     documentTitle,
			ContentMD: documentRow.ContentMD,
			Version:   documentRow.Version,
			UpdatedAt: updatedAt,
		},
		Tree:        tree,
		ActiveDocID: strings.TrimSpace(documentRow.DocumentID),
	}, nil
}

func (s *ReaderPageService) resolveDocumentID(
	ctx context.Context,
	spaceID string,
	rawDocumentID string,
) (string, error) {
	normalizedSpaceID := strings.TrimSpace(spaceID)
	normalizedDocumentID := strings.TrimSpace(rawDocumentID)
	if normalizedSpaceID == "" || normalizedDocumentID == "" {
		return "", ErrDocumentNotFound
	}

	tryResolve := func(
		useSpaceFilter bool,
		condition string,
		args ...any,
	) (string, error) {
		query := s.db.WithContext(ctx).Table("documents AS d").Select("d.document_id")
		if useSpaceFilter {
			query = query.
				Joins("JOIN nodes AS n ON n.node_id = d.node_id").
				Where("n.space_id = ?", normalizedSpaceID)
		}

		var row readerResolvedDocument
		err := query.Where(condition, args...).Take(&row).Error
		if err == nil {
			return strings.TrimSpace(row.DocumentID), nil
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}

	// 先在目标空间内解析，优先级：document_id 精确 > node_id 精确 > document_id 忽略大小写 > node_id 忽略大小写。
	spaceScopedMatchers := []struct {
		condition string
		args      []any
	}{
		{condition: "d.document_id = ?", args: []any{normalizedDocumentID}},
		{condition: "d.node_id = ?", args: []any{normalizedDocumentID}},
		{condition: "LOWER(d.document_id) = LOWER(?)", args: []any{normalizedDocumentID}},
		{condition: "LOWER(d.node_id) = LOWER(?)", args: []any{normalizedDocumentID}},
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

	// 回退到全局解析：兼容异常历史数据（后续仍会校验 space 一致性）。
	globalMatchers := []struct {
		condition string
		args      []any
	}{
		{condition: "d.document_id = ?", args: []any{normalizedDocumentID}},
		{condition: "d.node_id = ?", args: []any{normalizedDocumentID}},
		{condition: "LOWER(d.document_id) = LOWER(?)", args: []any{normalizedDocumentID}},
		{condition: "LOWER(d.node_id) = LOWER(?)", args: []any{normalizedDocumentID}},
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
	return "", ErrDocumentNotFound
}

func (s *ReaderPageService) loadDocumentRow(
	ctx context.Context,
	documentID string,
) (readerDocumentRow, error) {
	var row readerDocumentRow
	err := s.db.WithContext(ctx).
		Table("documents AS d").
		Select(
			"d.document_id",
			"d.node_id",
			"d.theme_id",
			"d.title",
			"d.content_md",
			"d.version",
			"d.updated_at",
			"n.space_id AS space_id",
		).
		Joins("JOIN nodes AS n ON n.node_id = d.node_id").
		Where("d.document_id = ?", documentID).
		Take(&row).Error
	return row, err
}

func (s *ReaderPageService) loadSpaceDocumentIDs(
	ctx context.Context,
	spaceID string,
) ([]readerDocumentIDRow, error) {
	var rows []readerDocumentIDRow
	if err := s.db.WithContext(ctx).
		Table("documents AS d").
		Select("d.document_id").
		Joins("JOIN nodes AS n ON n.node_id = d.node_id").
		Where("n.space_id = ?", spaceID).
		// 仅过滤已软删除文档；状态是否可读交由 visibilityService 统一判断，
		// 以兼容历史数据中的空状态值。
		Where("d.deleted_at IS NULL").
		Order("CASE WHEN n.parent_node_id IS NULL THEN 0 ELSE 1 END ASC").
		Order("n.parent_node_id ASC").
		Order("n.sort ASC, n.id ASC, d.id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *ReaderPageService) loadTree(
	ctx context.Context,
	spaceID string,
) ([]ReaderTreeNodeViewModel, error) {
	var rows []readerTreeNodeRow
	if err := s.db.WithContext(ctx).
		Table("nodes").
		Select("node_id", "parent_node_id", "type", "title", "sort").
		Where("space_id = ?", spaceID).
		Order("parent_node_id ASC, sort ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	treeNodes := make(map[string]*readerTreeNode, len(rows))
	for _, row := range rows {
		nodeID := strings.TrimSpace(row.NodeID)
		if nodeID == "" {
			continue
		}
		treeNodes[nodeID] = &readerTreeNode{
			ID:       nodeID,
			ParentID: normalizeReaderOptionalString(row.ParentNodeID),
			Type:     normalizeReaderNodeType(row.Type),
			Title:    strings.TrimSpace(row.Title),
			Sort:     row.Sort,
			Children: make([]*readerTreeNode, 0),
		}
	}

	roots := make([]*readerTreeNode, 0)
	for _, row := range rows {
		nodeID := strings.TrimSpace(row.NodeID)
		node, ok := treeNodes[nodeID]
		if !ok {
			continue
		}
		parentID := normalizeReaderOptionalString(row.ParentNodeID)
		if parentID == nil {
			roots = append(roots, node)
			continue
		}
		parentNode, exists := treeNodes[*parentID]
		if !exists {
			roots = append(roots, node)
			continue
		}
		parentNode.Children = append(parentNode.Children, node)
	}

	return mapReaderTreeNodes(roots), nil
}

func normalizeReaderOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func normalizeReaderNodeType(value models.NodeType) models.NodeType {
	if value == models.NodeTypeFolder || value == models.NodeTypeDoc {
		return value
	}
	return models.NodeTypeDoc
}

func mapReaderTreeNodes(nodes []*readerTreeNode) []ReaderTreeNodeViewModel {
	items := make([]ReaderTreeNodeViewModel, 0, len(nodes))
	for _, node := range nodes {
		if node == nil {
			continue
		}
		items = append(items, ReaderTreeNodeViewModel{
			ID:       node.ID,
			ParentID: node.ParentID,
			Type:     normalizeReaderNodeType(node.Type),
			Title:    node.Title,
			Sort:     node.Sort,
			Children: mapReaderTreeNodes(node.Children),
		})
	}
	return items
}

func formatReaderTime(raw string) string {
	parsed := parseReaderTime(raw)
	if parsed.IsZero() {
		return time.Now().UTC().Format(time.RFC3339Nano)
	}
	return parsed.UTC().Format(time.RFC3339Nano)
}

func parseReaderTime(raw string) time.Time {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05.999999999",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05.999999999-07:00",
	}
	for _, layout := range layouts {
		if parsedAt, err := time.Parse(layout, value); err == nil {
			return parsedAt.UTC()
		}
	}
	return time.Time{}
}
