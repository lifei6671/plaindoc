package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/ssr/protocol"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
)

func TestAdminSpaceExportSSRReaderHTMLRenderer_RendersSpaceReaderPayload(t *testing.T) {
	t.Parallel()

	dispatcher := &fakeAdminSpaceExportSSRDispatcher{
		response: protocol.RenderResponse{
			OK:   true,
			HTML: `<html><body><article id="plaindoc-preview-body"><h1>SSR</h1></article></body></html>`,
		},
	}
	renderer := NewAdminSpaceExportSSRReaderHTMLRenderer(dispatcher, "https://docs.example.com")
	documentID := "doc-a"

	html, err := renderer.RenderMarkdownHTML(context.Background(), AdminSpaceExportReaderHTMLRenderInput{
		Space: models.Space{SpaceID: "space-a", Name: "空间 A", Visibility: models.VisibilityMember},
		Document: AdminSpaceExportDocumentEntry{
			DocumentID: documentID,
			NodeID:     "node-a",
			Title:      "文档 A",
			Format:     string(models.DocumentFormatMarkdown),
			Visibility: string(models.VisibilityPublic),
		},
		Content: "# 标题\n\n正文",
		Tree: AdminSpaceExportTree{
			Root: []AdminSpaceExportTreeNode{
				{
					NodeID:     "node-a",
					DocumentID: documentID,
					Type:       string(models.NodeTypeDoc),
					Title:      "文档 A",
					Format:     string(models.DocumentFormatMarkdown),
				},
			},
		},
		ExportedAt: time.Date(2026, 5, 16, 8, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("render markdown html failed: %v", err)
	}
	if !strings.Contains(html, `plaindoc-preview-body`) {
		t.Fatalf("expected reader html, got %s", html)
	}
	if dispatcher.calls != 1 {
		t.Fatalf("expected one dispatcher call, got %d", dispatcher.calls)
	}
	if dispatcher.lastRequest.Type != protocol.MessageTypeRender || dispatcher.lastRequest.Route != "space-reader" {
		t.Fatalf("unexpected render request: %#v", dispatcher.lastRequest)
	}
	payloadJSON := string(dispatcher.lastRequest.Payload)
	for _, unexpected := range []string{`"attachments":null`, `"tree":null`, `"children":null`} {
		if strings.Contains(payloadJSON, unexpected) {
			t.Fatalf("reader ssr payload contains nullable array field %s: %s", unexpected, payloadJSON)
		}
	}

	var payload adminSpaceExportReaderPagePayload
	if err := json.Unmarshal(dispatcher.lastRequest.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload failed: %v", err)
	}
	if payload.Document.ID != documentID || payload.Document.ContentMD != "# 标题\n\n正文" {
		t.Fatalf("unexpected document payload: %#v", payload.Document)
	}
	if payload.RequestOrigin != "https://docs.example.com" || payload.ActiveDocID != documentID {
		t.Fatalf("unexpected request origin or active doc: %#v", payload)
	}
	if len(payload.Tree) != 1 || payload.Tree[0].DocumentRouteKey == nil || *payload.Tree[0].DocumentRouteKey != documentID {
		t.Fatalf("unexpected tree payload: %#v", payload.Tree)
	}
}

type fakeAdminSpaceExportSSRDispatcher struct {
	response    protocol.RenderResponse
	calls       int
	lastRequest protocol.RenderRequest
}

func (f *fakeAdminSpaceExportSSRDispatcher) Render(
	_ context.Context,
	request protocol.RenderRequest,
) (protocol.RenderResponse, error) {
	f.calls++
	f.lastRequest = request
	return f.response, nil
}
