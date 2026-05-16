package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/lifei6671/plaindoc/apps/server/internal/ssr/protocol"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
)

type adminSpaceExportSSRDispatcher interface {
	Render(ctx context.Context, request protocol.RenderRequest) (protocol.RenderResponse, error)
}

type adminSpaceExportReaderPagePayload struct {
	Space           ReaderSpaceViewModel                `json:"space"`
	Document        ReaderDocumentViewModel             `json:"document"`
	Attachments     []ReaderDocumentAttachmentViewModel `json:"attachments"`
	Tree            []ReaderTreeNodeViewModel           `json:"tree"`
	ActiveDocID     string                              `json:"activeDocId"`
	RequestOrigin   string                              `json:"requestOrigin,omitempty"`
	Viewer          adminSpaceExportReaderViewer        `json:"viewer"`
	OfficeRendering adminSpaceExportReaderOfficeState   `json:"officeRendering"`
}

type adminSpaceExportReaderViewer struct {
	UserID        string `json:"userId,omitempty"`
	Name          string `json:"name,omitempty"`
	Authenticated bool   `json:"authenticated"`
}

type adminSpaceExportReaderOfficeState struct {
	IndependentRenderEnabled            bool `json:"independentRenderEnabled"`
	FallbackToOnlyOfficeOnRenderFailure bool `json:"fallbackToOnlyOfficeOnRenderFailure"`
}

// AdminSpaceExportSSRReaderHTMLRenderer 通过阅读页 SSR worker 渲染 EPUB Markdown 章节。
type AdminSpaceExportSSRReaderHTMLRenderer struct {
	dispatcher    adminSpaceExportSSRDispatcher
	requestOrigin string
}

// NewAdminSpaceExportSSRReaderHTMLRenderer 创建导出侧阅读页 SSR 渲染适配器。
func NewAdminSpaceExportSSRReaderHTMLRenderer(
	dispatcher adminSpaceExportSSRDispatcher,
	requestOrigin string,
) *AdminSpaceExportSSRReaderHTMLRenderer {
	return &AdminSpaceExportSSRReaderHTMLRenderer{
		dispatcher:    dispatcher,
		requestOrigin: strings.TrimSpace(requestOrigin),
	}
}

// RenderMarkdownHTML 返回阅读页 SSR 的完整 HTML 文档。
func (r *AdminSpaceExportSSRReaderHTMLRenderer) RenderMarkdownHTML(
	ctx context.Context,
	input AdminSpaceExportReaderHTMLRenderInput,
) (string, error) {
	if r == nil || r.dispatcher == nil {
		return "", errors.New("reader ssr renderer is not configured")
	}
	payloadBytes, err := json.Marshal(r.buildPayload(input))
	if err != nil {
		return "", fmt.Errorf("marshal reader ssr payload: %w", err)
	}
	response, err := r.dispatcher.Render(ctx, protocol.RenderRequest{
		ID:      strings.ToLower(ulid.Make().String()),
		Type:    protocol.MessageTypeRender,
		Route:   "space-reader",
		Payload: payloadBytes,
	})
	if err != nil {
		return "", fmt.Errorf("render reader ssr: %w", err)
	}
	if !response.OK || strings.TrimSpace(response.HTML) == "" {
		if response.Error != nil {
			return "", fmt.Errorf("render reader ssr rejected: %s", strings.TrimSpace(response.Error.Message))
		}
		return "", errors.New("render reader ssr returned empty html")
	}
	return response.HTML, nil
}

func (r *AdminSpaceExportSSRReaderHTMLRenderer) buildPayload(
	input AdminSpaceExportReaderHTMLRenderInput,
) adminSpaceExportReaderPagePayload {
	document := input.Document
	visibility := normalizeAdminSpaceExportReaderVisibility(document.Visibility)
	exportedAt := input.ExportedAt
	if exportedAt.IsZero() {
		exportedAt = time.Now().UTC()
	}
	spaceID := strings.TrimSpace(input.Space.SpaceID)
	documentID := strings.TrimSpace(document.DocumentID)
	return adminSpaceExportReaderPagePayload{
		Space: ReaderSpaceViewModel{
			ID:    spaceID,
			Name:  strings.TrimSpace(input.Space.Name),
			Title: strings.TrimSpace(input.Space.Name),
		},
		Document: ReaderDocumentViewModel{
			ID:             documentID,
			NodeID:         strings.TrimSpace(document.NodeID),
			RouteKey:       documentID,
			ThemeID:        "default",
			Format:         models.DocumentFormatMarkdown,
			Visibility:     visibility,
			Title:          strings.TrimSpace(document.Title),
			ContentMD:      input.Content,
			RenderStatus:   models.DocumentRenderStatusIdle,
			RenderError:    "",
			Version:        1,
			ContentVersion: 1,
			AuthorNickname: "PlainDoc",
			UpdatedAt:      exportedAt.UTC().Format(time.RFC3339),
		},
		Attachments:   []ReaderDocumentAttachmentViewModel{},
		Tree:          buildAdminSpaceExportReaderTree(input.Tree.Root),
		ActiveDocID:   documentID,
		RequestOrigin: strings.TrimSpace(r.requestOrigin),
		Viewer: adminSpaceExportReaderViewer{
			Authenticated: false,
		},
		OfficeRendering: adminSpaceExportReaderOfficeState{
			IndependentRenderEnabled:            false,
			FallbackToOnlyOfficeOnRenderFailure: true,
		},
	}
}

func buildAdminSpaceExportReaderTree(nodes []AdminSpaceExportTreeNode) []ReaderTreeNodeViewModel {
	if len(nodes) == 0 {
		return []ReaderTreeNodeViewModel{}
	}
	result := make([]ReaderTreeNodeViewModel, 0, len(nodes))
	for _, node := range nodes {
		result = append(result, buildAdminSpaceExportReaderTreeNode(node))
	}
	return result
}

func buildAdminSpaceExportReaderTreeNode(node AdminSpaceExportTreeNode) ReaderTreeNodeViewModel {
	var documentID *string
	if trimmed := strings.TrimSpace(node.DocumentID); trimmed != "" {
		documentID = &trimmed
	}
	var format *models.DocumentFormat
	if documentID != nil {
		format = new(models.NormalizeDocumentFormat(models.DocumentFormat(node.Format)))
	}
	return ReaderTreeNodeViewModel{
		ID:               strings.TrimSpace(node.NodeID),
		DocumentID:       documentID,
		DocumentRouteKey: documentID,
		DocumentFormat:   format,
		ParentID:         node.ParentNodeID,
		Type:             models.NodeType(strings.TrimSpace(node.Type)),
		Title:            strings.TrimSpace(node.Title),
		Sort:             node.Sort,
		Children:         buildAdminSpaceExportReaderTree(node.Children),
	}
}

func normalizeAdminSpaceExportReaderVisibility(value string) models.Visibility {
	visibility := models.Visibility(strings.TrimSpace(value))
	if models.IsValidVisibility(visibility) {
		return visibility
	}
	return models.VisibilityMember
}
