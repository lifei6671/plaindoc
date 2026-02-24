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
	DocumentID     string `gorm:"column:document_id"`
	NodeID         string `gorm:"column:node_id"`
	ThemeID        string `gorm:"column:theme_id"`
	Visibility     string `gorm:"column:visibility"`
	Title          string `gorm:"column:title"`
	ContentMD      string `gorm:"column:content_md"`
	Version        int    `gorm:"column:version"`
	AuthorNickname string `gorm:"column:author_nickname"`
	UpdatedAt      string `gorm:"column:updated_at"`
	SpaceID        string `gorm:"column:space_id"`
}

type readerTreeNodeRow struct {
	NodeID             string          `gorm:"column:node_id"`
	DocumentID         *string         `gorm:"column:document_id"`
	ParentNodeID       *string         `gorm:"column:parent_node_id"`
	Type               models.NodeType `gorm:"column:type"`
	Title              string          `gorm:"column:title"`
	Sort               int             `gorm:"column:sort"`
	DocumentVisibility *string         `gorm:"column:document_visibility"`
}

type readerTreeNode struct {
	ID         string
	DocumentID *string
	ParentID   *string
	Type       models.NodeType
	Title      string
	Sort       int
	Visibility *models.Visibility
	Children   []*readerTreeNode
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

	updatedAt := formatReaderTime(documentRow.UpdatedAt)

	return ReaderPageViewModel{
		Space: ReaderSpaceViewModel{
			ID:    normalizedSpaceID,
			Name:  spaceName,
			Title: pageTitle,
		},
		Document: ReaderDocumentViewModel{
			ID:             strings.TrimSpace(documentRow.DocumentID),
			NodeID:         strings.TrimSpace(documentRow.NodeID),
			ThemeID:        strings.TrimSpace(documentRow.ThemeID),
			Visibility:     normalizeReaderVisibility(documentRow.Visibility),
			Title:          documentTitle,
			ContentMD:      documentRow.ContentMD,
			Version:        documentRow.Version,
			AuthorNickname: normalizeReaderAuthorNickname(documentRow.AuthorNickname),
			UpdatedAt:      updatedAt,
		},
		Tree:        tree,
		ActiveDocID: strings.TrimSpace(documentRow.DocumentID),
	}, nil
}

// BuildSpaceContext 按 space/viewer 组装阅读页左侧所需的空间与目录树上下文。
func (s *ReaderPageService) BuildSpaceContext(
	ctx context.Context,
	spaceID string,
	viewerUserID string,
) (ReaderSpaceViewModel, []ReaderTreeNodeViewModel, error) {
	if s == nil || s.db == nil || s.visibilityService == nil {
		return ReaderSpaceViewModel{}, nil, errors.New("reader page service dependencies are nil")
	}

	normalizedSpaceID := strings.TrimSpace(spaceID)
	normalizedViewerUserID := strings.TrimSpace(viewerUserID)
	if normalizedSpaceID == "" {
		return ReaderSpaceViewModel{}, nil, errors.New("space id is required")
	}

	space, err := s.visibilityService.GetSpace(ctx, normalizedSpaceID, normalizedViewerUserID)
	if err != nil {
		return ReaderSpaceViewModel{}, nil, err
	}

	tree, err := s.loadTree(ctx, normalizedSpaceID)
	if err != nil {
		return ReaderSpaceViewModel{}, nil, err
	}

	spaceName := strings.TrimSpace(space.Name)
	return ReaderSpaceViewModel{
		ID:    normalizedSpaceID,
		Name:  spaceName,
		Title: spaceName,
	}, tree, nil
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
			"d.visibility",
			"d.title",
			"d.content_md",
			"d.version",
			// 作者固定取首个版本（version=1）的编辑者，避免后续更新人覆盖创建人语义。
			"COALESCE(NULLIF(TRIM(u_creator.name), ''), '未知作者') AS author_nickname",
			"d.updated_at",
			"n.space_id AS space_id",
		).
		Joins("JOIN nodes AS n ON n.node_id = d.node_id").
		Joins("LEFT JOIN document_revisions AS dr_creator ON dr_creator.document_id = d.document_id AND dr_creator.version = 1").
		Joins("LEFT JOIN users AS u_creator ON u_creator.user_id = dr_creator.editor_user_id").
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
		Table("nodes AS n").
		Select(
			"n.node_id",
			"d.document_id AS document_id",
			"n.parent_node_id",
			"n.type",
			"n.title",
			"n.sort",
			"d.visibility AS document_visibility",
		).
		Joins("LEFT JOIN documents AS d ON d.node_id = n.node_id").
		Where("n.space_id = ?", spaceID).
		Order("n.parent_node_id ASC, n.sort ASC, n.id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	treeNodes := make(map[string]*readerTreeNode, len(rows))
	for _, row := range rows {
		nodeID := strings.TrimSpace(row.NodeID)
		if nodeID == "" {
			continue
		}
		documentID := normalizeReaderOptionalString(row.DocumentID)
		documentVisibility := normalizeReaderDocumentVisibility(row.Type, row.DocumentVisibility)
		treeNodes[nodeID] = &readerTreeNode{
			ID:         nodeID,
			DocumentID: documentID,
			ParentID:   normalizeReaderOptionalString(row.ParentNodeID),
			Type:       normalizeReaderNodeType(row.Type),
			Title:      strings.TrimSpace(row.Title),
			Sort:       row.Sort,
			Visibility: documentVisibility,
			Children:   make([]*readerTreeNode, 0),
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
			ID:         node.ID,
			DocumentID: node.DocumentID,
			ParentID:   node.ParentID,
			Type:       normalizeReaderNodeType(node.Type),
			Title:      node.Title,
			Sort:       node.Sort,
			Visibility: node.Visibility,
			Children:   mapReaderTreeNodes(node.Children),
		})
	}
	return items
}

func normalizeReaderVisibility(raw string) models.Visibility {
	value := models.Visibility(strings.TrimSpace(raw))
	if !models.IsValidVisibility(value) {
		return models.VisibilityMember
	}
	return value
}

func normalizeReaderDocumentVisibility(
	nodeType models.NodeType,
	rawVisibility *string,
) *models.Visibility {
	if normalizeReaderNodeType(nodeType) != models.NodeTypeDoc {
		return nil
	}
	if rawVisibility == nil {
		visibility := models.VisibilityMember
		return &visibility
	}
	visibility := normalizeReaderVisibility(*rawVisibility)
	return &visibility
}

func normalizeReaderAuthorNickname(raw string) string {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "未知作者"
	}
	return name
}

func formatReaderTime(raw string) string {
	parsed := parseReaderTime(raw)
	if parsed.IsZero() {
		return ""
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
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02T15:04:05.999999999-07:00",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02 15:04:05.999999999-0700",
		"2006-01-02T15:04:05.999999999-0700",
		"2006-01-02 15:04:05-0700",
		"2006-01-02T15:04:05-0700",
		"2006-01-02 15:04:05.999999999-07",
		"2006-01-02T15:04:05.999999999-07",
		"2006-01-02 15:04:05-07",
		"2006-01-02T15:04:05-07",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02T15:04:05.999999999",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed
		}
	}
	return time.Time{}
}
