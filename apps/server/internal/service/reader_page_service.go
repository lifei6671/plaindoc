package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

// ReaderPageService 负责聚合阅读页 SSR 所需的数据视图。
type ReaderPageService struct {
	readerPageRepo         repository.ReaderPageRepository
	visibilityService      *VisibilityService
	documentAttachmentRepo repository.DocumentAttachmentRepository
}

type readerTreeNode struct {
	ID             string
	DocumentID     *string
	ReaderSlug     *string
	DocumentPath   *string
	DocumentFormat *models.DocumentFormat
	ParentID       *string
	Type           models.NodeType
	Title          string
	Sort           int
	Visibility     *models.Visibility
	Children       []*readerTreeNode
}

// NewReaderPageService 创建阅读页聚合服务。
func NewReaderPageService(
	db *gorm.DB,
	visibilityService *VisibilityService,
	documentAttachmentRepo repository.DocumentAttachmentRepository,
) *ReaderPageService {
	return &ReaderPageService{
		readerPageRepo:         repository.NewGormReaderPageRepository(db),
		visibilityService:      visibilityService,
		documentAttachmentRepo: documentAttachmentRepo,
	}
}

// ResolveLandingDocumentID 返回空间阅读入口应跳转的首篇可读文档路由标识（slug 或 document_id）。
func (s *ReaderPageService) ResolveLandingDocumentID(
	ctx context.Context,
	spaceID string,
	viewerUserID string,
) (string, error) {
	if s == nil || s.readerPageRepo == nil || s.visibilityService == nil {
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
		documentID := strings.TrimSpace(item)
		if documentID == "" {
			continue
		}

		_, readErr := s.visibilityService.GetDocument(ctx, documentID, normalizedViewerUserID)
		if readErr == nil {
			documentRow, loadErr := s.loadDocumentRow(ctx, documentID)
			if loadErr != nil {
				if errors.Is(loadErr, gorm.ErrRecordNotFound) {
					continue
				}
				return "", loadErr
			}
			return resolveReaderDocumentRouteKey(documentRow.DocumentID, documentRow.ReaderSlug), nil
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
	if s == nil || s.readerPageRepo == nil || s.visibilityService == nil {
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
	attachments, err := s.loadDocumentAttachments(ctx, resolvedDocumentID)
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
	documentIDValue := strings.TrimSpace(documentRow.DocumentID)
	documentIdentifier := normalizeReaderOptionalString(documentRow.ReaderSlug)
	documentRouteKey := resolveReaderDocumentRouteKey(documentIDValue, documentRow.ReaderSlug)

	return ReaderPageViewModel{
		Space: ReaderSpaceViewModel{
			ID:    normalizedSpaceID,
			Name:  spaceName,
			Title: pageTitle,
		},
		Document: ReaderDocumentViewModel{
			ID:             documentIDValue,
			NodeID:         strings.TrimSpace(documentRow.NodeID),
			Identifier:     normalizeReaderString(documentIdentifier),
			RouteKey:       documentRouteKey,
			ThemeID:        strings.TrimSpace(documentRow.ThemeID),
			Format:         models.NormalizeDocumentFormat(documentRow.Format),
			Visibility:     normalizeReaderVisibility(documentRow.Visibility),
			Title:          documentTitle,
			ContentMD:      documentRow.ContentMD,
			RenderStatus:   models.NormalizeDocumentRenderStatus(documentRow.RenderStatus),
			RenderError:    strings.TrimSpace(documentRow.RenderError),
			RenderedAt:     formatReaderOptionalTime(documentRow.RenderedAt),
			Version:        documentRow.Version,
			SourceBlobID:   normalizeReaderOptionalString(documentRow.SourceBlobID),
			SourceFileName: normalizeReaderOptionalString(documentRow.SourceFileName),
			SourceMimeType: normalizeReaderOptionalString(documentRow.SourceMimeType),
			ContentVersion: normalizeReaderContentVersion(documentRow.ContentVersion, documentRow.Version),
			AuthorNickname: normalizeReaderAuthorNickname(documentRow.AuthorNickname),
			UpdatedAt:      updatedAt,
		},
		Attachments: attachments,
		Tree:        tree,
		ActiveDocID: documentIDValue,
	}, nil
}

// BuildSpaceContext 按 space/viewer 组装阅读页左侧所需的空间与目录树上下文。
func (s *ReaderPageService) BuildSpaceContext(
	ctx context.Context,
	spaceID string,
	viewerUserID string,
) (ReaderSpaceViewModel, []ReaderTreeNodeViewModel, error) {
	if s == nil || s.readerPageRepo == nil || s.visibilityService == nil {
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
	if s == nil || s.readerPageRepo == nil {
		return "", errors.New("reader page repository is nil")
	}

	resolvedDocumentID, err := s.readerPageRepo.ResolveDocumentID(ctx, spaceID, rawDocumentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrDocumentNotFound
		}
		return "", err
	}
	resolvedDocumentID = strings.TrimSpace(resolvedDocumentID)
	if resolvedDocumentID == "" {
		return "", ErrDocumentNotFound
	}
	return resolvedDocumentID, nil
}

func (s *ReaderPageService) loadDocumentRow(
	ctx context.Context,
	documentID string,
) (repository.ReaderPageDocumentRecord, error) {
	if s == nil || s.readerPageRepo == nil {
		return repository.ReaderPageDocumentRecord{}, errors.New("reader page repository is nil")
	}
	row, err := s.readerPageRepo.GetDocumentByDocumentID(ctx, documentID)
	if err != nil {
		return repository.ReaderPageDocumentRecord{}, err
	}
	if row == nil {
		return repository.ReaderPageDocumentRecord{}, gorm.ErrRecordNotFound
	}
	return *row, nil
}

func (s *ReaderPageService) loadSpaceDocumentIDs(
	ctx context.Context,
	spaceID string,
) ([]string, error) {
	if s == nil || s.readerPageRepo == nil {
		return nil, errors.New("reader page repository is nil")
	}
	rows, err := s.readerPageRepo.ListSpaceDocumentIDs(ctx, spaceID)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *ReaderPageService) loadTree(
	ctx context.Context,
	spaceID string,
) ([]ReaderTreeNodeViewModel, error) {
	if s == nil || s.readerPageRepo == nil {
		return nil, errors.New("reader page repository is nil")
	}
	rows, err := s.readerPageRepo.ListTreeNodesBySpaceID(ctx, spaceID)
	if err != nil {
		return nil, err
	}

	treeNodes := make(map[string]*readerTreeNode, len(rows))
	for _, row := range rows {
		nodeID := strings.TrimSpace(row.NodeID)
		if nodeID == "" {
			continue
		}
		documentID := normalizeReaderOptionalString(row.DocumentID)
		readerSlug := normalizeReaderOptionalString(row.ReaderSlug)
		documentPath := resolveReaderTreeDocumentRouteKey(documentID, readerSlug)
		documentVisibility := normalizeReaderDocumentVisibility(row.Type, row.DocumentVisibility)
		documentFormat := normalizeReaderTreeDocumentFormat(row.Type, row.DocumentFormat)
		treeNodes[nodeID] = &readerTreeNode{
			ID:             nodeID,
			DocumentID:     documentID,
			ReaderSlug:     readerSlug,
			DocumentPath:   documentPath,
			DocumentFormat: documentFormat,
			ParentID:       normalizeReaderOptionalString(row.ParentNodeID),
			Type:           normalizeReaderNodeType(row.Type),
			Title:          strings.TrimSpace(row.Title),
			Sort:           row.Sort,
			Visibility:     documentVisibility,
			Children:       make([]*readerTreeNode, 0),
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
			ID:                 node.ID,
			DocumentID:         node.DocumentID,
			DocumentIdentifier: node.ReaderSlug,
			DocumentRouteKey:   node.DocumentPath,
			DocumentFormat:     node.DocumentFormat,
			ParentID:           node.ParentID,
			Type:               normalizeReaderNodeType(node.Type),
			Title:              node.Title,
			Sort:               node.Sort,
			Visibility:         node.Visibility,
			Children:           mapReaderTreeNodes(node.Children),
		})
	}
	return items
}

func resolveReaderDocumentRouteKey(documentID string, readerSlug *string) string {
	normalizedSlug := strings.ToLower(normalizeReaderString(normalizeReaderOptionalString(readerSlug)))
	if normalizedSlug != "" {
		return normalizedSlug
	}
	return strings.TrimSpace(documentID)
}

func resolveReaderTreeDocumentRouteKey(documentID *string, readerSlug *string) *string {
	if documentID == nil {
		return nil
	}
	documentRouteKey := resolveReaderDocumentRouteKey(*documentID, readerSlug)
	if strings.TrimSpace(documentRouteKey) == "" {
		return nil
	}
	return &documentRouteKey
}

func normalizeReaderString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func (s *ReaderPageService) loadDocumentAttachments(
	ctx context.Context,
	documentID string,
) ([]ReaderDocumentAttachmentViewModel, error) {
	if s == nil || s.documentAttachmentRepo == nil {
		return []ReaderDocumentAttachmentViewModel{}, nil
	}

	normalizedDocumentID := strings.TrimSpace(documentID)
	if normalizedDocumentID == "" {
		return []ReaderDocumentAttachmentViewModel{}, nil
	}

	attachmentRows, err := s.documentAttachmentRepo.ListByDocumentID(ctx, normalizedDocumentID, false)
	if err != nil {
		return nil, err
	}

	attachments := make([]ReaderDocumentAttachmentViewModel, 0, len(attachmentRows))
	for _, item := range attachmentRows {
		attachmentID := strings.TrimSpace(item.AttachmentID)
		if attachmentID == "" {
			continue
		}
		fileName := strings.TrimSpace(item.FileName)
		if fileName == "" {
			fileName = attachmentID
		}
		mimeType := strings.TrimSpace(item.MimeType)
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		previewKind := normalizeReaderAttachmentPreviewKind(item.PreviewKind)
		attachments = append(attachments, ReaderDocumentAttachmentViewModel{
			AttachmentID:     attachmentID,
			DocumentID:       strings.TrimSpace(item.DocumentID),
			FileName:         fileName,
			MimeType:         mimeType,
			SizeBytes:        item.SizeBytes,
			PreviewKind:      previewKind,
			PreviewSupported: isReaderAttachmentPreviewSupported(previewKind),
		})
	}
	return attachments, nil
}

func normalizeReaderAttachmentPreviewKind(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "image":
		return "image"
	case "pdf":
		return "pdf"
	case "office":
		return "office"
	case "text":
		return "text"
	default:
		return "none"
	}
}

func isReaderAttachmentPreviewSupported(previewKind string) bool {
	switch normalizeReaderAttachmentPreviewKind(previewKind) {
	case "image", "pdf", "office", "text":
		return true
	default:
		return false
	}
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

func normalizeReaderTreeDocumentFormat(
	nodeType models.NodeType,
	rawFormat *models.DocumentFormat,
) *models.DocumentFormat {
	if normalizeReaderNodeType(nodeType) != models.NodeTypeDoc {
		return nil
	}
	if rawFormat == nil {
		format := models.DocumentFormatMarkdown
		return &format
	}
	format := models.NormalizeDocumentFormat(*rawFormat)
	return &format
}

func normalizeReaderContentVersion(contentVersion int, version int) int {
	switch {
	case contentVersion > 0:
		return contentVersion
	case version > 0:
		return version
	default:
		return 1
	}
}

func normalizeReaderAuthorNickname(raw string) string {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "未知作者"
	}
	return name
}

func formatReaderTime(raw time.Time) string {
	if raw.IsZero() {
		return ""
	}
	return raw.UTC().Format(time.RFC3339Nano)
}

func formatReaderOptionalTime(raw *time.Time) string {
	if raw == nil || raw.IsZero() {
		return ""
	}
	return raw.UTC().Format(time.RFC3339Nano)
}
